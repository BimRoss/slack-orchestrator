package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bimross/slack-orchestrator/internal/config"
	"github.com/bimross/slack-orchestrator/internal/slackrun"
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

	api := slack.New(cfg.BotToken)
	client := socketmode.New(api)

	go func() {
		for evt := range client.Events {
			switch evt.Type {
			case socketmode.EventTypeConnecting:
				slog.Info("socket_mode", "state", "connecting")
			case socketmode.EventTypeConnectionError:
				slog.Warn("socket_mode", "state", "connection_error")
			case socketmode.EventTypeConnected:
				slog.Info("socket_mode", "state", "connected")
			case socketmode.EventTypeEventsAPI:
				eventsAPI, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					continue
				}
				client.Ack(*evt.Request)
				slackrun.HandleEventsAPI(ctx, cfg, eventsAPI)
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
