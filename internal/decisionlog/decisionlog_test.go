package decisionlog

import (
	"testing"
	"time"

	"github.com/bimross/slack-orchestrator/internal/routing"
)

func TestStore_SnapshotOrder(t *testing.T) {
	s := New(10)
	s.Append(Entry{Time: time.Unix(1, 0).UTC(), TextPreview: "a", Decision: routing.Decision{Trigger: routing.TriggerPlain}})
	s.Append(Entry{Time: time.Unix(2, 0).UTC(), TextPreview: "b", Decision: routing.Decision{Trigger: routing.TriggerPlain}})
	out := s.Snapshot(10)
	if len(out) != 2 || out[0].TextPreview != "a" || out[1].TextPreview != "b" {
		t.Fatalf("expected oldest-first a,b got %#v", out)
	}
	rev := s.Snapshot(1)
	if len(rev) != 1 || rev[0].TextPreview != "b" {
		t.Fatalf("expected last entry b got %#v", rev)
	}
}
