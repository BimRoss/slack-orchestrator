package routing

import (
	"fmt"
	"strings"
)

// ExecutionModePipeline is routing.Decision.ExecutionMode when ordered pipeline_steps apply.
const ExecutionModePipeline = "pipeline"

// PipelineStep is one ordered step in a pipeline chain (one human message).
// Steps may involve different employees or the same employee mentioned multiple times.
type PipelineStep struct {
	TargetEmployee string `json:"target_employee"`
	StepText       string `json:"step_text"`
	Kind           Kind   `json:"kind"`
	ToolID         string `json:"tool_id,omitempty"`
	// ClassificationReason explains Tier-1 classification for this step text.
	ClassificationReason string `json:"classification_reason,omitempty"`
}

// squadMentionOccurrences returns every squad bot mention in left-to-right order (byte indices).
// The same employee may appear more than once so callers can build multi-step pipelines
// (e.g. @joanne read company @joanne draft email).
func squadMentionOccurrences(text string, botUserToKey map[string]string) []mentionOccurrence {
	if len(botUserToKey) == 0 {
		return nil
	}
	all := reSlackUserMention.FindAllStringSubmatchIndex(text, -1)
	var out []mentionOccurrence
	for _, m := range all {
		if len(m) < 4 {
			continue
		}
		uid := text[m[2]:m[3]]
		key, ok := botUserToKey[uid]
		if !ok {
			continue
		}
		out = append(out, mentionOccurrence{
			key:       key,
			end:       m[1],
			fullStart: m[0],
		})
	}
	return out
}

type mentionOccurrence struct {
	key       string
	end       int
	fullStart int
}

// segmentTextsForSquadMentions splits text into one segment per squad mention (appearance order).
// Text before the first mention is prepended to the first segment.
func segmentTextsForSquadMentions(text string, occ []mentionOccurrence) []string {
	if len(occ) == 0 {
		return nil
	}
	var segs []string
	prefix := strings.TrimSpace(text[:occ[0].fullStart])
	for i := range occ {
		bodyStart := occ[i].end
		var bodyEnd int
		if i+1 < len(occ) {
			bodyEnd = occ[i+1].fullStart
		} else {
			bodyEnd = len(text)
		}
		seg := strings.TrimSpace(text[bodyStart:bodyEnd])
		if i == 0 && prefix != "" {
			seg = strings.TrimSpace(prefix + " " + seg)
		}
		segs = append(segs, seg)
	}
	return segs
}

// TryPipelineDecision returns a pipeline decision when there are 2+ squad mentions in the message.
// Mentions may repeat the same bot; ok is false when fewer than two squad mentions appear.
func TryPipelineDecision(cfg DecideConfig, in Input) (Decision, bool) {
	occ := squadMentionOccurrences(in.Text, cfg.BotUserToKey)
	if len(occ) < 2 {
		return Decision{}, false
	}
	keys := make([]string, len(occ))
	for i := range occ {
		keys[i] = occ[i].key
	}
	segs := segmentTextsForSquadMentions(in.Text, occ)
	if len(segs) != len(keys) {
		return Decision{}, false
	}
	var steps []PipelineStep
	for i := range keys {
		st := strings.TrimSpace(segs[i])
		if st == "" {
			st = strings.TrimSpace(in.Text)
		}
		toolID, k, reason := ClassifyToolOrConversationWithReason(st)
		steps = append(steps, PipelineStep{
			TargetEmployee:       strings.ToLower(strings.TrimSpace(keys[i])),
			StepText:             st,
			Kind:                 k,
			ToolID:               toolID,
			ClassificationReason: reason,
		})
	}
	chainID := fmt.Sprintf("%s:%s", strings.TrimSpace(in.ChannelID), strings.TrimSpace(in.MessageTS))
	first := steps[0]
	d := Decision{
		Trigger:              TriggerMention,
		ExecutionMode:        ExecutionModePipeline,
		PipelineSteps:        steps,
		PipelineStepIndex:    0,
		ChainID:              chainID,
		Employees:            []string{first.TargetEmployee},
		Kind:                 first.Kind,
		ToolID:               first.ToolID,
		ClassificationReason: first.ClassificationReason,
	}
	return withPipelineMeta(d), true
}

func withPipelineMeta(d Decision) Decision {
	d.DispatchMode = DispatchModeSingle
	if len(d.Employees) > 0 {
		d.PrimaryEmployee = strings.ToLower(strings.TrimSpace(d.Employees[0]))
	}
	return d
}
