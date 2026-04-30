package routing

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
	"regexp"
	"strings"
)

// Kind is conversation vs tool routing intent.
type Kind string

const (
	KindConversation Kind = "conversation"
	KindTool         Kind = "tool"
)

// Trigger describes why this routing decision was made.
type Trigger string

const (
	TriggerEveryone Trigger = "everyone"
	TriggerChannel  Trigger = "channel"
	TriggerMention  Trigger = "mention"
	TriggerPlain    Trigger = "plain"
)

const ClassificationReasonThreadHandoffToolBlocked = "thread_handoff_tool_requires_explicit_mention"

// DispatchMode describes how many NATS targets were chosen (observability + worker hints).
type DispatchMode string

const (
	// DispatchModeSingle: one primary actor; NATS publishes to Employees (length 1 for normal turns).
	DispatchModeSingle DispatchMode = "single"
	// DispatchModeFanout: multiple workers (e.g. @everyone / @channel multi-agent).
	DispatchModeFanout DispatchMode = "fanout"
)

// Decision is the orchestrator output for one normalized message.
type Decision struct {
	Trigger   Trigger  `json:"trigger"`
	Employees []string `json:"employees"`
	Kind      Kind     `json:"kind"`
	ToolID    string   `json:"tool_id,omitempty"`
	// ClassificationReason explains how Tier-1 classified tool vs conversation.
	ClassificationReason string `json:"classification_reason,omitempty"`
	// DispatchMode is single vs fanout (everyone/channel caps).
	DispatchMode DispatchMode `json:"dispatch_mode"`
	// PrimaryEmployee is the canonical actor for single-target turns (first responder); empty for pure fanout.
	PrimaryEmployee string `json:"primary_employee,omitempty"`

	// ExecutionMode is empty for legacy decisions, or ExecutionModePipeline for ordered multi-step chains.
	ExecutionMode     string         `json:"execution_mode,omitempty"`
	PipelineSteps     []PipelineStep `json:"pipeline_steps,omitempty"`
	PipelineStepIndex int            `json:"pipeline_step_index,omitempty"`
	ChainID           string         `json:"chain_id,omitempty"`
}

// Slack user mentions in message text (bot user ids are typically U…; include A for app-style ids when present).
var reSlackUserMention = regexp.MustCompile(`<@([UWA][A-Za-z0-9]+)>`)

// Input is a normalized Slack message.
type Input struct {
	ChannelID string
	ThreadTS  string
	MessageTS string
	UserID    string
	Text      string
	// SlackImageFileIDs are Slack file IDs for image attachments on this message (from Events API).
	// Published on the JetStream envelope so workers can files.info + download with the bot token.
	SlackImageFileIDs []string
	// ThreadPlainHandoffKey is the last squad-bot @mention before this message in thread history
	// (from conversations.replies). Empty when unknown or no prior squad mentions.
	ThreadPlainHandoffKey string
	// ThreadRootText is the Slack text of the thread parent (empty for top-level messages). Used to
	// suppress automated squad replies on plain follow-ups under Joanne’s create-company confirmation.
	ThreadRootText string
	// PreAcceptanceTermsBypass is set by slackrun when the poster has not stored #humans terms
	// acceptance but the text matches UpdateTermsIntentText. Decide must route only to Joanne as
	// the update-terms tool (the sole allowed agent turn before acceptance).
	PreAcceptanceTermsBypass bool
}

