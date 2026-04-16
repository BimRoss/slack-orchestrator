package dispatch

import (
	"context"
	"testing"

	"github.com/bimross/slack-orchestrator/internal/config"
	"github.com/bimross/slack-orchestrator/internal/routing"
	"github.com/slack-go/slack/slackevents"
)

func TestDecision_SkipsWhenNatsURLMissing(t *testing.T) {
	cfg := config.Config{
		DispatchEnabled: true,
		NatsURL:         "",
		NatsStream:      "SLACK_WORK",
	}
	outer := slackevents.EventsAPIEvent{
		Data: &slackevents.EventsAPICallbackEvent{EventID: "Ev123"},
	}
	in := routing.Input{ChannelID: "C1", MessageTS: "1.0", UserID: "U1", Text: "hi"}
	d := routing.Decision{Employees: []string{"alex"}, Trigger: routing.TriggerPlain, Kind: routing.KindConversation}
	Decision(context.Background(), cfg, outer, in, d, "message")
}
