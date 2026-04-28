package routing

import "strings"

// UpdateTermsIntentText matches employee-factory slackbot.updateTermsKeywordIntent: explicit
// update-terms phrasing and natural language to read or agree to platform (Make A Company) user terms.
// When slack-orchestrator enforces #humans acceptance, this is the allowlist for a single
// pre-acceptance pass-through to Joanne (update-terms) only. Keep in sync with employee-factory.
func UpdateTermsIntentText(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if strings.Contains(lower, "update-terms") || strings.Contains(lower, "update terms") || strings.Contains(lower, "update_terms") {
		return true
	}
	if strings.Contains(lower, "terms and conditions") {
		return true
	}
	if strings.Contains(lower, "terms of use") || strings.Contains(lower, "terms of service") {
		return true
	}
	if strings.Contains(lower, "the terms") {
		if strings.Contains(lower, "agree") || strings.Contains(lower, "accept") || strings.Contains(lower, "acknowledge") {
			return true
		}
		if strings.Contains(lower, "show me") || strings.Contains(lower, "show the") || strings.Contains(lower, "read the") || strings.Contains(lower, "view the") || strings.Contains(lower, "see the") || strings.Contains(lower, "review the") {
			return true
		}
	}
	return false
}
