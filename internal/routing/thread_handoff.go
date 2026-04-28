package routing

import "strings"

// ThreadMessage is one Slack thread reply in chronological order (oldest first).
type ThreadMessage struct {
	Timestamp string
	Text      string
}

// IsCreateCompanySlackPostConfirmationText is true for Joanne’s #humans epilogue after create-company:
// "Created: …" plus "Invited: …" with <@U…> roster. That line lists cofounders and auto-invited squad
// user ids; it is not a handoff to the last-mentioned agent. See employee-factory
// companySlackPostCreateConfirmationEpilogue.
func IsCreateCompanySlackPostConfirmationText(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	// Keep in sync with companySlackPostCreateConfirmationEpilogue (mrkdwn channel link on first line).
	if !strings.HasPrefix(t, "Created:") {
		return false
	}
	return strings.Contains(t, "\nInvited:")
}

// LastSquadHandoffKey returns the employee key of the last @mention of a configured squad bot
// in messages, scanning chronologically (appearance order within each message).
// Messages must be restricted to those strictly before the current inbound message.
//
// If the thread root (Timestamp == threadTS) is a workspace broadcast (<!everyone>, <!here>, <!channel>, etc.),
// mentions in that root message are ignored so plain follow-ups keep the hashed random-picker
// behavior until a later message @mentions a squad bot.
//
// If the thread root is Joanne’s create-company confirmation (Created: + Invited: roster), squad
// mentions in that root are also ignored for the same reason: the <@U…> list is an invite list, not
// a routing handoff.
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
		if ts == threadTS && IsCreateCompanySlackPostConfirmationText(text) {
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
