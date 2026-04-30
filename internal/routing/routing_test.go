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
		EveryoneLimit: 3,
		ChannelLimit:  3,
		ShuffleSecret: "test",
	}
	for _, text := range []string{"<!everyone> ship it", "<!here> ship it", "hey @here team"} {
		in := Input{ChannelID: "C1", MessageTS: "1.0", Text: text}
		d := Decide(cfg, in)
		if d.Trigger != TriggerEveryone {
			t.Fatalf("text=%q trigger=%s", text, d.Trigger)
		}
		if len(d.Employees) != 3 {
			t.Fatalf("text=%q employees=%v", text, d.Employees)
		}
		want := slices.Clone(cfg.Order[:3])
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
		EveryoneLimit: 3,
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

func TestDecideBroadcastChannelExcludesSquadPoster(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		BotUserToKey:  map[string]string{"UJOANNE": "joanne"},
		EveryoneLimit: 3,
		ChannelLimit:  3,
		ShuffleSecret: "test",
	}
	d := Decide(cfg, Input{
		MessageTS: "99.0",
		UserID:    "UJOANNE",
		Text:      "<!channel> team intro",
	})
	if d.Trigger != TriggerChannel || d.DispatchMode != DispatchModeFanout || len(d.Employees) != 3 {
		t.Fatalf("got %+v", d)
	}
	for _, e := range d.Employees {
		if e == "joanne" {
			t.Fatalf("poster must not receive own broadcast: %v", d.Employees)
		}
	}
	want := []string{"alex", "ross", "tim"}
	slices.Sort(want)
	got := slices.Clone(d.Employees)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		t.Fatalf("want first three slots from roster excluding joanne, got %v", d.Employees)
	}
}

func TestDecideBroadcastEveryoneExcludesSquadPoster(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		BotUserToKey:  map[string]string{"UJOANNE": "joanne"},
		EveryoneLimit: 3,
		ChannelLimit:  3,
		ShuffleSecret: "test",
	}
	d := Decide(cfg, Input{
		MessageTS: "1.0",
		UserID:    "UJOANNE",
		Text:      "<!everyone> all hands",
	})
	if d.Trigger != TriggerEveryone || d.DispatchMode != DispatchModeFanout || len(d.Employees) != 3 {
		t.Fatalf("got %+v", d)
	}
	for _, e := range d.Employees {
		if e == "joanne" {
			t.Fatalf("poster must not receive own broadcast: %v", d.Employees)
		}
	}
	want := []string{"alex", "ross", "tim"}
	slices.Sort(want)
	got := slices.Clone(d.Employees)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		t.Fatalf("want all roster except joanne, got %v", d.Employees)
	}
}

func TestDecideBroadcastBeatsExplicitMention(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		BotUserToKey:  map[string]string{"UROSS": "ross"},
		EveryoneLimit: 3,
		ChannelLimit:  3,
		ShuffleSecret: "test",
	}
	d := Decide(cfg, Input{MessageTS: "1.0", Text: "<!everyone> <@UROSS> weigh in"})
	if d.Trigger != TriggerEveryone {
		t.Fatalf("broadcast must win over explicit mention, got %+v", d)
	}
	if d.DispatchMode != DispatchModeFanout || len(d.Employees) != 3 {
		t.Fatalf("everyone must fan out to 3 participants, got %+v", d)
	}
	want := slices.Clone(cfg.Order[:3])
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
		EveryoneLimit: 3,
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
		EveryoneLimit: 3,
		ChannelLimit:  3,
		ShuffleSecret: "x",
	}
	d := Decide(cfg, Input{Text: "<@UGARTH> read-twitter"})
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

