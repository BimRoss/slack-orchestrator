package routing

import (
	"regexp"
	"sort"
	"strings"
)

// tier1ExplicitToolIDs are canonical skill ids (Tier 1 routing). Keep in sync with
// internal/inbound.DefaultCapabilityContractV1 Skills[].ID when adding skills.
//
// Match only when the user explicitly names the tool: "read-company", "read company",
// "read_company", etc. Substrings like "onboard" or "search twitter for …" do not match.
// Keep in sync with internal/inbound.DefaultCapabilityContractV1 Skills[].ID.
var tier1ExplicitToolIDs = []string{
	"write-email",
	"write-doc",
	"write-company",
	"read-company",
	"read-skills",
	"read-twitter",
	"read-trends",
}

var tier1ToolPatterns []tier1Pattern

type tier1Pattern struct {
	toolID string
	re     *regexp.Regexp
}

var reSlackAngleTokens = regexp.MustCompile(`<[@#][^>]*>`)

func init() {
	ids := append([]string(nil), tier1ExplicitToolIDs...)
	sort.SliceStable(ids, func(i, j int) bool {
		if len(ids[i]) != len(ids[j]) {
			return len(ids[i]) > len(ids[j])
		}
		return ids[i] < ids[j]
	})
	for _, id := range ids {
		tier1ToolPatterns = append(tier1ToolPatterns, tier1Pattern{
			toolID: id,
			re:     mustCompileTier1Pattern(id),
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
			return p.toolID, KindTool
		}
	}
	return "", KindConversation
}