// Decide returns routing for a channel message. Priority: broadcast → explicit squad mention → plain.
func Decide(cfg DecideConfig, in Input) Decision {
	if in.PreAcceptanceTermsBypass {
		return withSingleMeta(Decision{
			Trigger:   TriggerPlain,
			Employees: []string{"joanne"},
			Kind:      KindTool,
			ToolID:    "update-terms",
		})
	}
	text := strings.TrimSpace(in.Text)
	bc := ClassifyBroadcastTrigger(text)
	switch bc {
	case BroadcastEveryone:
		order := orderExcludingBroadcastPoster(cfg, in.UserID)
		emps := limitParticipants(order, cfg.EveryoneLimit)
		emps = shuffleOrder(strings.TrimSpace(in.MessageTS), emps, cfg.ShuffleSecret)
		return withFanoutMeta(Decision{
			Trigger:   TriggerEveryone,
			Employees: emps,
			Kind:      KindConversation,
		})
	case BroadcastChannel:
		order := orderExcludingBroadcastPoster(cfg, in.UserID)
		emps := limitParticipants(order, cfg.ChannelLimit)
		emps = shuffleOrder(strings.TrimSpace(in.MessageTS), emps, cfg.ShuffleSecret)
		return withFanoutMeta(Decision{
			Trigger:   TriggerChannel,
			Employees: emps,
			Kind:      KindConversation,
		})
	}

	mentioned := mentionedEmployeeKeys(text, cfg.BotUserToKey, cfg.Order)
	if len(mentioned) > 0 {
		// Joanne posts the create-company epilogue: "Created:…\nInvited: <@U…>, …" with <@U…> for humans,
		// orchestrator, and squad. Many of those tokens do not map to BotUserToKey, so |mentioned| is often
		// 1 (e.g. only Alex) and the len(mentioned)>=2 roster guard below does not run—Alex would still
		// be routed. This line is a status roster, not a handoff: do not dispatch any squad.
		if IsCreateCompanySlackPostConfirmationText(text) {
			if _, ok := cfg.BotUserToKey[strings.TrimSpace(in.UserID)]; ok {
				return Decision{}
			}
		}
		// Guard roster/status posts from squad bots ("Participants: <@...>, <@...>") so they do not
		// fan out into multi-agent chat. Directed callouts in normal prose still route by mention.
		if posterKey, ok := cfg.BotUserToKey[strings.TrimSpace(in.UserID)]; ok && posterKey != "" && len(mentioned) >= 2 && looksLikeParticipantRosterText(text) {
			toolID, k, reason := ClassifyToolOrConversationWithReason(text)
			if k == KindTool && toolID != "" {
				return withSingleMeta(Decision{
					Trigger:              TriggerPlain,
					Employees:            []string{posterKey},
					Kind:                 KindTool,
					ToolID:               toolID,
					ClassificationReason: reason,
				})
			}
			return withSingleMeta(Decision{
				Trigger:              TriggerPlain,
				Employees:            []string{posterKey},
				Kind:                 KindConversation,
				ClassificationReason: reason,
			})
		}
		if pd, ok := TryPipelineDecision(cfg, in); ok {
			return pd
		}
		toolID, k, reason := ClassifyToolOrConversationWithReason(text)
		if k == KindTool && toolID != "" {
			d := Decision{
				Trigger:              TriggerMention,
				Employees:            mentioned,
				Kind:                 KindTool,
				ToolID:               toolID,
				ClassificationReason: reason,
			}
			if len(mentioned) > 1 {
				return withFanoutMeta(d)
			}
			return withSingleMeta(d)
		}
		d := Decision{
			Trigger:              TriggerMention,
			Employees:            mentioned,
			Kind:                 KindConversation,
			ClassificationReason: reason,
		}
		if len(mentioned) > 1 {
			return withFanoutMeta(d)
		}
		return withSingleMeta(d)
	}

	// Plain thread reply: continue with the last squad bot @mentioned earlier in this thread
	// (handoff). Thread history is scanned in slackrun (conversations.replies); broadcast roots
	// do not pin a bot until a later explicit @mention (see LastSquadHandoffKey).
	if strings.TrimSpace(in.ThreadTS) != "" {
		key := strings.TrimSpace(in.ThreadPlainHandoffKey)
		if key != "" {
			toolID, k, reason := ClassifyToolOrConversationWithReason(text)
			if k == KindTool && toolID != "" {
				return withSingleMeta(Decision{
					Trigger:              TriggerPlain,
					Employees:            []string{key},
					Kind:                 KindConversation,
					ClassificationReason: ClassificationReasonThreadHandoffToolBlocked,
				})
			}
			return withSingleMeta(Decision{
				Trigger:              TriggerPlain,
				Employees:            []string{key},
				Kind:                 KindConversation,
				ClassificationReason: reason,
			})
		}
	}

	// Plain reply in a thread under Joanne’s "Created: … / Invited: …" post: do not route to any squad
	// agent (the Invited line is a roster, not a conversation kickoff). Explicit @mentions in the
	// reply are handled above. When thread routing is disabled, ThreadRootText is empty and this is a no-op.
	if strings.TrimSpace(in.ThreadTS) != "" && len(mentioned) == 0 && IsCreateCompanySlackPostConfirmationText(in.ThreadRootText) {
		return Decision{}
	}

	// Plain message → one responder: first agent after the same shuffle as @here/@channel multi-agent
	// (shuffleOrder(message_ts, roster, secret)[0]; keys vary per message like broadcast slot order).
	picked := pickPlainResponder(in.MessageTS, cfg.Order, cfg.ShuffleSecret)
	toolID, k, reason := ClassifyToolOrConversationWithReason(text)
	if k == KindTool && toolID != "" {
		return withSingleMeta(Decision{
			Trigger:              TriggerPlain,
			Employees:            []string{picked},
			Kind:                 KindTool,
			ToolID:               toolID,
			ClassificationReason: reason,
		})
	}
	return withSingleMeta(Decision{
		Trigger:              TriggerPlain,
		Employees:            []string{picked},
		Kind:                 KindConversation,
		ClassificationReason: reason,
	})
}