func TestDecideMentionPipelineSameBotTwice(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"joanne"},
		BotUserToKey:  map[string]string{"UJOANNE": "joanne"},
		EveryoneLimit: 3,
		ChannelLimit:  3,
		ShuffleSecret: "x",
	}
	d := Decide(cfg, Input{ChannelID: "C", MessageTS: "13.0", Text: "<@UJOANNE> read twitter <@UJOANNE> read trends"})
	if d.Trigger != TriggerMention || d.ExecutionMode != ExecutionModePipeline {
		t.Fatalf("got %+v", d)
	}
	if d.DispatchMode != DispatchModeSingle || len(d.Employees) != 1 || d.Employees[0] != "joanne" {
		t.Fatalf("pipeline first hop: %+v", d)
	}
	if len(d.PipelineSteps) != 2 ||
		d.PipelineSteps[0].TargetEmployee != "joanne" ||
		d.PipelineSteps[1].TargetEmployee != "joanne" {
		t.Fatalf("pipeline_steps=%+v", d.PipelineSteps)
	}
}

func TestDecideMentionToolPipelineForMultipleMentions(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "joanne"},
		BotUserToKey:  map[string]string{"UROSS": "ross", "UJOANNE": "joanne"},
		EveryoneLimit: 3,
		ChannelLimit:  3,
		ShuffleSecret: "x",
	}
	d := Decide(cfg, Input{ChannelID: "C", MessageTS: "9.0", Text: "<@UJOANNE> read twitter <@UROSS> read trends"})
	if d.Trigger != TriggerMention || d.ExecutionMode != ExecutionModePipeline {
		t.Fatalf("got %+v", d)
	}
	if d.DispatchMode != DispatchModeSingle || len(d.Employees) != 1 || d.Employees[0] != "joanne" {
		t.Fatalf("pipeline must single-target first step: %+v", d)
	}
	if len(d.PipelineSteps) != 2 || d.PipelineSteps[0].ToolID != "read-twitter" || d.PipelineSteps[1].ToolID != "read-trends" {
		t.Fatalf("pipeline_steps=%+v", d.PipelineSteps)
	}
}

func TestDecideSquadBotParticipantListDoesNotPipelineToMentionedBots(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "garth", "joanne"},
		BotUserToKey:  map[string]string{"UJOANNE": "joanne", "UALEX": "alex", "UGARTH": "garth"},
		EveryoneLimit: 3,
		ChannelLimit:  3,
		ShuffleSecret: "x",
	}
	text := "Create this company workspace?\n\nParticipants: <@UALEX>, <@UGARTH>"
	d := Decide(cfg, Input{UserID: "UJOANNE", MessageTS: "1.0", Text: text})
	if d.PrimaryEmployee != "joanne" || len(d.Employees) != 1 || d.Employees[0] != "joanne" {
		t.Fatalf("want poster only, got %+v", d)
	}
	if d.ExecutionMode == ExecutionModePipeline {
		t.Fatalf("participant roster must not become pipeline: %+v", d)
	}
	if d.Kind != KindConversation || d.Trigger != TriggerPlain {
		t.Fatalf("got %+v", d)
	}
}

func TestDecideMentionConversationFallback(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"tim"},
		BotUserToKey:  map[string]string{"UTIM": "tim"},
		EveryoneLimit: 3,
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

func TestDecideMentionConversationPipelineForMultipleMentions(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "joanne"},
		BotUserToKey:  map[string]string{"UROSS": "ross", "UJOANNE": "joanne"},
		EveryoneLimit: 3,
		ChannelLimit:  3,
		ShuffleSecret: "x",
	}
	d := Decide(cfg, Input{ChannelID: "C", MessageTS: "8.0", Text: "hey <@UJOANNE> and <@UROSS> can you both check?"})
	if d.Trigger != TriggerMention || d.ExecutionMode != ExecutionModePipeline {
		t.Fatalf("got %+v", d)
	}
	if d.DispatchMode != DispatchModeSingle || len(d.Employees) != 1 {
		t.Fatalf("pipeline first hop: %+v", d)
	}
	if len(d.PipelineSteps) != 2 || d.PipelineSteps[0].Kind != KindConversation {
		t.Fatalf("steps=%+v", d.PipelineSteps)
	}
}

func TestDecideMentionInThreadOverridesRootMentionStickiness(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "joanne"},
		BotUserToKey:  map[string]string{"UROSS": "ross", "UJOANNE": "joanne"},
		EveryoneLimit: 3,
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
		EveryoneLimit: 3,
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
		EveryoneLimit: 3,
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
		EveryoneLimit: 3,
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
		EveryoneLimit: 3,
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
		EveryoneLimit: 3,
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
		EveryoneLimit: 3,
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

