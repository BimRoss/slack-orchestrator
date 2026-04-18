package routing

import (
	"testing"
)

func TestTryPipelineDecisionTwoSteps(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"alex", "tim", "ross", "joanne"},
		BotUserToKey:  map[string]string{"UJOANNE": "joanne", "UROSS": "ross"},
		ShuffleSecret: "x",
	}
	in := Input{ChannelID: "C1", MessageTS: "10.0", Text: "<@UJOANNE> read twitter for btc <@UROSS> read trends now"}
	d, ok := TryPipelineDecision(cfg, in)
	if !ok {
		t.Fatal("expected pipeline")
	}
	if d.ExecutionMode != ExecutionModePipeline || len(d.PipelineSteps) != 2 {
		t.Fatalf("got %+v", d)
	}
	if d.PipelineSteps[0].TargetEmployee != "joanne" || d.PipelineSteps[1].TargetEmployee != "ross" {
		t.Fatalf("steps=%+v", d.PipelineSteps)
	}
	if d.PipelineSteps[0].Kind != KindTool || d.PipelineSteps[0].ToolID != "read-twitter" {
		t.Fatalf("step0 kind/tool=%s %q", d.PipelineSteps[0].Kind, d.PipelineSteps[0].ToolID)
	}
	if d.PipelineSteps[1].Kind != KindTool || d.PipelineSteps[1].ToolID != "read-trends" {
		t.Fatalf("step1 kind/tool=%s %q", d.PipelineSteps[1].Kind, d.PipelineSteps[1].ToolID)
	}
	if d.DispatchMode != DispatchModeSingle || len(d.Employees) != 1 || d.Employees[0] != "joanne" {
		t.Fatalf("dispatch %+v", d)
	}
	if d.Kind != KindTool || d.ToolID != "read-twitter" {
		t.Fatalf("top-level kind should mirror step0: %+v", d)
	}
}

func TestDecidePipelineBeatsWholeMessageTool(t *testing.T) {
	cfg := DecideConfig{
		Order:         []string{"joanne", "ross"},
		BotUserToKey:  map[string]string{"UJOANNE": "joanne", "UROSS": "ross"},
		ShuffleSecret: "x",
	}
	in := Input{ChannelID: "C1", MessageTS: "11.0", Text: "<@UJOANNE> thanks <@UROSS> read-twitter"}
	d := Decide(cfg, in)
	if d.ExecutionMode != ExecutionModePipeline {
		t.Fatalf("want pipeline, got %+v", d)
	}
	if len(d.PipelineSteps) != 2 {
		t.Fatalf("steps=%d", len(d.PipelineSteps))
	}
	if d.PipelineSteps[0].Kind != KindConversation || d.PipelineSteps[1].Kind != KindTool || d.PipelineSteps[1].ToolID != "read-twitter" {
		t.Fatalf("steps=%+v", d.PipelineSteps)
	}
}
