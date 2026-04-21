package routing

import "strings"

// MatchCompanyOnboardingPath returns a non-empty branch when text matches Joanne's company-channel
// onboarding classifier (1/2/3 and natural-language variants). Kept in sync with
// employee-factory classifyCompanyOnboardingPath so orchestrator can route plain messages to joanne.
func MatchCompanyOnboardingPath(text string) string {
	low := strings.ToLower(strings.TrimSpace(text))
	if low == "" {
		return ""
	}
	if low == "3" || low == "three" || low == "option 3" || low == "#3" {
		return "ideate"
	}
	if low == "2" || low == "two" || low == "option 2" || low == "#2" {
		return "fresh"
	}
	if low == "1" || low == "one" || low == "option 1" || low == "#1" {
		return "existing"
	}
	if low == "idea" || onboardingContainsAny(low, "ideat", "ideation", "brainstorm", "need an idea", "need idea", "not sure", "figure it out", "shape the idea", "help me think", "no idea yet") {
		return "ideate"
	}
	if onboardingContainsAny(low, "existing", "already have", "already running", "current business", "my company", "established company") {
		return "existing"
	}
	if low == "new" || strings.HasPrefix(low, "new ") || onboardingContainsAny(low, "new company", "starting fresh", "from scratch", "something new", "start a company", "start company") {
		return "fresh"
	}
	return ""
}

func onboardingContainsAny(s string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(s, term) {
			return true
		}
	}
	return false
}