func TestDecide_CompanyOnboardingShapedPlainUsesPlainResponder(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		BotUserToKey:  map[string]string{"UALEX": "alex", "UJOANNE": "joanne"},
		EveryoneLimit: 3,
		ChannelLimit:  3,
		ShuffleSecret: "secret",
	}
	in := Input{
		ChannelID: "CNEWCO",
		ThreadTS:  "",
		MessageTS: "99.1",
		Text:      "2",
	}
	d := Decide(cfg, in)
	want := pickPlainResponder(in.MessageTS, cfg.Order, cfg.ShuffleSecret)
	if d.Trigger != TriggerPlain || len(d.Employees) != 1 || d.Employees[0] != want {
		t.Fatalf("onboarding-shaped text should use plain responder; got %+v want employee=%q", d, want)
	}
}

func TestDecide_OnboardingReplyDoesNotOverrideThreadHandoff(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		BotUserToKey:  map[string]string{"UROSS": "ross", "UJOANNE": "joanne"},
		EveryoneLimit: 3,
		ChannelLimit:  3,
		ShuffleSecret: "secret",
	}
	in := Input{
		ChannelID:             "C",
		ThreadTS:              "100.0",
		MessageTS:             "100.2",
		Text:                  "1",
		ThreadPlainHandoffKey: "ross",
	}
	d := Decide(cfg, in)
	if len(d.Employees) != 1 || d.Employees[0] != "ross" {
		t.Fatalf("want ross handoff preserved over onboarding match; got %+v", d)
	}
}

func TestDecide_CompanyOnboardingShapedThreadNoHandoffUsesPlainResponder(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		BotUserToKey:  map[string]string{"UALEX": "alex"},
		EveryoneLimit: 3,
		ChannelLimit:  3,
		ShuffleSecret: "secret",
	}
	in := Input{
		ChannelID:             "CNEWCO",
		ThreadTS:              "100.0",
		MessageTS:             "100.1",
		Text:                  "1",
		ThreadPlainHandoffKey: "",
	}
	d := Decide(cfg, in)
	want := pickPlainResponder(in.MessageTS, cfg.Order, cfg.ShuffleSecret)
	if d.Trigger != TriggerPlain || len(d.Employees) != 1 || d.Employees[0] != want {
		t.Fatalf("onboarding-shaped thread reply without handoff should use plain responder; got %+v want employee=%q", d, want)
	}
}

func TestSquadBotMentionsOtherSquadMember(t *testing.T) {
	cfg := DecideConfig{
		Order:        []string{"tim", "joanne"},
		BotUserToKey: map[string]string{"UTIM": "tim", "UJOANNE": "joanne"},
	}
	if !SquadBotMentionsOtherSquadMember(cfg, "UTIM", "<@UJOANNE> what has happened lately at the company") {
		t.Fatal("expected tim→joanne delegation")
	}
	if SquadBotMentionsOtherSquadMember(cfg, "UTIM", "<@UTIM> ping myself") {
		t.Fatal("did not expect mention-only-self")
	}
	if SquadBotMentionsOtherSquadMember(cfg, "UXUNKNOWN", "<@UJOANNE> hi") {
		t.Fatal("did not expect non-squad poster")
	}
	if SquadBotMentionsOtherSquadMember(cfg, "UJOANNE", "no mentions") {
		t.Fatal("did not expect text without squad mentions")
	}
}

func TestHasOnlyNonSquadMentions(t *testing.T) {
	botMap := map[string]string{"UJOANNE": "joanne", "UTIM": "tim"}

	if !HasOnlyNonSquadMentions("<@UGRANT> are you around?", botMap) {
		t.Fatal("expected true for non-squad mention only")
	}
	if HasOnlyNonSquadMentions("<@UJOANNE> can you check this?", botMap) {
		t.Fatal("did not expect true when a squad mention is present")
	}
	if HasOnlyNonSquadMentions("<@UGRANT> and <@UTIM> can you both look?", botMap) {
		t.Fatal("did not expect true when mixed squad + non-squad mentions are present")
	}
	if HasOnlyNonSquadMentions("plain text, no mentions", botMap) {
		t.Fatal("did not expect true for text without mentions")
	}
}

