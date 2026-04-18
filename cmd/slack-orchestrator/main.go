package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bimross/slack-orchestrator/internal/channelmembers"
	"github.com/bimross/slack-orchestrator/internal/config"
	"github.com/bimross/slack-orchestrator/internal/decisionlog"
	"github.com/bimross/slack-orchestrator/internal/logging"
	"github.com/bimross/slack-orchestrator/internal/memberchannels"
	"github.com/bimross/slack-orchestrator/internal/metrics"
	"github.com/bimross/slack-orchestrator/internal/routing"
	"github.com/bimross/slack-orchestrator/internal/slackrun"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func main() {
	_ = godotenv.Load()
	cfg := config.FromEnv()
	logging.Init()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/metrics", promhttp.Handler())
	logStore := decisionlog.New(cfg.DecisionLogMax)
	slackrun.SetDecisionLog(logStore)
	mux.HandleFunc("/debug/decisions", decisionlog.HTTPHandler(logStore, cfg.DebugToken, cfg.DebugAllowAnon))
	mux.HandleFunc("/debug/member-channels", memberchannels.HTTPHandler(cfg.BotToken, cfg.DebugToken, cfg.DebugAllowAnon))
	mux.HandleFunc("/debug/channel-members", channelmembers.HTTPHandler(cfg.BotToken, cfg.DebugToken, cfg.DebugAllowAnon))
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: mux}
	go func() {
		slog.Info("http_listen", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http_listen", "error", err)
			os.Exit(1)
		}
	}()

	if len(cfg.MultiagentOrder) == 0 {
		slog.Warn("multiagent_roster_empty", "msg", "set MULTIAGENT_BOT_USER_IDS (roster is derived and shuffled) or optional MULTIAGENT_ORDER override")
	}
	if cfg.BotToken == "" || cfg.AppToken == "" {
		slog.Warn("slack_tokens_missing", "msg", "set SLACK_BOT_TOKEN and SLACK_APP_TOKEN to enable Socket Mode")
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
		return
	}

	// Socket Mode requires the app-level token on the API client (apps.connections.open).
	api := slack.New(cfg.BotToken, slack.OptionAppLevelToken(cfg.AppToken))
	slackrun.SetThreadRoutingFetcher(func(ctx context.Context, channelID, threadTS, currentMessageTS string) (string, error) {
		msgs, _, _, err := api.GetConversationRepliesContext(ctx, &slack.GetConversationRepliesParameters{
			ChannelID: channelID,
			Timestamp: threadTS,
			Limit:     200,
			Inclusive: true,
		})
		if err != nil {
			return "", err
		}
		if len(msgs) == 0 {
			return "", fmt.Errorf("conversations.replies: empty thread")
		}
		var threadMsgs []routing.ThreadMessage
		for i := range msgs {
			ts := strings.TrimSpace(msgs[i].Timestamp)
			if ts == "" {
				continue
			}
			if slackTimestampLess(ts, currentMessageTS) {
				threadMsgs = append(threadMsgs, routing.ThreadMessage{
					Timestamp: ts,
					Text:      msgs[i].Text,
				})
			}
		}
		rc := routing.DecideConfig{
			Order:         cfg.MultiagentOrder,
			BotUserToKey:  cfg.BotUserToKey,
			EveryoneLimit: cfg.EveryoneLimit,
			ChannelLimit:  cfg.ChannelLimit,
			ShuffleSecret: cfg.ShuffleSecret,
		}
		return routing.LastSquadHandoffKey(threadMsgs, threadTS, rc), nil
	})
	var smOpts []socketmode.Option
	if cfg.SocketModeDebug {
		smOpts = append(smOpts, socketmode.OptionDebug(true))
		slog.Info("socket_mode_debug", "enabled", true)
	}
	if cfg.SocketPingSec > 0 {
		d := time.Duration(cfg.SocketPingSec) * time.Second
		smOpts = append(smOpts, socketmode.OptionPingInterval(d))
		slog.Info("socket_mode_ping_interval_override", "seconds", cfg.SocketPingSec)
	}
	client := socketmode.New(api, smOpts...)

	go func() {
		for evt := range client.Events {
			switch evt.Type {
			case socketmode.EventTypeConnecting:
				metrics.SocketModeState.Set(metrics.SocketStateConnecting)
				slog.Info("socket_mode", "state", "connecting")
			case socketmode.EventTypeConnectionError:
				metrics.SocketModeState.Set(metrics.SocketStateConnectionError)
				slog.Warn("socket_mode", "state", "connection_error")
			case socketmode.EventTypeConnected:
				metrics.SocketModeState.Set(metrics.SocketStateConnected)
				slog.Info("socket_mode", "state", "connected")
			case socketmode.EventTypeHello:
				if evt.Request != nil {
					slog.Info("socket_mode_hello",
						"num_connections", evt.Request.NumConnections,
						"app_id", evt.Request.ConnectionInfo.AppID,
						"debug_host", evt.Request.DebugInfo.Host,
						"approximate_connection_time", evt.Request.DebugInfo.ApproximateConnectionTime,
						"build_number", evt.Request.DebugInfo.BuildNumber,
					)
				} else {
					slog.Info("socket_mode_hello")
				}
			case socketmode.EventTypeDisconnect:
				metrics.SocketModeState.Set(metrics.SocketStateDisconnected)
				if evt.Request != nil {
					slog.Info("socket_mode_disconnect",
						"reason", evt.Request.Reason,
						"debug_host", evt.Request.DebugInfo.Host,
						"approximate_connection_time", evt.Request.DebugInfo.ApproximateConnectionTime,
						"build_number", evt.Request.DebugInfo.BuildNumber,
					)
				} else {
					slog.Info("socket_mode", "state", "disconnect")
				}
			case socketmode.EventTypeEventsAPI:
				eventsAPI, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					slog.Warn("socket_mode_events_api_unexpected_data_type", "got", fmt.Sprintf("%T", evt.Data))
					if evt.Request != nil {
						client.Ack(*evt.Request)
						metrics.EventsAPIAckedTotal.Inc()
					}
					continue
				}
				if evt.Request == nil {
					slog.Error("socket_mode_events_api_nil_request")
					continue
				}
				client.Ack(*evt.Request)
				metrics.EventsAPIAckedTotal.Inc()
				slog.Info("orchestrator_socket_mode_event_callback",
					"inner_type", strings.TrimSpace(eventsAPI.InnerEvent.Type),
					"slack_event_id", eventsAPIEventID(eventsAPI),
					"team_id", strings.TrimSpace(eventsAPI.TeamID),
				)
				slackrun.HandleEventsAPI(ctx, cfg, eventsAPI)
			case socketmode.EventTypeErrorBadMessage:
				metrics.SocketModeBadMessageTotal.Inc()
				cause := "unknown"
				if bm, ok := evt.Data.(*socketmode.ErrorBadMessage); ok && bm != nil && bm.Cause != nil {
					cause = bm.Cause.Error()
				}
				slog.Warn("socket_mode_bad_message", "cause", cause)
			case socketmode.EventTypeIncomingError:
				if errEv, ok := evt.Data.(*slack.IncomingEventError); ok && errEv != nil && errEv.ErrorObj != nil {
					slog.Warn("socket_mode_incoming_error", "error", errEv.ErrorObj.Error())
				} else {
					slog.Warn("socket_mode_incoming_error")
				}
			case socketmode.EventTypeInvalidAuth:
				slog.Error("socket_mode_invalid_auth_check_app_token")
			default:
				// Ack other interactive events if needed
				if evt.Request != nil {
					client.Ack(*evt.Request)
				}
			}
		}
	}()

	go func() {
		if err := client.Run(); err != nil {
			slog.Error("socket_mode_run", "error", err)
			cancel()
		}
	}()

	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}

func eventsAPIEventID(ev slackevents.EventsAPIEvent) string {
	if cb, ok := ev.Data.(*slackevents.EventsAPICallbackEvent); ok && cb != nil {
		return strings.TrimSpace(cb.EventID)
	}
	return ""
}

// slackTimestampLess compares Slack message timestamps (e.g. "1776450504.274629").
func slackTimestampLess(a, b string) bool {
	fa, err1 := strconv.ParseFloat(strings.TrimSpace(a), 64)
	fb, err2 := strconv.ParseFloat(strings.TrimSpace(b), 64)
	if err1 != nil || err2 != nil {
		return strings.TrimSpace(a) < strings.TrimSpace(b)
	}
	return fa < fb
}
