package routing

import (
	"slices"
	"testing"
)

func TestClassifyBroadcastTrigger(t *testing.T) {
	tests := []struct {
		in   string
		want BroadcastTrigger
	}{
		{"<!everyone> hi", BroadcastEveryone},
		{"hey @everyone there", BroadcastEveryone},
		{"<!here> ping", BroadcastEveryone},
		{"team @here FYI", BroadcastEveryone},
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
		ShuffleSecret: "test",
	}
	for _, text := range []string{"<!everyone> ship it", "<!here> ship it", "hey @here team"} {
		in := Input{ChannelID: "C1", MessageTS: "1.0", Text: text}
		d := Decide(cfg, in)
		if d.Trigger != TriggerEveryone {
			t.Fatalf("text=%q trigger=%s", text, d.Trigger)
		}
		if len(d.Employees) != 5 {
			t.Fatalf("text=%q employees=%v", text, d.Employees)
		}
		want := slices.Clone(cfg.Order)
		slices.Sort(want)
		got := slices.Clone(d.Employees)
		slices.Sort(got)
		if !slices.Equal(want, got) {
			t.Fatalf("text=%q employees must be a permutation of roster, got %v", text, d.Employees)
		}
		if d.Kind != KindConversation {
			t.Fatalf("text=%q kind=%s", text, d.Kind)
		}
		if d.DispatchMode != DispatchModeFanout {
			t.Fatalf("text=%q dispatch_mode=%s want fanout", text, d.DispatchMode)
		}
	}
}

func TestDecideChannelLimitsToThree(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		EveryoneLimit: 5,
		ChannelLimit:  3,
		ShuffleSecret: "test",
	}
	d := Decide(cfg, Input{MessageTS: "99.0", Text: "<!channel> hi"})
	if d.Trigger != TriggerChannel || len(d.Employees) != 3 {
		t.Fatalf("got %+v", d)
	}
	want := slices.Clone(cfg.Order[:3])
	slices.Sort(want)
	got := slices.Clone(d.Employees)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		t.Fatalf("channel employees must be a permutation of first 3 in roster, got %v", d.Employees)
	}
	if d.DispatchMode != DispatchModeFanout {
		t.Fatalf("dispatch_mode=%s", d.DispatchMode)
	}
}

func TestDecideBroadcastBeatsExplicitMention(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		BotUserToKey:  map[string]string{"UROSS": "ross"},
		EveryoneLimit: 5,
		ChannelLimit:  3,
		ShuffleSecret: "test",
	}
	d := Decide(cfg, Input{MessageTS: "1.0", Text: "<!everyone> <@UROSS> weigh in"})
	if d.Trigger != TriggerEveryone {
		t.Fatalf("broadcast must win over explicit mention, got %+v", d)
	}
	if d.DispatchMode != DispatchModeFanout || len(d.Employees) != 5 {
		t.Fatalf("everyone must fan out to all configured employees, got %+v", d)
	}
	want := slices.Clone(cfg.Order)
	slices.Sort(want)
	got := slices.Clone(d.Employees)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		t.Fatalf("everyone employees must be a permutation of roster, got %v", d.Employees)
	}
}

func TestDecideChannelBeatsExplicitMention(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		BotUserToKey:  map[string]string{"UROSS": "ross"},
		EveryoneLimit: 5,
		ChannelLimit:  3,
		ShuffleSecret: "test",
	}
	d := Decide(cfg, Input{MessageTS: "2.0", Text: "<!channel> <@UROSS> thoughts?"})
	if d.Trigger != TriggerChannel {
		t.Fatalf("channel broadcast must win over explicit mention, got %+v", d)
	}
	if d.DispatchMode != DispatchModeFanout || len(d.Employees) != 3 {
		t.Fatalf("channel must fan out to 3, got %+v", d)
	}
	want := slices.Clone(cfg.Order[:3])
	slices.Sort(want)
	got := slices.Clone(d.Employees)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		t.Fatalf("channel employees must be a permutation of first 3 in roster, got %v", d.Employees)
	}
}

func TestDecideMentionTool(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"garth", "alex"},
		BotUserToKey:  map[string]string{"UGARTH": "garth"},
		EveryoneLimit: 5,
		ChannelLimit:  3,
		ShuffleSecret: "x",
	}
	d := Decide(cfg, Input{Text: "<@UGARTH> search twitter for bitcoin"})
	if d.Trigger != TriggerMention || d.Employees[0] != "garth" {
		t.Fatalf("got %+v", d)
	}
	if d.Kind != KindTool || d.ToolID != "read-twitter" {
		t.Fatalf("kind/tool=%s %q", d.Kind, d.ToolID)
	}
	if d.DispatchMode != DispatchModeSingle || len(d.Employees) != 1 {
		t.Fatalf("single mention tool must stay single-target: %+v", d)
	}
}

