package routing

import (
	"testing"
)

func TestClassifyBroadcastTrigger(t *testing.T) {
	tests := []struct {
		in   string
		want BroadcastTrigger
	}{
		{"<!everyone> hi", BroadcastEveryone},
		{"hey @everyone there", BroadcastEveryone},
		{"<!channel> x", BroadcastChannel},
		{"note @channel please", BroadcastChannel},
		{"Hey everyone", BroadcastNone},
		{"plain", BroadcastNone},
	}
	for _, tc := range tests {
		got := ClassifyBroadcastTrigger(tc.in)
		if got != tc.want {
			t.Fatalf("ClassifyBroadcastTrigger(%q)=%v want %v", tc.in, got, tc.want)
		}
	}
}

func TestDecideEveryone(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		EveryoneLimit: 5,
		ChannelLimit:  3,
		ShuffleSecret:   "test",
	}
	in := Input{ChannelID: "C1", MessageTS: "1.0", Text: "<!everyone> ship it"}
	d := Decide(cfg, in)
	if d.Trigger != TriggerEveryone {
		t.Fatalf("trigger=%s", d.Trigger)
	}
	if len(d.Employees) != 5 {
		t.Fatalf("employees=%v", d.Employees)
	}
	if d.Kind != KindConversation {
		t.Fatalf("kind=%s", d.Kind)
	}
}

func TestDecideChannelLimitsToThree(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		EveryoneLimit: 5,
		ChannelLimit:  3,
		ShuffleSecret:   "test",
	}
	d := Decide(cfg, Input{Text: "<!channel> hi"})
	if d.Trigger != TriggerChannel || len(d.Employees) != 3 {
		t.Fatalf("got %+v", d)
	}
}

func TestDecideMentionTool(t *testing.T) {
	cfg := DecideConfig{
		Order:        []string{"garth", "alex"},
		BotUserToKey: map[string]string{"UGARTH": "garth"},
		EveryoneLimit: 5,
		ChannelLimit:  3,
		ShuffleSecret:   "x",
	}
	d := Decide(cfg, Input{Text: "<@UGARTH> search twitter for bitcoin"})
	if d.Trigger != TriggerMention || d.Employees[0] != "garth" {
		t.Fatalf("got %+v", d)
	}
	if d.Kind != KindTool || d.ToolID != "read-twitter" {
		t.Fatalf("kind/tool=%s %q", d.Kind, d.ToolID)
	}
}

func TestDecideMentionConversationFallback(t *testing.T) {
	cfg := DecideConfig{
		Order:        []string{"tim"},
		BotUserToKey: map[string]string{"UTIM": "tim"},
		EveryoneLimit: 5,
		ChannelLimit:  3,
		ShuffleSecret:   "x",
	}
	d := Decide(cfg, Input{Text: "<@UTIM> thanks for the help"})
	if d.Kind != KindConversation || d.ToolID != "" {
		t.Fatalf("got %+v", d)
	}
}

func TestDecidePlainDeterministic(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		BotUserToKey:  nil,
		EveryoneLimit: 5,
		ChannelLimit:  3,
		ShuffleSecret: "secret",
	}
	in := Input{ChannelID: "C", ThreadTS: "T1", MessageTS: "M1", Text: "hello room"}
	d1 := Decide(cfg, in)
	d2 := Decide(cfg, in)
	if d1.Employees[0] != d2.Employees[0] || d1.Trigger != TriggerPlain {
		t.Fatalf("d1=%+v d2=%+v", d1, d2)
	}
}

func TestClassifyToolOrConversationAmbiguous(t *testing.T) {
	tool, k := ClassifyToolOrConversation("we have email and twitter tooling")
	if k != KindConversation || tool != "" {
		t.Fatalf("tool=%q k=%s", tool, k)
	}
}