func TestIsCreateCompanySlackPostConfirmationText(t *testing.T) {
	epilogue := "Created: <#C0ABC|acme>\nInvited: <@UH1>, <@UALEX>"
	if !IsCreateCompanySlackPostConfirmationText(epilogue) {
		t.Fatal("expected create-company epilogue shape")
	}
	if IsCreateCompanySlackPostConfirmationText("Created: only first line") {
		t.Fatal("did not expect without Invited line")
	}
	if IsCreateCompanySlackPostConfirmationText("note\nInvited: x") {
		t.Fatal("did not expect without Created: prefix")
	}
}

func TestLastSquadHandoffKey_SkipsCreateCompanyConfirmationRoot(t *testing.T) {
	cfg := DecideConfig{
		BotUserToKey: map[string]string{"UALEX": "alex", "UJOANNE": "joanne"},
	}
	msgs := []ThreadMessage{
		{Timestamp: "1.0", Text: "Created: <#C1|x>\nInvited: <@UALEX>, <@UJOANNE>"},
	}
	if got := LastSquadHandoffKey(msgs, "1.0", cfg); got != "" {
		t.Fatalf("create-company confirmation root must not pin a squad handoff; got %q", got)
	}
}

func TestDecide_PlainThreadUnderCreateCompanyConfirmRoot_NoEmployees(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		BotUserToKey:  map[string]string{"UALEX": "alex"},
		EveryoneLimit: 3,
		ChannelLimit:  3,
		ShuffleSecret: "secret",
	}
	in := Input{
		ChannelID:      "CHUMANS",
		ThreadTS:       "100.0",
		MessageTS:      "100.1",
		Text:           "thanks!",
		ThreadRootText: "Created: <#CNEW|co>\nInvited: <@UH>, <@UALEX>",
	}
	d := Decide(cfg, in)
	if len(d.Employees) != 0 {
		t.Fatalf("want no squad dispatch under create-confirm thread; got %+v", d)
	}
}

func TestDecide_CreateCompanyEpilogue_FromSquadBot_NoDispatchEvenWithOneSquadMention(t *testing.T) {
	// Regresses: Invited: lists humans + orchestrator (no BotUserToKey) + Alex → |mentioned|==["alex"] only;
	// Alex must not get a spurious "mention" route for Joanne's status post.
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		BotUserToKey:  map[string]string{"UJOANNE": "joanne", "UALEX": "alex"},
		ShuffleSecret: "secret",
	}
	epilogue := "Created: <#CNEW|co>\nInvited: <@UHUMAN>, <@UORCH>, <@UALEX>"
	in := Input{
		ChannelID: "CHUMANS",
		ThreadTS:  "100.0",
		MessageTS: "100.1",
		UserID:    "UJOANNE",
		Text:      epilogue,
	}
	d := Decide(cfg, in)
	if len(d.Employees) != 0 {
		t.Fatalf("want no dispatch for create-company epilogue from squad bot; got %+v", d)
	}
}

func TestDecide_ExplicitMentionInThreadUnderCreateCompanyConfirmRoot_StillRoutes(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "garth", "joanne"},
		BotUserToKey:  map[string]string{"UALEX": "alex"},
		EveryoneLimit: 3,
		ChannelLimit:  3,
		ShuffleSecret: "secret",
	}
	in := Input{
		ChannelID:      "CHUMANS",
		ThreadTS:       "100.0",
		MessageTS:      "100.1",
		Text:           "<@UALEX> can you help?",
		ThreadRootText: "Created: <#CNEW|co>\nInvited: <@UH>, <@UALEX>",
	}
	d := Decide(cfg, in)
	if len(d.Employees) != 1 || d.Employees[0] != "alex" || d.Trigger != TriggerMention {
		t.Fatalf("want explicit mention routing; got %+v", d)
	}
}
