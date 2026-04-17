package routing

import (
	"crypto/sha256"
	"encoding/binary"
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
}

// Decide returns routing for a channel message. Priority: broadcast → explicit squad mention → plain.
func Decide(cfg DecideConfig, in Input) Decision {
	text := strings.TrimSpace(in.Text)
	bc := ClassifyBroadcastTrigger(text)
	switch bc {
	case BroadcastEveryone:
		emps := limitParticipants(cfg.Order, cfg.EveryoneLimit)
		return withFanoutMeta(Decision{
			Trigger:   TriggerEveryone,
			Employees: emps,
			Kind:      KindConversation,
		})
	case BroadcastChannel:
		emps := limitParticipants(cfg.Order, cfg.ChannelLimit)
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
			return withSingleMeta(Decision{
				Trigger:   TriggerMention,
				Employees: []string{mentioned[0]},
				Kind:      KindTool,
				ToolID:    toolID,
			})
		}
		return withSingleMeta(Decision{
			Trigger:   TriggerMention,
			Employees: []string{mentioned[0]},
			Kind:      KindConversation,
		})
	}

	// Plain message → one pseudo-random agent (deterministic from thread + message + secret).
	// Channel-root and thread replies use the same pick — matches employee-factory
	// selectBroadcastThreadFollowupResponder / pickPlainResponder hash (prefix "plain-followup").
	picked := pickPlainResponder(in.ThreadTS, in.MessageTS, cfg.Order, cfg.ShuffleSecret)
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

func pickPlainResponder(threadTS, messageTS string, order []string, secret string) string {
	if len(order) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("plain-followup")
	b.WriteByte(0)
	b.WriteString(strings.TrimSpace(threadTS))
	b.WriteByte(0)
	b.WriteString(strings.TrimSpace(messageTS))
	b.WriteByte(0)
	b.WriteString(strings.Join(order, ","))
	b.WriteByte(0)
	b.WriteString(secret)
	sum := sha256.Sum256([]byte(b.String()))
	idx := int(binary.BigEndian.Uint64(sum[:8]) % uint64(len(order)))
	return order[idx]
}
