package slackrun

import (
	"context"
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
	st := strings.TrimSpace(ev.SubType)
	if st == messageRepliedSubtype {
		// Slack sends this as a system notification about thread activity; the real reply is a separate message.* event.
		logMessageIngress(outer, "message", ev.Channel, effectiveThreadTS(ev), ev.TimeStamp, effectiveMessageUser(ev), effectiveBotID(ev), ev.SubType, ev.ChannelType, effectiveMessageText(ev), topLevelTextEmpty(ev))
		logMessageDrop(outer, "message", "subtype_message_replied_notification", ev.Channel, effectiveThreadTS(ev), ev.TimeStamp)
		return
	}

	effUser := effectiveMessageUser(ev)
	effBot := effectiveBotID(ev)
	effText := effectiveMessageText(ev)
	imgIDs := messageEventImageFileIDs(ev)
	routeText := routingTextForDispatch(effText, imgIDs)
	effThread := effectiveThreadTS(ev)
	logMessageIngress(outer, "message", ev.Channel, effThread, ev.TimeStamp, effUser, effBot, ev.SubType, ev.ChannelType, effText, topLevelTextEmpty(ev))

	if effUser == "" {
		logMessageDrop(outer, "message", "missing_user", ev.Channel, effThread, ev.TimeStamp)
		return
	}
	if effBot != "" {
		rc := routingDecideConfig(cfg)
		// Squad bots sometimes post <!channel> / <!here> (e.g. Joanne company onboarding). Treat those
		// roots like human broadcast triggers so Decide fans out to the roster instead of dropping.
		if routing.ClassifyBroadcastTrigger(effText) != routing.BroadcastNone {
			slog.Info("orchestrator_message_allow_bot_broadcast_root",
				"slack_event_id", slackEventID(outer),
				"channel_id", strings.TrimSpace(ev.Channel),
				"thread_ts", strings.TrimSpace(effThread),
				"message_ts", strings.TrimSpace(ev.TimeStamp),
				"posting_user_id", strings.TrimSpace(effUser),
			)
		} else if !routing.SquadBotMentionsOtherSquadMember(rc, effUser, effText) {
			logMessageDrop(outer, "message", "bot_or_integration_message", ev.Channel, effThread, ev.TimeStamp)
			return
		} else {
			slog.Info("orchestrator_message_allow_squad_bot_delegation",
				"slack_event_id", slackEventID(outer),
				"channel_id", strings.TrimSpace(ev.Channel),
				"thread_ts", strings.TrimSpace(effThread),
				"message_ts", strings.TrimSpace(ev.TimeStamp),
				"posting_user_id", strings.TrimSpace(effUser),
			)
		}
	}
	if st != "" && st != "thread_broadcast" {
		if st == "message_changed" || st == "message_deleted" ||
			st == "channel_join" || st == "channel_leave" ||
			st == "group_join" || st == "group_leave" {
			logMessageDrop(outer, "message", "subtype_"+st, ev.Channel, effThread, ev.TimeStamp)
			return
		}
	}
	if routeText == "" {
		logMessageDrop(outer, "message", "empty_text_after_trim", ev.Channel, effThread, ev.TimeStamp)
		return
	}
	rc := routingDecideConfig(cfg)
	if routing.HasOnlyNonSquadMentions(routeText, rc.BotUserToKey) {
		logMessageDrop(outer, "message", "human_to_human_mention", ev.Channel, effThread, ev.TimeStamp)
		return
	}
	in := routing.Input{
		ChannelID:         ev.Channel,
		ThreadTS:          effThread,
		MessageTS:         ev.TimeStamp,
		UserID:            effUser,
		Text:              routeText,
		SlackImageFileIDs: imgIDs,
	}
	if strings.TrimSpace(effThread) != "" && threadRoutingFetcher != nil && len(routing.SquadMentionsFromText(routeText, rc)) == 0 {
		routeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		handoffKey, err := threadRoutingFetcher(routeCtx, ev.Channel, effThread, ev.TimeStamp)
		cancel()
		if err != nil {
			slog.Warn("orchestrator_thread_routing_fetch_failed",
				"slack_event_id", slackEventID(outer),
				"channel_id", ev.Channel,
				"thread_ts", effThread,
				"error", err,
			)
		} else {
			in.ThreadPlainHandoffKey = handoffKey
		}
	}
	if strings.TrimSpace(effBot) == "" {
		if !posterMayUseOrchestratorRouting(ctx, effUser) {
			logMessageDrop(outer, "message", "humans_terms_not_accepted", ev.Channel, effThread, ev.TimeStamp)
			slog.Info("orchestrator_message_drop_terms",
				"slack_event_id", slackEventID(outer),
				"inner_type", "message",
				"user_id", strings.TrimSpace(effUser),
				"channel_id", strings.TrimSpace(ev.Channel),
				"thread_ts", strings.TrimSpace(effThread),
				"message_ts", strings.TrimSpace(ev.TimeStamp),
			)
			return
		}
	}
	emitDecision(ctx, cfg, outer, in, "message")
}

