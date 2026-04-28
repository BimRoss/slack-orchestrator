package routing

import "testing"

func TestUpdateTermsIntentText(t *testing.T) {
	t.Parallel()
	yes := []string{
		"@joanne update-terms",
		"update terms",
		"update_terms",
		"show me the terms and conditions",
		"<@U09> show the terms",
		"read the terms of use",
		"I'd like to agree to the terms",
		"view the terms",
	}
	no := []string{
		"",
		"hello",
		"read company digest",
		"what are the payment terms of this deal",
	}
	for _, s := range yes {
		s := s
		t.Run("yes:"+truncate(s, 40), func(t *testing.T) {
			if !UpdateTermsIntentText(s) {
				t.Fatalf("expected true for %q", s)
			}
		})
	}
	for _, s := range no {
		if UpdateTermsIntentText(s) {
			t.Fatalf("expected false for %q", s)
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func TestDecidePreAcceptanceTermsBypassForcesJoanne(t *testing.T) {
	t.Parallel()
	cfg := DecideConfig{
		Order:         []string{"garth", "ross", "joanne"},
		BotUserToKey:  map[string]string{"UBOTJ": "joanne"},
		EveryoneLimit: 0,
		ChannelLimit:  0,
		ShuffleSecret: "s",
	}
	d := Decide(cfg, Input{
		ChannelID:                "C1",
		MessageTS:                "1.0",
		UserID:                   "U1",
		Text:                     "ignored; bypass uses fixed decision",
		PreAcceptanceTermsBypass: true,
	})
	if d.PrimaryEmployee != "joanne" {
		t.Fatalf("primary: got %q want joanne", d.PrimaryEmployee)
	}
	if len(d.Employees) != 1 || d.Employees[0] != "joanne" {
		t.Fatalf("employees: %+v", d.Employees)
	}
	if d.ToolID != "update-terms" || d.Kind != KindTool {
		t.Fatalf("decision: %+v", d)
	}
}
