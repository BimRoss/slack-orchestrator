package slackrun

import (
	"strings"

	"github.com/slack-go/slack"
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

const orchestratorImageOnlyRoutingText = "(The user attached one or more images with no text.)"

func isSlackImageFile(f slack.File) bool {
	mt := strings.ToLower(strings.TrimSpace(f.Mimetype))
	if strings.HasPrefix(mt, "image/") {
		return true
	}
	ft := strings.ToLower(strings.TrimSpace(f.Filetype))
	switch ft {
	case "png", "jpg", "jpeg", "gif", "webp", "heic", "heif", "bmp", "tif", "tiff":
		return true
	default:
		return false
	}
}

func imageFileIDsFromFiles(files []slack.File, max int) []string {
	if max <= 0 {
		max = 8
	}
	var out []string
	for _, f := range files {
		if !isSlackImageFile(f) {
			continue
		}
		id := strings.TrimSpace(f.ID)
		if id == "" {
			continue
		}
		out = append(out, id)
		if len(out) >= max {
			break
		}
	}
	return out
}

func messageEventImageFileIDs(ev *slackevents.MessageEvent) []string {
	if ev == nil || ev.Message == nil {
		return nil
	}
	return imageFileIDsFromFiles(ev.Message.Files, 8)
}

func appMentionImageFileIDs(ev *slackevents.AppMentionEvent) []string {
	if ev == nil {
		return nil
	}
	return imageFileIDsFromFiles(ev.Files, 8)
}

// routingTextForDispatch returns text used for routing + NATS dispatch. When Slack sends no text but
// image attachments are present, a neutral placeholder keeps Decide + JetStream payloads non-empty.
func routingTextForDispatch(effText string, imageFileIDs []string) string {
	t := strings.TrimSpace(effText)
	if t != "" {
		return t
	}
	if len(imageFileIDs) > 0 {
		return orchestratorImageOnlyRoutingText
	}
	return ""
}

// routingMentionDetectionText combines message text surfaces that may contain
// canonical Slack user mention tokens (<@U...>) for routing guards. Some event
// shapes populate top-level and nested message text differently.
func routingMentionDetectionText(ev *slackevents.MessageEvent, routeText string) string {
	seen := make(map[string]struct{}, 3)
	var parts []string
	appendText := func(in string) {
		t := strings.TrimSpace(in)
		if t == "" {
			return
		}
		if _, ok := seen[t]; ok {
			return
		}
		seen[t] = struct{}{}
		parts = append(parts, t)
	}

	appendText(routeText)
	if ev != nil {
		appendText(ev.Text)
		if ev.Message != nil {
			appendText(ev.Message.Text)
		}
	}
	return strings.Join(parts, "\n")
}
