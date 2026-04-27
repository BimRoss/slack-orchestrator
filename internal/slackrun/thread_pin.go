package slackrun

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/bimross/slack-orchestrator/internal/config"
	"github.com/bimross/slack-orchestrator/internal/routing"
	"github.com/bimross/slack-orchestrator/internal/threadpin"
	"github.com/slack-go/slack/slackevents"
)

var threadPinStore *threadpin.Store

// SetThreadPinStore wires optional Redis-backed thread follow-up routing (nil disables).
func SetThreadPinStore(s *threadpin.Store) {
	threadPinStore = s
}

func teamIDFromEventsAPI(ev slackevents.EventsAPIEvent) string {
	if cb, ok := ev.Data.(*slackevents.EventsAPICallbackEvent); ok && cb != nil {
		return strings.TrimSpace(cb.TeamID)
	}
	return strings.TrimSpace(ev.TeamID)
}

// threadRootAnchorForSkillPin returns the Slack thread root timestamp to key Redis pins.
// Empty when this inbound message is not the thread root / channel parent that starts a thread.
func threadRootAnchorForSkillPin(in routing.Input) string {
	ts := strings.TrimSpace(in.ThreadTS)
	msg := strings.TrimSpace(in.MessageTS)
	if msg == "" {
		return ""
	}
	if ts == "" {
		return msg
	}
	if ts == msg {
		return ts
	}
	return ""
}

func mergeThreadHandoffWithSkillPin(ctx context.Context, outer slackevents.EventsAPIEvent, cfg config.Config, effThread string, in *routing.Input) {
	if threadPinStore == nil || in == nil {
		return
	}
	if strings.TrimSpace(effThread) == "" || strings.TrimSpace(in.ThreadPlainHandoffKey) != "" {
		return
	}
	rc := routingDecideConfig(cfg)
	if len(routing.SquadMentionsFromText(in.Text, rc)) > 0 {
		return
	}
	team := teamIDFromEventsAPI(outer)
	if team == "" {
		return
	}
	getCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	k, err := threadPinStore.GetFollowupEmployee(getCtx, team, in.ChannelID, strings.TrimSpace(effThread))
	if err != nil {
		slog.Warn("orchestrator_thread_pin_get_failed",
			"error", err,
			"channel_id", strings.TrimSpace(in.ChannelID),
			"thread_ts", strings.TrimSpace(effThread),
		)
		return
	}
	if k == "" {
		return
	}
	in.ThreadPlainHandoffKey = k
	slog.Info("orchestrator_thread_handoff_from_pin",
		"slack_event_id", slackEventID(outer),
		"channel_id", strings.TrimSpace(in.ChannelID),
		"thread_ts", strings.TrimSpace(effThread),
		"employee", k,
	)
}

func maybePersistThreadSkillPin(ctx context.Context, cfg config.Config, outer slackevents.EventsAPIEvent, in routing.Input, d routing.Decision) {
	if threadPinStore == nil {
		return
	}
	if !cfg.DispatchEnabled || strings.TrimSpace(cfg.NatsURL) == "" {
		return
	}
	if len(d.Employees) != 1 || d.Trigger != routing.TriggerMention {
		return
	}
	if d.Kind != routing.KindTool || !routing.ToolPinsThreadSkillFollowup(d.ToolID) {
		return
	}
	rootAnchor := threadRootAnchorForSkillPin(in)
	if rootAnchor == "" {
		return
	}
	team := teamIDFromEventsAPI(outer)
	if team == "" {
		slog.Warn("orchestrator_thread_pin_set_skip_missing_team",
			"channel_id", strings.TrimSpace(in.ChannelID),
			"thread_root_ts", rootAnchor,
		)
		return
	}
	emp := strings.ToLower(strings.TrimSpace(d.Employees[0]))
	if emp == "" {
		return
	}
	setCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := threadPinStore.SetFollowupEmployee(setCtx, team, in.ChannelID, rootAnchor, emp); err != nil {
		slog.Warn("orchestrator_thread_pin_set_failed",
			"error", err,
			"channel_id", strings.TrimSpace(in.ChannelID),
			"thread_root_ts", rootAnchor,
			"employee", emp,
		)
		return
	}
	slog.Info("orchestrator_thread_pin_set",
		"slack_event_id", slackEventID(outer),
		"channel_id", strings.TrimSpace(in.ChannelID),
		"thread_root_ts", rootAnchor,
		"employee", emp,
		"tool_id", strings.TrimSpace(d.ToolID),
	)
}
