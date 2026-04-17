package slackrun

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bimross/slack-orchestrator/internal/config"
	"github.com/bimross/slack-orchestrator/internal/decisionlog"
	"github.com/bimross/slack-orchestrator/internal/dispatch"
	"github.com/bimross/slack-orchestrator/internal/routing"
	"github.com/slack-go/slack/slackevents"
)

// textPreviewRunes caps logged text previews (full text still drives routing when non-empty).
const textPreviewRunes = 100

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
		slog.Info("orchestrator_events_api_inner_unhandled",
			"slack_event_id", slackEventID(ev),
			"inner_type", strings.TrimSpace(ev.InnerEvent.Type),
		)
	}
}

func handleMessage(ctx context.Context, cfg config.Config, outer slackevents.EventsAPIEvent, ev *slackevents.MessageEvent) {
	if ev == nil {
		return
	}
	logMessageIngress(outer, "message", ev.Channel, ev.ThreadTimeStamp, ev.TimeStamp, ev.User, ev.BotID, ev.SubType, ev.ChannelType, ev.Text)

	if ev.User == "" || ev.BotID != "" {
		reason := "missing_user"
		if ev.BotID != "" {
			reason = "bot_or_integration_message"
		}
		logMessageDrop(outer, "message", reason, ev.Channel, ev.ThreadTimeStamp, ev.TimeStamp)
		return
	}
	st := strings.TrimSpace(ev.SubType)
	if st != "" && st != "thread_broadcast" {
		if st == "message_changed" || st == "message_deleted" {
			logMessageDrop(outer, "message", "subtype_"+st, ev.Channel, ev.ThreadTimeStamp, ev.TimeStamp)
			return
		}
	}
	text := strings.TrimSpace(ev.Text)
	if text == "" {
		logMessageDrop(outer, "message", "empty_text_after_trim", ev.Channel, ev.ThreadTimeStamp, ev.TimeStamp)
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
	if ev == nil {
		return
	}
	logMessageIngress(outer, "app_mention", ev.Channel, ev.ThreadTimeStamp, ev.TimeStamp, ev.User, ev.BotID, "", "", ev.Text)

	text := strings.TrimSpace(ev.Text)
	if text == "" {
		logMessageDrop(outer, "app_mention", "empty_text_after_trim", ev.Channel, ev.ThreadTimeStamp, ev.TimeStamp)
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
			Event        string           `json:"event"`
			SlackEventID string           `json:"slack_event_id,omitempty"`
			InnerType    string           `json:"inner_type"`
			ChannelID    string           `json:"channel_id"`
			ThreadTS     string           `json:"thread_ts"`
			MessageTS    string           `json:"message_ts"`
			UserID       string           `json:"user_id"`
			TextLen      int              `json:"text_len"`
			TextPreview  string           `json:"text_preview"`
			Decision     routing.Decision `json:"decision"`
			DispatchNote string           `json:"dispatch_note,omitempty"`
		}{
			Event:        "orchestrator_routing_decision",
			SlackEventID: slackEventID(outer),
			InnerType:    innerType,
			ChannelID:    in.ChannelID,
			ThreadTS:     in.ThreadTS,
			MessageTS:    in.MessageTS,
			UserID:       in.UserID,
			TextLen:      utf8.RuneCountInString(in.Text),
			TextPreview:  truncatePreview(in.Text, textPreviewRunes),
			Decision:     d,
			DispatchNote: note,
		})
		slog.Info(string(b))
		return
	}
	slog.Info("orchestrator_routing_decision",
		"slack_event_id", slackEventID(outer),
		"inner_type", innerType,
		"channel_id", in.ChannelID,
		"thread_ts", in.ThreadTS,
		"message_ts", in.MessageTS,
		"user_id", in.UserID,
		"text_len", utf8.RuneCountInString(in.Text),
		"text_preview", truncatePreview(in.Text, textPreviewRunes),
		"trigger", d.Trigger,
		"employees", strings.Join(d.Employees, ","),
		"kind", d.Kind,
		"tool_id", d.ToolID,
		"dispatch_mode", d.DispatchMode,
		"primary_employee", d.PrimaryEmployee,
		"dispatch_note", note,
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

func slackEventID(ev slackevents.EventsAPIEvent) string {
	if cb, ok := ev.Data.(*slackevents.EventsAPICallbackEvent); ok && cb != nil {
		return strings.TrimSpace(cb.EventID)
	}
	return ""
}

// logMessageIngress records every Slack message-shaped event before routing filters (includes bots and empty text).
func logMessageIngress(outer slackevents.EventsAPIEvent, innerType, channelID, threadTS, messageTS, userID, botID, subtype, channelType, rawText string) {
	trim := strings.TrimSpace(rawText)
	slog.Info("orchestrator_message_ingress",
		"slack_event_id", slackEventID(outer),
		"inner_type", innerType,
		"channel_id", strings.TrimSpace(channelID),
		"thread_ts", strings.TrimSpace(threadTS),
		"message_ts", strings.TrimSpace(messageTS),
		"user_id", strings.TrimSpace(userID),
		"bot_id", strings.TrimSpace(botID),
		"subtype", strings.TrimSpace(subtype),
		"channel_type", strings.TrimSpace(channelType),
		"text_len", utf8.RuneCountInString(trim),
		"text_preview", truncatePreview(trim, textPreviewRunes),
	)
}

func logMessageDrop(outer slackevents.EventsAPIEvent, innerType, reason, channelID, threadTS, messageTS string) {
	slog.Info("orchestrator_message_drop",
		"slack_event_id", slackEventID(outer),
		"inner_type", innerType,
		"reason", reason,
		"channel_id", strings.TrimSpace(channelID),
		"thread_ts", strings.TrimSpace(threadTS),
		"message_ts", strings.TrimSpace(messageTS),
	)
}

func truncatePreview(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if maxRunes <= 0 || s == "" {
		return s
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}
