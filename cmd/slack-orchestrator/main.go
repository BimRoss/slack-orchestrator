package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bimross/slack-orchestrator/internal/config"
	"github.com/bimross/slack-orchestrator/internal/decisionlog"
	"github.com/bimross/slack-orchestrator/internal/metrics"
	"github.com/bimross/slack-orchestrator/internal/slackrun"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func main() {
	cfg := config.FromEnv()
	if cfg.LogJSON {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	}

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
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: mux}
	go func() {
		slog.Info("http_listen", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
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
	client := socketmode.New(api)

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
