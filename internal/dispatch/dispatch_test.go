package dispatch

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bimross/slack-orchestrator/internal/config"
	"github.com/bimross/slack-orchestrator/internal/inbound"
	"github.com/bimross/slack-orchestrator/internal/routing"
	"github.com/nats-io/nats.go"
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

type fakeJetStream struct {
	streamExists bool
	published    []fakePublish
}

type fakePublish struct {
	subject string
	data    []byte
}

func (f *fakeJetStream) Publish(subject string, data []byte, _ ...nats.PubOpt) (*nats.PubAck, error) {
	f.published = append(f.published, fakePublish{subject: subject, data: data})
	return &nats.PubAck{Stream: "SLACK_WORK", Sequence: uint64(len(f.published))}, nil
}

func (f *fakeJetStream) StreamInfo(_ string, _ ...nats.JSOpt) (*nats.StreamInfo, error) {
	if !f.streamExists {
		return nil, nats.ErrStreamNotFound
	}
	return &nats.StreamInfo{}, nil
}

func (f *fakeJetStream) AddStream(_ *nats.StreamConfig, _ ...nats.JSOpt) (*nats.StreamInfo, error) {
	f.streamExists = true
	return &nats.StreamInfo{}, nil
}

func TestDecision_MultiMentionToolFanoutPublishesPerEmployee(t *testing.T) {
	fake := &fakeJetStream{}
	origJet := jetStreamContextFn
	origEnsure := ensureStreamFn
	jetStreamContextFn = func(_ config.Config) (jetStreamClient, error) { return fake, nil }
	ensureStreamFn = ensureStream
	t.Cleanup(func() {
		jetStreamContextFn = origJet
		ensureStreamFn = origEnsure
	})

	cfg := config.Config{
		DispatchEnabled: true,
		NatsURL:         "nats://stubbed",
		NatsStream:      "SLACK_WORK",
	}
	outer := slackevents.EventsAPIEvent{
		Data: &slackevents.EventsAPICallbackEvent{EventID: "EvMentionTool"},
	}
	in := routing.Input{
		ChannelID: "C123",
		ThreadTS:  "177.1",
		MessageTS: "177.2",
		UserID:    "UHuman",
		Text:      "<@UJOANNE> <@UROSS> search twitter for AI creators",
	}
	d := routing.Decision{
		Trigger:      routing.TriggerMention,
		Employees:    []string{"ross", "joanne"},
		Kind:         routing.KindTool,
		ToolID:       "read-twitter",
		DispatchMode: routing.DispatchModeFanout,
	}

	results := Decision(context.Background(), cfg, outer, in, d, "message")
	if len(results) != 2 {
		t.Fatalf("expected 2 dispatch results, got %d (%+v)", len(results), results)
	}
	if len(fake.published) != 2 {
		t.Fatalf("expected 2 published messages, got %d", len(fake.published))
	}

	gotSubjects := map[string]bool{}
	gotTargets := map[string]bool{}
	for _, p := range fake.published {
		gotSubjects[p.subject] = true
		var evt inbound.EventV1
		if err := json.Unmarshal(p.data, &evt); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		if evt.Decision.Kind != routing.KindTool || evt.Decision.ToolID != "read-twitter" {
			t.Fatalf("expected tool routing payload, got %+v", evt.Decision)
		}
		gotTargets[evt.TargetEmployee] = true
	}
	if !gotSubjects["slack.work.ross.events"] || !gotSubjects["slack.work.joanne.events"] {
		t.Fatalf("unexpected publish subjects: %+v", gotSubjects)
	}
	if !gotTargets["ross"] || !gotTargets["joanne"] {
		t.Fatalf("unexpected target employees in payloads: %+v", gotTargets)
	}
}

func TestDecision_DedupesDuplicateEmployeesBeforePublish(t *testing.T) {
	fake := &fakeJetStream{}
	origJet := jetStreamContextFn
	origEnsure := ensureStreamFn
	jetStreamContextFn = func(_ config.Config) (jetStreamClient, error) { return fake, nil }
	ensureStreamFn = ensureStream
	t.Cleanup(func() {
		jetStreamContextFn = origJet
		ensureStreamFn = origEnsure
	})

	cfg := config.Config{
		DispatchEnabled: true,
		NatsURL:         "nats://stubbed",
		NatsStream:      "SLACK_WORK",
	}
	outer := slackevents.EventsAPIEvent{
		Data: &slackevents.EventsAPICallbackEvent{EventID: "EvDedup"},
	}
	in := routing.Input{
		ChannelID: "C123",
		MessageTS: "177.2",
		UserID:    "UHuman",
		Text:      "hello",
	}
	d := routing.Decision{
		Trigger:      routing.TriggerMention,
		Employees:    []string{"ross", "ROSS", " ross ", "joanne", "joanne"},
		Kind:         routing.KindConversation,
		DispatchMode: routing.DispatchModeFanout,
	}

	results := Decision(context.Background(), cfg, outer, in, d, "message")
	if len(results) != 2 {
		t.Fatalf("expected 2 deduped dispatch results, got %d (%+v)", len(results), results)
	}
	if len(fake.published) != 2 {
		t.Fatalf("expected 2 deduped publishes, got %d", len(fake.published))
	}
	gotSubjects := map[string]bool{}
	for _, p := range fake.published {
		gotSubjects[p.subject] = true
	}
	if !gotSubjects["slack.work.ross.events"] || !gotSubjects["slack.work.joanne.events"] {
		t.Fatalf("unexpected deduped subjects: %+v", gotSubjects)
	}
}
