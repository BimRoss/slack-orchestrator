package slackrun

import (
	"strings"
	"testing"
)

func TestTruncatePreview(t *testing.T) {
	if got := truncatePreview("hi", 100); got != "hi" {
		t.Fatalf("short string: got %q", got)
	}
	s := strings.Repeat("a", 101)
	got := truncatePreview(s, 100)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis suffix, got %q", got)
	}
	if strings.Count(got, "a") != 100 {
		t.Fatalf("expected 100 a runes before ellipsis")
	}
}
