package slackrun

import (
	"strings"

	"github.com/slack-go/slack/slackevents"
)

// messageRepliedSubtype is Slack's system notification when a thread receives a reply.
// The actual human reply is a separate message event with thread_ts set; see
// https://api.slack.com/events/message/message_replied
const messageRepliedSubtype = "message_replied"

func effectiveMessageText(ev *slackevents.MessageEvent) string {
	if ev == nil {
		return ""
	}
	t := strings.TrimSpace(ev.Text)
	if t != "" {
		return t
	}
	if ev.Message != nil {
		return strings.TrimSpace(ev.Message.Text)
	}
	return ""
}

func effectiveMessageUser(ev *slackevents.MessageEvent) string {
	if ev == nil {
		return ""
	}
	u := strings.TrimSpace(ev.User)
	if u != "" {
		return u
	}
	if ev.Message != nil {
		return strings.TrimSpace(ev.Message.User)
	}
	return ""
}

func effectiveBotID(ev *slackevents.MessageEvent) string {
	if ev == nil {
		return ""
	}
	if ev.BotID != "" {
		return ev.BotID
	}
	if ev.Message != nil && ev.Message.BotID != "" {
		return ev.Message.BotID
	}
	return ""
}

func effectiveThreadTS(ev *slackevents.MessageEvent) string {
	if ev == nil {
		return ""
	}
	ts := strings.TrimSpace(ev.ThreadTimeStamp)
	if ts != "" {
		return ts
	}
	if ev.Message != nil {
		return strings.TrimSpace(ev.Message.ThreadTimestamp)
	}
	return ""
}

func topLevelTextEmpty(ev *slackevents.MessageEvent) bool {
	if ev == nil {
		return true
	}
	return strings.TrimSpace(ev.Text) == ""
}
