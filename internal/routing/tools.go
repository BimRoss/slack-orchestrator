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
	{"create-email-welcome", "create-email-welcome"},
	{"write-email-welcome", "create-email-welcome"},
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
	{"read-backend", "read-backend"},
	{"update-issue", "update-issue"},
	{"edit-issue", "update-issue"},
	{"update-an-issue", "update-issue"},
	{"update-a-issue", "update-issue"},
	{"edit-an-issue", "update-issue"},
	{"edit-a-issue", "update-issue"},
	{"delete-company", "delete-company"},
	{"update-company", "update-company"},
	{"read-company", "read-company"},
	{"read-google", "read-web"},
	{"read-web", "read-web"},
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

type tier1Match struct {
	canonicalID string
	start       int
	end         int
}

var reSlackAngleTokens = regexp.MustCompile(`<[@#][^>]*>`)

const (
	ClassificationReasonTier1SingleMatch          = "tier1_single_match"
	ClassificationReasonTier1NoMatch              = "tier1_no_match"
	ClassificationReasonTier1MultiMatch           = "tier1_multi_match_conversation"
	ClassificationReasonTier1DescriptiveReference = "tier1_descriptive_reference_conversation"
)

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

func matchedCanonicalTier1(text string) []tier1Match {
	if text == "" {
		return nil
	}
	seen := make(map[string]struct{}, 2)
	var out []tier1Match
	// Patterns iterate in length-descending order (see init), so longer compound
	// skill ids (e.g. "create-email-welcome") are evaluated before their substrings
	// (e.g. "create-email"). Skip a candidate when its span sits fully inside any
	// already-added match so a single explicit name does not register two intents.
	for _, p := range tier1ToolPatterns {
		loc := p.re.FindStringIndex(text)
		if loc == nil {
			continue
		}
		if _, ok := seen[p.canonicalID]; ok {
			continue
		}
		if isRangeCoveredByExistingMatch(loc[0], loc[1], out) {
			continue
		}
		seen[p.canonicalID] = struct{}{}
		out = append(out, tier1Match{
			canonicalID: p.canonicalID,
			start:       loc[0],
			end:         loc[1],
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].start < out[j].start
	})
	return out
}

func isRangeCoveredByExistingMatch(start, end int, existing []tier1Match) bool {
	for _, m := range existing {
		if start >= m.start && end <= m.end {
			return true
		}
	}
	return false
}

func firstWord(s string) string {
	for _, tok := range strings.Fields(s) {
		w := strings.Trim(tok, ".,!?;:()[]{}\"'")
		if w != "" {
			return w
		}
	}
	return ""
}

func lastWord(s string) string {
	toks := strings.Fields(s)
	for i := len(toks) - 1; i >= 0; i-- {
		w := strings.Trim(toks[i], ".,!?;:()[]{}\"'")
		if w != "" {
			return w
		}
	}
	return ""
}

func secondWord(s string) string {
	count := 0
	for _, tok := range strings.Fields(s) {
		w := strings.Trim(tok, ".,!?;:()[]{}\"'")
		if w == "" {
			continue
		}
		count++
		if count == 2 {
			return w
		}
	}
	return ""
}

func isDescriptiveSkillReference(text string, m tier1Match) bool {
	before := strings.TrimSpace(text[:m.start])
	after := strings.TrimSpace(text[m.end:])
	beforeLast := lastWord(before)
	afterFirst := firstWord(after)
	afterSecond := secondWord(after)

	switch beforeLast {
	case "including", "include", "includes", "with", "have", "has", "seeing", "saw", "core":
		return true
	}
	switch afterFirst {
	case "tool", "tools", "skill", "skills", "enabled", "available", "live":
		return true
	case "is":
		switch afterSecond {
		case "enabled", "available", "live":
			return true
		}
	}
	return false
}

// ClassifyToolOrConversation is Tier 1: explicit tool name only (hyphen / space / underscore
// between segments). No broad keyword intent. No match → conversation (Tier 2 is future work).
func ClassifyToolOrConversation(text string) (toolID string, mode Kind) {
	toolID, mode, _ = ClassifyToolOrConversationWithReason(text)
	return toolID, mode
}

// ClassifyToolOrConversationWithReason returns Tier-1 classification plus a stable reason code
// for routing telemetry.
func ClassifyToolOrConversationWithReason(text string) (toolID string, mode Kind, reason string) {
	t := normalizeMessageForTier1(text)
	if t == "" {
		return "", KindConversation, ClassificationReasonTier1NoMatch
	}
	matches := matchedCanonicalTier1(t)
	if len(matches) == 1 {
		m := matches[0]
		if isDescriptiveSkillReference(t, m) {
			return "", KindConversation, ClassificationReasonTier1DescriptiveReference
		}
		return m.canonicalID, KindTool, ClassificationReasonTier1SingleMatch
	}
	if len(matches) > 1 {
		return "", KindConversation, ClassificationReasonTier1MultiMatch
	}
	return "", KindConversation, ClassificationReasonTier1NoMatch
}
