package slackrun

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/bimross/slack-orchestrator/internal/config"
	"github.com/bimross/slack-orchestrator/internal/decisionlog"
	"github.com/bimross/slack-orchestrator/internal/dispatch"
	"github.com/bimross/slack-orchestrator/internal/routing"
	"github.com/slack-go/slack/slackevents"
)

var decisionLog *decisionlog.Store

// SetDecisionLog wires the in-memory decision log (optional).
func SetDecisionLog(s *decisionlog.Store) {
	decisionLog = s
}

// HandleEventsAPI logs a routing Decision for Socket Mode Events API payloads.
func HandleEventsAPI(ctx context.Context, cfg config.Config, ev slackevents.EventsAPIEvent) {
	switch inner := ev.InnerEvent.Data.(type) {
	case *slackevents.MessageEvent:
		handleMessage(ctx, cfg, ev, inner)
	case *slackevents.AppMentionEvent:
		handleAppMention(ctx, cfg, ev, inner)
	default:
		_ = ctx
		// ignore other event types in phase 1
	}
}

func handleMessage(ctx context.Context, cfg config.Config, outer slackevents.EventsAPIEvent, ev *slackevents.MessageEvent) {
	if ev.User == "" || ev.BotID != "" {
		return
	}
	st := strings.TrimSpace(ev.SubType)
	if st != "" && st != "thread_broadcast" {
		// skip edits, deletes, etc.
		if st == "message_changed" || st == "message_deleted" {
			return
		}
	}
	text := strings.TrimSpace(ev.Text)
	if text == "" {
		return
	}
	in := routing.Input{
		ChannelID: ev.Channel,
		ThreadTS:  strings.TrimSpace(ev.ThreadTimeStamp),
		MessageTS: ev.TimeStamp,
		UserID:    ev.User,
		Text:      text,
	}
	emitDecision(ctx, cfg, outer, in, "message")
}

func handleAppMention(ctx context.Context, cfg config.Config, outer slackevents.EventsAPIEvent, ev *slackevents.AppMentionEvent) {
	text := strings.TrimSpace(ev.Text)
	if text == "" {
		return
	}
	in := routing.Input{
		ChannelID: ev.Channel,
		ThreadTS:  strings.TrimSpace(ev.ThreadTimeStamp),
		MessageTS: ev.TimeStamp,
		UserID:    ev.User,
		Text:      text,
	}
	emitDecision(ctx, cfg, outer, in, "app_mention")
}

func emitDecision(ctx context.Context, cfg config.Config, outer slackevents.EventsAPIEvent, in routing.Input, innerType string) {
	rc := routing.DecideConfig{
		Order:         cfg.MultiagentOrder,
		BotUserToKey:  cfg.BotUserToKey,
		EveryoneLimit: cfg.EveryoneLimit,
		ChannelLimit:  cfg.ChannelLimit,
		ShuffleSecret: cfg.ShuffleSecret,
	}
	d := routing.Decide(rc, in)
	results := dispatch.Decision(ctx, cfg, outer, in, d, innerType)
	note := dispatchNote(cfg, d)

	if decisionLog != nil {
		decisionLog.Append(decisionlog.Entry{
			Time:            time.Now().UTC(),
			InnerType:       innerType,
			ChannelID:       in.ChannelID,
			ThreadTS:        in.ThreadTS,
			MessageTS:       in.MessageTS,
			UserID:          in.UserID,
			TextPreview:     in.Text,
			Decision:        d,
			DispatchNote:    note,
			DispatchResults: results,
		})
	}

	if cfg.LogJSON {
		b, _ := json.Marshal(struct {
			ChannelID string           `json:"channel_id"`
			ThreadTS  string           `json:"thread_ts"`
			MessageTS string           `json:"message_ts"`
			UserID    string           `json:"user_id"`
			Decision  routing.Decision `json:"decision"`
		}{
			ChannelID: in.ChannelID,
			ThreadTS:  in.ThreadTS,
			MessageTS: in.MessageTS,
			UserID:    in.UserID,
			Decision:  d,
		})
		slog.Info(string(b))
		return
	}
	slog.Info("routing_decision",
		"channel_id", in.ChannelID,
		"thread_ts", in.ThreadTS,
		"message_ts", in.MessageTS,
		"user_id", in.UserID,
		"trigger", d.Trigger,
		"employees", strings.Join(d.Employees, ","),
		"kind", d.Kind,
		"tool_id", d.ToolID,
	)
}

func dispatchNote(cfg config.Config, d routing.Decision) string {
	if !cfg.DispatchEnabled {
		return "dispatch_disabled"
	}
	if strings.TrimSpace(cfg.NatsURL) == "" {
		return "no_nats_url"
	}
	if len(d.Employees) == 0 {
		return "no_employees"
	}
	return ""
}