func withSingleMeta(d Decision) Decision {
	d.DispatchMode = DispatchModeSingle
	if len(d.Employees) > 0 {
		d.PrimaryEmployee = strings.ToLower(strings.TrimSpace(d.Employees[0]))
	}
	return d
}

func withFanoutMeta(d Decision) Decision {
	d.DispatchMode = DispatchModeFanout
	if len(d.Employees) > 0 {
		d.PrimaryEmployee = strings.ToLower(strings.TrimSpace(d.Employees[0]))
	}
	return d
}

// DecideConfig is routing configuration subset.
type DecideConfig struct {
	Order         []string
	BotUserToKey  map[string]string
	EveryoneLimit int
	ChannelLimit  int
	ShuffleSecret string
}

func limitParticipants(order []string, limit int) []string {
	if limit <= 0 || len(order) == 0 {
		return nil
	}
	if len(order) <= limit {
		out := make([]string, len(order))
		copy(out, order)
		return out
	}
	out := make([]string, limit)
	copy(out, order[:limit])
	return out
}

// orderExcludingBroadcastPoster returns cfg.Order with the message author removed when they map to a
// squad employee key (via BotUserToKey). Human authors and unknown user ids keep the full order so
// @channel / @here behave unchanged; squad bots do not receive their own broadcast fan-out.
func orderExcludingBroadcastPoster(cfg DecideConfig, posterUserID string) []string {
	uid := strings.TrimSpace(posterUserID)
	if uid == "" || len(cfg.BotUserToKey) == 0 {
		return cfg.Order
	}
	posterKey, ok := cfg.BotUserToKey[uid]
	if !ok {
		return cfg.Order
	}
	posterKey = strings.ToLower(strings.TrimSpace(posterKey))
	if posterKey == "" {
		return cfg.Order
	}
	var out []string
	for _, k := range cfg.Order {
		kk := strings.ToLower(strings.TrimSpace(k))
		if kk == posterKey {
			continue
		}
		out = append(out, k)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// SquadMentionsFromText returns employee keys for @mentions of configured squad bots in text
// (same ordering rules as routing for multi-mention roots).
func SquadMentionsFromText(text string, cfg DecideConfig) []string {
	return mentionedEmployeeKeys(text, cfg.BotUserToKey, cfg.Order)
}

// SquadBotMentionsOtherSquadMember is true when the message is authored by a configured squad bot
// (posting user id appears in BotUserToKey) and the text @mentions a different squad bot by user id.
// Employee-factory posts this pattern for capability delegation (e.g. Tim sends @Joanne with the
// operator's text). Without an exception, slack-orchestrator drops every message with bot_id before
// NATS dispatch, so the specialist never runs.
//
// Product note: the mention must be a real Slack token (<@U…>), not a display name; use exactly one
// other-squad <@> on delegating bot posts (see README “Squad bot posts and cross-bot delegation”).
func SquadBotMentionsOtherSquadMember(cfg DecideConfig, postingUserID, text string) bool {
	postingUserID = strings.TrimSpace(postingUserID)
	if postingUserID == "" || len(cfg.BotUserToKey) == 0 {
		return false
	}
	posterKey, ok := cfg.BotUserToKey[postingUserID]
	if !ok {
		return false
	}
	matches := reSlackUserMention.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		uid := m[1]
		if uid == postingUserID {
			continue
		}
		if key, ok := cfg.BotUserToKey[uid]; ok && !strings.EqualFold(key, posterKey) {
			return true
		}
	}
	return false
}

// HasOnlyNonSquadMentions returns true when text contains at least one Slack @mention
// and none of those mentions map to configured squad bots.
func HasOnlyNonSquadMentions(text string, botUserToKey map[string]string) bool {
	matches := reSlackUserMention.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return false
	}
	hasNonSquad := false
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		uid := strings.TrimSpace(m[1])
		if uid == "" {
			continue
		}
		if _, ok := botUserToKey[uid]; ok {
			return false
		}
		hasNonSquad = true
	}
	return hasNonSquad
}

