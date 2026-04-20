package routing_test

import (
	"testing"

	"github.com/bimross/slack-orchestrator/internal/inbound"
	"github.com/bimross/slack-orchestrator/internal/routing"
)

func TestTier1CanonicalSkillIDsAreSubsetOfDefaultContract(t *testing.T) {
	t.Parallel()
	allowed := inbound.DefaultCapabilityContractSkillIDSet()
	for _, id := range routing.Tier1CanonicalSkillIDs() {
		if _, ok := allowed[id]; !ok {
			t.Fatalf("tier1 canonical id %q is not in DefaultCapabilityContractV1 Skills", id)
		}
	}
}
