package slackrun

import (
	"testing"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

func TestEffectiveMessageText_nestedMessage(t *testing.T) {
	ev := &slackevents.MessageEvent{
		Text: "",
		Message: &slack.Msg{
			Text: "hello from nested",
		},
	}
	if got := effectiveMessageText(ev); got != "hello from nested" {
		t.Fatalf("got %q", got)
	}
}

func TestEffectiveThreadTS_fromNested(t *testing.T) {
	ev := &slackevents.MessageEvent{
		ThreadTimeStamp: "",
		Message: &slack.Msg{
			ThreadTimestamp: "123.456",
		},
	}
	if got := effectiveThreadTS(ev); got != "123.456" {
		t.Fatalf("got %q", got)
	}
}