func handleAppMention(ctx context.Context, cfg config.Config, outer slackevents.EventsAPIEvent, ev *slackevents.AppMentionEvent) {
	if ev == nil {
		return
	}
	trim := strings.TrimSpace(ev.Text)
	logMessageIngress(outer, "app_mention", ev.Channel, ev.ThreadTimeStamp, ev.TimeStamp, ev.User, ev.BotID, "", "", trim, trim == "")

	imgIDs := appMentionImageFileIDs(ev)
	text := routingTextForDispatch(trim, imgIDs)
	if text == "" {
		logMessageDrop(outer, "app_mention", "empty_text_after_trim", ev.Channel, ev.ThreadTimeStamp, ev.TimeStamp)
		return
	}
	if !posterMayUseOrchestratorRouting(ctx, ev.User) {
		logMessageDrop(outer, "app_mention", "humans_terms_not_accepted", ev.Channel, ev.ThreadTimeStamp, ev.TimeStamp)
		slog.Info("orchestrator_message_drop_terms",
			"slack_event_id", slackEventID(outer),
			"inner_type", "app_mention",
			"user_id", strings.TrimSpace(ev.User),
			"channel_id", strings.TrimSpace(ev.Channel),
			"thread_ts", strings.TrimSpace(ev.ThreadTimeStamp),
			"message_ts", strings.TrimSpace(ev.TimeStamp),
		)
		return
	}
	in := routing.Input{
		ChannelID:         ev.Channel,
		ThreadTS:          strings.TrimSpace(ev.ThreadTimeStamp),
		MessageTS:         ev.TimeStamp,
		UserID:            ev.User,
		Text:              text,
		SlackImageFileIDs: imgIDs,
	}
	emitDecision(ctx, cfg, outer, in, "app_mention")
}

func routingDecideConfig(cfg config.Config) routing.DecideConfig {
	return routing.DecideConfig{
		Order:         cfg.MultiagentOrder,
		BotUserToKey:  cfg.BotUserToKey,
		EveryoneLimit: cfg.EveryoneLimit,
		ChannelLimit:  cfg.ChannelLimit,
		ShuffleSecret: cfg.ShuffleSecret,
	}
}

func emitDecision(ctx context.Context, cfg config.Config, outer slackevents.EventsAPIEvent, in routing.Input, innerType string) {
	rc := routingDecideConfig(cfg)
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

	slog.Info("orchestrator_routing_decision",
		"slack_event_id", slackEventID(outer),
		"inner_type", innerType,
		"channel_id", in.ChannelID,
		"thread_ts", in.ThreadTS,
		"message_ts", in.MessageTS,
		"user_id", in.UserID,
		"text_len", utf8.RuneCountInString(in.Text),
		"text_preview", truncatePreview(in.Text, textPreviewRunes),
		"decision", d,
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
// previewText is the text used for routing (effective / trimmed). topLevelTextEmpty is true when the raw API
// payload had no top-level text (e.g. text only on nested Message) — useful for diagnosing Slack payload shapes.
func logMessageIngress(outer slackevents.EventsAPIEvent, innerType, channelID, threadTS, messageTS, userID, botID, subtype, channelType, previewText string, topLevelTextEmpty bool) {
	trim := strings.TrimSpace(previewText)
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
		"top_level_text_empty", topLevelTextEmpty,
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
