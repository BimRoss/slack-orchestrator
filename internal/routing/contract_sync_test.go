package routing_test

import (
	"slices"
	"sort"
	"testing"

	"github.com/bimross/slack-orchestrator/internal/contractsync"
	"github.com/bimross/slack-orchestrator/internal/inbound"
	"github.com/bimross/slack-orchestrator/internal/routing"
)

func TestTier1CanonicalSkillIDsExistInCapabilityContract(t *testing.T) {
	t.Parallel()

	contractIDs := inbound.DefaultCapabilityContractSkillIDs()
	for _, id := range routing.Tier1CanonicalSkillIDs() {
		if !slices.Contains(contractIDs, id) {
			t.Fatalf("tier1 skill id %q missing from capability contract", id)
		}
	}
}

func TestThreadPinnedSkillIDsExistInCapabilityContract(t *testing.T) {
	t.Parallel()

	contractIDs := inbound.DefaultCapabilityContractSkillIDs()
	for _, id := range routing.ToolPinnedSkillIDs() {
		if !slices.Contains(contractIDs, id) {
			t.Fatalf("thread-pinned skill id %q missing from capability contract", id)
		}
	}
}

func TestTier1AliasesMatchGeneratedContract(t *testing.T) {
	t.Parallel()

	got := routing.Tier1PatternEntries()
	aliases := make([]string, 0, len(got))
	for _, entry := range got {
		aliases = append(aliases, entry.PatternID+"=>"+entry.CanonicalID)
	}
	sort.Strings(aliases)

	want := make([]string, 0, len(contractsync.GeneratedTier1Aliases))
	for _, entry := range contractsync.GeneratedTier1Aliases {
		want = append(want, entry.PatternID+"=>"+entry.CanonicalID)
	}
	sort.Strings(want)
	if !slices.Equal(aliases, want) {
		t.Fatalf("tier1 alias drift: got %v want %v", aliases, want)
	}
}

func TestThreadPinnedSkillIDsMatchGeneratedContract(t *testing.T) {
	t.Parallel()

	got := append([]string(nil), routing.ToolPinnedSkillIDs()...)
	want := append([]string(nil), contractsync.GeneratedThreadPinSkillIDs...)
	sort.Strings(got)
	sort.Strings(want)
	if !slices.Equal(got, want) {
		t.Fatalf("thread-pinned ids drift: got %v want %v", got, want)
	}
}
