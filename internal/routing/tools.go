package routing

import (
	"regexp"
	"sort"
	"strings"
)

// tier1PatternEntry binds a message pattern (hyphenated skill id) to the canonical skill id
// returned from Tier 1. Legacy write-* patterns map to create-* (CRUD naming).
//
// Keep canonical ids in sync with internal/inbound.DefaultCapabilityContractV1 Skills[].ID.
//
// Match only when the user explicitly names the tool: "read-company", "read company",
// "read_company", etc. Substrings like "onboard" or "search twitter for …" do not match.
type Tier1PatternEntry struct {
	PatternID   string
	CanonicalID string
}

var tier1PatternEntries = []Tier1PatternEntry{
	{"create-email", "create-email"},
	{"write-email", "create-email"},
	{"create-doc", "create-doc"},
	{"write-doc", "create-doc"},
	{"create-image", "create-image"},
	{"write-image", "create-image"},
	{"generate-image", "create-image"},
	{"create-company", "create-company"},
	{"write-company", "create-company"},
	{"create-connect", "create-connect"},
	{"write-connect", "create-connect"},
	{"create-issue", "create-issue"},
	{"create-an-issue", "create-issue"},
	{"create-a-issue", "create-issue"},
	{"file-issue", "create-issue"},
	{"file-an-issue", "create-issue"},
	{"file-a-issue", "create-issue"},
	{"open-issue", "create-issue"},
	{"open-an-issue", "create-issue"},
	{"open-a-issue", "create-issue"},
	{"write-issue", "create-issue"},
	{"read-issue", "read-issue"},
	{"read-issues", "read-issue"},
	{"read-an-issue", "read-issue"},
	{"read-a-issue", "read-issue"},
	{"update-issue", "update-issue"},
	{"edit-issue", "update-issue"},
	{"update-an-issue", "update-issue"},
	{"update-a-issue", "update-issue"},
	{"edit-an-issue", "update-issue"},
	{"edit-a-issue", "update-issue"},
	{"delete-company", "delete-company"},
	{"read-company", "read-company"},
	{"read-web", "read-web"},
	{"read-internet", "read-web"},
	{"read-google", "read-web"},
	{"read-skills", "read-skills"},
	{"read-user", "read-user"},
	{"read-twitter", "read-twitter"},
	{"read-trends", "read-trends"},
	{"update-terms", "update-terms"},
}

// Tier1PatternEntries returns a copy of raw Tier-1 aliases/pattern bindings.
func Tier1PatternEntries() []Tier1PatternEntry {
	out := make([]Tier1PatternEntry, len(tier1PatternEntries))
	copy(out, tier1PatternEntries)
	return out
}

var tier1ToolPatterns []tier1Pattern

// Tier1CanonicalSkillIDs returns sorted unique canonical skill IDs bound in [tier1PatternEntries].
func Tier1CanonicalSkillIDs() []string {
	seen := map[string]struct{}{}
	for _, e := range tier1PatternEntries {
		id := strings.TrimSpace(e.CanonicalID)
		if id == "" {
			continue
		}
		seen[id] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

type tier1Pattern struct {
	canonicalID string
	re          *regexp.Regexp
}

var reSlackAngleTokens = regexp.MustCompile(`<[@#][^>]*>`)

func init() {
	entries := make([]struct {
		patternID   string
		canonicalID string
	}, 0, len(tier1PatternEntries))
	for _, entry := range tier1PatternEntries {
		entries = append(entries, struct {
			patternID   string
			canonicalID string
		}{patternID: entry.PatternID, canonicalID: entry.CanonicalID})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if len(entries[i].patternID) != len(entries[j].patternID) {
			return len(entries[i].patternID) > len(entries[j].patternID)
		}
		return entries[i].patternID < entries[j].patternID
	})
	for _, e := range entries {
		tier1ToolPatterns = append(tier1ToolPatterns, tier1Pattern{
			canonicalID: e.canonicalID,
			re:          mustCompileTier1Pattern(e.patternID),
		})
	}
}

func mustCompileTier1Pattern(toolID string) *regexp.Regexp {
	parts := strings.Split(toolID, "-")
	if len(parts) < 2 {
		panic("routing: tier1 tool id must have at least two segments: " + toolID)
	}
	var b strings.Builder
	b.WriteString(`(?i)\b`)
	for i, p := range parts {
		if i > 0 {
			b.WriteString(`[\s_-]+`)
		}
		b.WriteString(regexp.QuoteMeta(p))
	}
	b.WriteString(`\b`)
	return regexp.MustCompile(b.String())
}

func normalizeMessageForTier1(text string) string {
	t := strings.TrimSpace(text)
	t = reSlackAngleTokens.ReplaceAllString(t, " ")
	t = strings.ToLower(t)
	t = strings.Join(strings.Fields(t), " ")
	return t
}

// ClassifyToolOrConversation is Tier 1: explicit tool name only (hyphen / space / underscore
// between segments). No broad keyword intent. No match → conversation (Tier 2 is future work).
func ClassifyToolOrConversation(text string) (toolID string, mode Kind) {
	t := normalizeMessageForTier1(text)
	if t == "" {
		return "", KindConversation
	}
	for _, p := range tier1ToolPatterns {
		if p.re.MatchString(t) {
			return p.canonicalID, KindTool
		}
	}
	return "", KindConversation
}
