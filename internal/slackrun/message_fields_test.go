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

func TestRoutingTextForDispatch_imageOnly(t *testing.T) {
	if got := routingTextForDispatch("", []string{"F123"}); got != orchestratorImageOnlyRoutingText {
		t.Fatalf("got %q want %q", got, orchestratorImageOnlyRoutingText)
	}
	if got := routingTextForDispatch("", nil); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := routingTextForDispatch("  hi  ", []string{"F123"}); got != "hi" {
		t.Fatalf("got %q", got)
	}
}

func TestMessageEventImageFileIDs(t *testing.T) {
	ev := &slackevents.MessageEvent{
		Message: &slack.Msg{
			Files: []slack.File{
				{ID: "F1", Mimetype: "image/png"},
				{ID: "F2", Mimetype: "application/pdf"},
				{ID: "F3", Mimetype: "image/jpeg"},
			},
		},
	}
	got := messageEventImageFileIDs(ev)
	if len(got) != 2 || got[0] != "F1" || got[1] != "F3" {
		t.Fatalf("got %#v", got)
	}
}

func TestRoutingMentionDetectionTextIncludesNestedMessageText(t *testing.T) {
	ev := &slackevents.MessageEvent{
		Text: "@Tom W please take this",
		Message: &slack.Msg{
			Text: "<@UTOM> please take this",
		},
	}
	got := routingMentionDetectionText(ev, ev.Text)
	want := "@Tom W please take this\n<@UTOM> please take this"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRoutingMentionDetectionTextDeDupeAndTrim(t *testing.T) {
	ev := &slackevents.MessageEvent{
		Text: "  <@UTOM> hi  ",
		Message: &slack.Msg{
			Text: "<@UTOM> hi",
		},
	}
	got := routingMentionDetectionText(ev, " <@UTOM> hi ")
	if got != "<@UTOM> hi" {
		t.Fatalf("got %q", got)
	}
}
