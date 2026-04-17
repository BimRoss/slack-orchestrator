package routing

import "strings"

// ThreadMessage is one Slack thread reply in chronological order (oldest first).
type ThreadMessage struct {
	Timestamp string
	Text      string
}

// LastSquadHandoffKey returns the employee key of the last @mention of a configured squad bot
// in messages, scanning chronologically (appearance order within each message).
// Messages must be restricted to those strictly before the current inbound message.
//
// If the thread root (Timestamp == threadTS) is a workspace broadcast (<!everyone>, <!channel>, etc.),
// mentions in that root message are ignored so plain follow-ups keep the hashed random-picker
// behavior until a later message @mentions a squad bot.
func LastSquadHandoffKey(messages []ThreadMessage, threadTS string, cfg DecideConfig) string {
	if len(cfg.BotUserToKey) == 0 {
		return ""
	}
	threadTS = strings.TrimSpace(threadTS)
	var last string
	for _, m := range messages {
		ts := strings.TrimSpace(m.Timestamp)
		text := strings.TrimSpace(m.Text)
		if ts == "" || text == "" {
			continue
		}
		if ts == threadTS && ClassifyBroadcastTrigger(text) != BroadcastNone {
			continue
		}
		matches := reSlackUserMention.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			if key, ok := cfg.BotUserToKey[match[1]]; ok {
				last = key
			}
		}
	}
	return last
}
