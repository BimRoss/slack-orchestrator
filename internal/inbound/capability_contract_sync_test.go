package inbound

import (
	"encoding/json"
	"slices"
	"testing"
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
