package routing_test

import (
	"slices"
	"testing"

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
