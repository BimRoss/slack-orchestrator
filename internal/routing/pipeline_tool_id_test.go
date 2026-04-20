package routing

import (
	"strings"
	"testing"
)

// Pipeline steps derive ToolID from [ClassifyToolOrConversation], which only emits Tier-1 canonical IDs.
func TestPipelineSyntheticMessageToolIDsAreTier1OrEmpty(t *testing.T) {
	t.Parallel()
	cfg := DecideConfig{
		BotUserToKey: map[string]string{
			"UJOANNE": "joanne",
			"UGARTH":  "garth",
		},
		Order: []string{"joanne", "garth"},
	}
	// Two mentions; first segment asks for read-company style phrasing, second names read-twitter explicitly.
	text := "<@UJOANNE> read company <@UGARTH> read-twitter"
	in := Input{ChannelID: "C1", MessageTS: "1.1", UserID: "UOP", Text: text}
	d, ok := TryPipelineDecision(cfg, in)
	if !ok {
		t.Fatal("expected pipeline decision")
	}
	if len(d.PipelineSteps) != 2 {
		t.Fatalf("steps: got %d", len(d.PipelineSteps))
	}
	allowed := map[string]struct{}{}
	for _, id := range Tier1CanonicalSkillIDs() {
		allowed[id] = struct{}{}
	}
	for i, st := range d.PipelineSteps {
		if st.Kind != KindTool {
			continue
		}
		tid := strings.TrimSpace(st.ToolID)
		if tid == "" {
			continue
		}
		if _, ok := allowed[tid]; !ok {
			t.Fatalf("step %d: tool_id %q not a tier1 canonical id", i, tid)
		}
	}
}
