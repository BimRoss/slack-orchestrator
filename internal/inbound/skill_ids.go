package inbound

import (
	"sort"
	"strings"
)

// skillIDsFromContract returns sorted canonical skill IDs from c, matching [DefaultCapabilityContractSkillIDs]
// normalization (trim whitespace, omit empty).
func skillIDsFromContract(c *CapabilityContractV1) []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c.Skills))
	for _, s := range c.Skills {
		id := strings.TrimSpace(s.ID)
		if id == "" {
			continue
		}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// DefaultCapabilityContractSkillIDs returns sorted canonical skill IDs from [DefaultCapabilityContractV1].
// Use this instead of hand-maintained slices when checking tier-1 routing, pipelines, or docs.
func DefaultCapabilityContractSkillIDs() []string {
	return skillIDsFromContract(DefaultCapabilityContractV1())
}

// DefaultCapabilityContractSkillIDSet is a set view of [DefaultCapabilityContractSkillIDs].
func DefaultCapabilityContractSkillIDSet() map[string]struct{} {
	ids := DefaultCapabilityContractSkillIDs()
	m := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return m
}