func TestDecideMentionToolFanoutForMultipleMentions(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "joanne"},
		BotUserToKey:  map[string]string{"UROSS": "ross", "UJOANNE": "joanne"},
		EveryoneLimit: 5,
		ChannelLimit:  3,
		ShuffleSecret: "x",
	}
	d := Decide(cfg, Input{Text: "<@UJOANNE> <@UROSS> search twitter for AI creators"})
	if d.Trigger != TriggerMention || d.Kind != KindTool || d.ToolID != "read-twitter" {
		t.Fatalf("got %+v", d)
	}
	if d.DispatchMode != DispatchModeFanout {
		t.Fatalf("multi mention tool must fan out: %+v", d)
	}
	if len(d.Employees) != 2 || d.Employees[0] != "ross" || d.Employees[1] != "joanne" {
		t.Fatalf("mentions should be ordered by roster: %+v", d)
	}
}

func TestDecideMentionConversationFallback(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"tim"},
		BotUserToKey:  map[string]string{"UTIM": "tim"},
		EveryoneLimit: 5,
		ChannelLimit:  3,
		ShuffleSecret: "x",
	}
	d := Decide(cfg, Input{Text: "<@UTIM> thanks for the help"})
	if d.Kind != KindConversation || d.ToolID != "" {
		t.Fatalf("got %+v", d)
	}
	if d.DispatchMode != DispatchModeSingle || len(d.Employees) != 1 || d.Employees[0] != "tim" {
		t.Fatalf("single mention must stay single-target: %+v", d)
	}
}

func TestDecideMentionConversationFanoutForMultipleMentions(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "joanne"},
		BotUserToKey:  map[string]string{"UROSS": "ross", "UJOANNE": "joanne"},
		EveryoneLimit: 5,
		ChannelLimit:  3,
		ShuffleSecret: "x",
	}
	d := Decide(cfg, Input{Text: "hey <@UJOANNE> and <@UROSS> can you both check?"})
	if d.Trigger != TriggerMention || d.Kind != KindConversation {
		t.Fatalf("got %+v", d)
	}
	if d.DispatchMode != DispatchModeFanout {
		t.Fatalf("multi mention must fan out: %+v", d)
	}
	if len(d.Employees) != 2 || d.Employees[0] != "ross" || d.Employees[1] != "joanne" {
		t.Fatalf("mentions should be ordered by roster: %+v", d)
	}
}

func TestDecideMentionInThreadOverridesRootMentionStickiness(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "joanne"},
		BotUserToKey:  map[string]string{"UROSS": "ross", "UJOANNE": "joanne"},
		EveryoneLimit: 5,
		ChannelLimit:  3,
		ShuffleSecret: "x",
	}
	in := Input{
		ThreadTS:  "177.1",
		MessageTS: "177.2",
		Text:      "<@UJOANNE> can you take this one?",
	}
	d := Decide(cfg, in)
	if d.Trigger != TriggerMention || len(d.Employees) != 1 || d.Employees[0] != "joanne" {
		t.Fatalf("thread mention should override root stickiness: %+v", d)
	}
}

func TestDecideBroadcastRootThreadFollowupUsesRandomPicker(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		BotUserToKey:  map[string]string{"UROSSBOT": "ross"},
		EveryoneLimit: 5,
		ChannelLimit:  3,
		ShuffleSecret: "secret",
	}
	in := Input{
		ThreadTS:  "177.1",
		MessageTS: "177.2",
		Text:      "plain follow-up",
	}
	d := Decide(cfg, in)
	want := pickPlainResponder(in.MessageTS, cfg.Order, cfg.ShuffleSecret)
	if d.Trigger != TriggerPlain || d.DispatchMode != DispatchModeSingle || len(d.Employees) != 1 {
		t.Fatalf("broadcast follow-up must be single random: %+v", d)
	}
	if d.Employees[0] != want {
		t.Fatalf("broadcast follow-up should use random picker, got=%q want=%q", d.Employees[0], want)
	}
}

func TestDecideBroadcastRootThreadMentionStillBroadcasts(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		BotUserToKey:  map[string]string{"UROSSBOT": "ross"},
		EveryoneLimit: 5,
		ChannelLimit:  3,
		ShuffleSecret: "secret",
	}
	in := Input{
		ThreadTS:  "177.1",
		MessageTS: "177.2",
		Text:      "<!channel> <@UROSSBOT> new ask",
	}
	d := Decide(cfg, in)
	if d.Trigger != TriggerChannel || d.DispatchMode != DispatchModeFanout || len(d.Employees) != 3 {
		t.Fatalf("broadcast in thread must still win precedence, got %+v", d)
	}
	want := slices.Clone(cfg.Order[:3])
	slices.Sort(want)
	got := slices.Clone(d.Employees)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		t.Fatalf("channel employees must be a permutation of first 3 in roster, got %v", d.Employees)
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
	if len(d1.Employees) != 1 || d1.DispatchMode != DispatchModeSingle {
		t.Fatalf("plain must be single-target: %+v", d1)
	}
	if d1.PrimaryEmployee != d1.Employees[0] {
		t.Fatalf("primary=%q emp=%q", d1.PrimaryEmployee, d1.Employees[0])
	}
}

