package inbound

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"testing"

	"github.com/bimross/slack-orchestrator/internal/contractsync"
)

func TestDefaultCapabilityContractSkillIDsMatchStruct(t *testing.T) {
	t.Parallel()
	c := DefaultCapabilityContractV1()
	got := skillIDsFromContract(c)
	want := DefaultCapabilityContractSkillIDs()
	if !slices.Equal(got, want) {
		t.Fatalf("skill id helper drift: got %v want %v", got, want)
	}
}

func TestDefaultCapabilityContractJSONMatchesStruct(t *testing.T) {
	t.Parallel()
	c1 := DefaultCapabilityContractV1()
	raw := DefaultCapabilityContractJSON()
	var c2 CapabilityContractV1
	if err := json.Unmarshal(raw, &c2); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(skillIDsFromContract(c1), skillIDsFromContract(&c2)) {
		t.Fatalf("json roundtrip: struct skill ids != unmarshaled json")
	}
}

func TestDefaultCapabilityContractMetadataMatchesPayload(t *testing.T) {
	t.Parallel()

	raw := DefaultCapabilityContractJSON()
	if len(raw) == 0 {
		t.Fatal("expected non-empty default capability contract json")
	}
	if got, want := DefaultCapabilityContractRevision(), "default"; got != want {
		t.Fatalf("revision: got %q want %q", got, want)
	}
	sum := sha256.Sum256(raw)
	wantDigest := hex.EncodeToString(sum[:8])
	if got := DefaultCapabilityContractDigest(); got != wantDigest {
		t.Fatalf("digest: got %q want %q", got, wantDigest)
	}
}

func TestDefaultCapabilityContractSkillIDsMatchGeneratedContract(t *testing.T) {
	t.Parallel()

	got := DefaultCapabilityContractSkillIDs()
	want := append([]string(nil), contractsync.GeneratedSkillIDs...)
	if !slices.Equal(got, want) {
		t.Fatalf("generated contract drift: got %v want %v", got, want)
	}
}
