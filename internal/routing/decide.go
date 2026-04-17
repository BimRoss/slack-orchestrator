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
	// DispatchMode is single vs fanout (everyone/channel caps).
	DispatchMode DispatchMode `json:"dispatch_mode"`
	// PrimaryEmployee is the canonical actor for single-target turns (first responder); empty for pure fanout.
	PrimaryEmployee string `json:"primary_employee,omitempty"`
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
	// ThreadPlainHandoffKey is the last squad-bot @mention before this message in thread history
	// (from conversations.replies). Empty when unknown or no prior squad mentions.
	ThreadPlainHandoffKey string
}

// Decide returns routing for a channel message. Priority: broadcast → explicit squad mention → plain.
func Decide(cfg DecideConfig, in Input) Decision {
	text := strings.TrimSpace(in.Text)
	bc := ClassifyBroadcastTrigger(text)
	switch bc {
	case BroadcastEveryone:
		emps := limitParticipants(cfg.Order, cfg.EveryoneLimit)
		emps = shuffleOrder(strings.TrimSpace(in.MessageTS), emps, cfg.ShuffleSecret)
		return withFanoutMeta(Decision{
			Trigger:   TriggerEveryone,
			Employees: emps,
			Kind:      KindConversation,
		})
	case BroadcastChannel:
		emps := limitParticipants(cfg.Order, cfg.ChannelLimit)
		emps = shuffleOrder(strings.TrimSpace(in.MessageTS), emps, cfg.ShuffleSecret)
		return withFanoutMeta(Decision{
			Trigger:   TriggerChannel,
			Employees: emps,
			Kind:      KindConversation,
		})
	}

	mentioned := mentionedEmployeeKeys(text, cfg.BotUserToKey, cfg.Order)
	if len(mentioned) > 0 {
		toolID, k := ClassifyToolOrConversation(text)
		if k == KindTool && toolID != "" {
			d := Decision{
				Trigger:   TriggerMention,
				Employees: mentioned,
				Kind:      KindTool,
				ToolID:    toolID,
			}
			if len(mentioned) > 1 {
				return withFanoutMeta(d)
			}
			return withSingleMeta(d)
		}
		d := Decision{
			Trigger:   TriggerMention,
			Employees: mentioned,
			Kind:      KindConversation,
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
			toolID, k := ClassifyToolOrConversation(text)
			if k == KindTool && toolID != "" {
				return withSingleMeta(Decision{
					Trigger:   TriggerPlain,
					Employees: []string{key},
					Kind:      KindTool,
					ToolID:    toolID,
				})
			}
			return withSingleMeta(Decision{
				Trigger:   TriggerPlain,
				Employees: []string{key},
				Kind:      KindConversation,
			})
		}
	}

	// Plain message → one responder: first agent after the same shuffle as @here/@channel multi-agent
	// (shuffleOrder(message_ts, roster, secret)[0]; keys vary per message like broadcast slot order).
	picked := pickPlainResponder(in.MessageTS, cfg.Order, cfg.ShuffleSecret)
	toolID, k := ClassifyToolOrConversation(text)
	if k == KindTool && toolID != "" {
		return withSingleMeta(Decision{
			Trigger:   TriggerPlain,
			Employees: []string{picked},
			Kind:      KindTool,
			ToolID:    toolID,
		})
	}
	return withSingleMeta(Decision{
		Trigger:   TriggerPlain,
		Employees: []string{picked},
		Kind:      KindConversation,
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

// SquadMentionsFromText returns employee keys for @mentions of configured squad bots in text
// (same ordering rules as routing for multi-mention roots).
func SquadMentionsFromText(text string, cfg DecideConfig) []string {
	return mentionedEmployeeKeys(text, cfg.BotUserToKey, cfg.Order)
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
