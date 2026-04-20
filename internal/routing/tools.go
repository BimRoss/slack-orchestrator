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
var tier1PatternEntries = []struct {
	patternID   string
	canonicalID string
}{
	{"create-email", "create-email"},
	{"write-email", "create-email"},
	{"create-doc", "create-doc"},
	{"write-doc", "create-doc"},
	{"create-company", "create-company"},
	{"write-company", "create-company"},
	{"read-company", "read-company"},
	{"read-skills", "read-skills"},
	{"read-twitter", "read-twitter"},
	{"read-trends", "read-trends"},
}

var tier1ToolPatterns []tier1Pattern

type tier1Pattern struct {
	canonicalID string
	re          *regexp.Regexp
}

var reSlackAngleTokens = regexp.MustCompile(`<[@#][^>]*>`)

func init() {
	entries := append([]struct {
		patternID   string
		canonicalID string
	}(nil), tier1PatternEntries...)
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
