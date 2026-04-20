package inbound

import (
	"encoding/json"
	"slices"
	"sort"
	"testing"
)

func skillIDsSorted(c *CapabilityContractV1) []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c.Skills))
	for _, s := range c.Skills {
		out = append(out, s.ID)
	}
	sort.Strings(out)
	return out
}

func TestDefaultCapabilityContractSkillIDsMatchStruct(t *testing.T) {
	t.Parallel()
	c := DefaultCapabilityContractV1()
	got := skillIDsSorted(c)
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
	if !slices.Equal(skillIDsSorted(c1), skillIDsSorted(&c2)) {
		t.Fatalf("json roundtrip: struct skill ids != unmarshaled json")
	}
}