func mentionedEmployeeKeys(text string, botUserToKey map[string]string, order []string) []string {
	if len(botUserToKey) == 0 {
		return nil
	}
	matches := reSlackUserMention.FindAllStringSubmatch(text, -1)
	seen := make(map[string]bool)
	var keys []string
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		id := m[1]
		key, ok := botUserToKey[id]
		if !ok || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	// Stabilize order against MULTIAGENT_ORDER
	if len(keys) <= 1 || len(order) == 0 {
		return keys
	}
	pos := make(map[string]int, len(order))
	for i, k := range order {
		pos[strings.ToLower(k)] = i
	}
	// simple sort by order index
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if pos[strings.ToLower(keys[i])] > pos[strings.ToLower(keys[j])] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

// shuffleOrder returns a deterministic permutation of order for this trigger.
// All processes compute the same sequence from anchorTS + order + secret (SHA-256 seed), matching
// employee-factory shuffleBroadcastParticipants.
func shuffleOrder(anchorTS string, order []string, secret string) []string {
	if len(order) == 0 {
		return nil
	}
	out := make([]string, len(order))
	copy(out, order)
	if len(out) <= 1 {
		return out
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(anchorTS))
	b.WriteByte(0)
	b.WriteString(strings.Join(order, ","))
	b.WriteByte(0)
	b.WriteString(secret)
	sum := sha256.Sum256([]byte(b.String()))
	seed := int64(binary.BigEndian.Uint64(sum[:8]))
	rng := rand.New(rand.NewSource(seed))
	for i := len(out) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// pickPlainResponder chooses who answers a plain message: first slot after shuffling the roster
// by message_ts, matching broadcast multi-agent ordering (shuffleOrder anchor = root message ts there).
func pickPlainResponder(messageTS string, order []string, secret string) string {
	if len(order) == 0 {
		return ""
	}
	shuffled := shuffleOrder(strings.TrimSpace(messageTS), order, secret)
	if len(shuffled) == 0 {
		return ""
	}
	return shuffled[0]
}

func looksLikeParticipantRosterText(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return strings.Contains(lower, "participants:") || strings.Contains(lower, "\nparticipants:")
}