func TestDecidePlainThreadSingleTargetNotFanout(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		BotUserToKey:  nil,
		EveryoneLimit: 5,
		ChannelLimit:  3,
		ShuffleSecret: "secret",
	}
	in := Input{ChannelID: "C", ThreadTS: "177.1", MessageTS: "177.2", Text: "follow up in thread"}
	d := Decide(cfg, in)
	if len(d.Employees) != 1 {
		t.Fatalf("thread plain must fan out to 0 extra pods; got %d employees: %v", len(d.Employees), d.Employees)
	}
	if d.DispatchMode != DispatchModeSingle {
		t.Fatalf("dispatch_mode=%s", d.DispatchMode)
	}
}

func TestDecidePlainThreadFollowsRootMention(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		BotUserToKey:  map[string]string{"UROSSBOT": "ross", "UJOANNE": "joanne"},
		EveryoneLimit: 5,
		ChannelLimit:  3,
		ShuffleSecret: "secret",
	}
	in := Input{
		ChannelID:             "C",
		ThreadTS:              "1776448093.830689",
		MessageTS:             "1776448142.137509",
		Text:                  "Amazing, good to hear",
		ThreadPlainHandoffKey: "ross",
	}
	d := Decide(cfg, in)
	if d.Trigger != TriggerPlain {
		t.Fatalf("trigger=%s want plain", d.Trigger)
	}
	if len(d.Employees) != 1 || d.Employees[0] != "ross" {
		t.Fatalf("want ross from thread root mention; got %+v", d)
	}
}

func TestClassifyToolOrConversationAmbiguous(t *testing.T) {
	tool, k := ClassifyToolOrConversation("we have email and twitter tooling")
	if k != KindConversation || tool != "" {
		t.Fatalf("tool=%q k=%s", tool, k)
	}
}

func TestLastSquadHandoffKey_LastMentionWinsOverRoot(t *testing.T) {
	cfg := DecideConfig{
		BotUserToKey: map[string]string{"UROSS": "ross", "UJOANNE": "joanne"},
	}
	msgs := []ThreadMessage{
		{Timestamp: "1.0", Text: "<@UROSS> kickoff"},
		{Timestamp: "2.0", Text: "<@UJOANNE> are you around?"},
	}
	if got := LastSquadHandoffKey(msgs, "1.0", cfg); got != "joanne" {
		t.Fatalf("got %q want joanne", got)
	}
}

func TestLastSquadHandoffKey_SkipsMentionsInBroadcastRoot(t *testing.T) {
	cfg := DecideConfig{
		BotUserToKey: map[string]string{"UROSSBOT": "ross"},
	}
	msgs := []ThreadMessage{
		{Timestamp: "1.0", Text: "<!everyone> <@UROSSBOT> kickoff"},
	}
	if got := LastSquadHandoffKey(msgs, "1.0", cfg); got != "" {
		t.Fatalf("broadcast root must not pin ross; got %q", got)
	}
}

func TestLastSquadHandoffKey_BroadcastRootThenLaterMention(t *testing.T) {
	cfg := DecideConfig{
		BotUserToKey: map[string]string{"UROSSBOT": "ross", "UJOANNE": "joanne"},
	}
	msgs := []ThreadMessage{
		{Timestamp: "1.0", Text: "<!everyone> <@UROSSBOT> kickoff"},
		{Timestamp: "2.0", Text: "<@UJOANNE> ping"},
	}
	if got := LastSquadHandoffKey(msgs, "1.0", cfg); got != "joanne" {
		t.Fatalf("got %q want joanne", got)
	}
}

func TestDecidePlainThreadHandoffFromLastMention(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		BotUserToKey:  map[string]string{"UROSS": "ross", "UJOANNE": "joanne"},
		EveryoneLimit: 5,
		ChannelLimit:  3,
		ShuffleSecret: "secret",
	}
	in := Input{
		ChannelID:             "C",
		ThreadTS:              "177.1",
		MessageTS:             "177.9",
		Text:                  "plain after joanne was addressed",
		ThreadPlainHandoffKey: "joanne",
	}
	d := Decide(cfg, in)
	if d.Trigger != TriggerPlain || len(d.Employees) != 1 || d.Employees[0] != "joanne" {
		t.Fatalf("want joanne handoff; got %+v", d)
	}
}
