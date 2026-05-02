package routing

import "strings"

// ToolPinnedSkillIDs returns the canonical skill IDs that enable Redis thread-followup pinning.
func ToolPinnedSkillIDs() []string {
	return []string{"create-email", "create-email-welcome", "create-doc", "create-company", "create-issue", "update-issue", "delete-company"}
}

// ToolPinsThreadSkillFollowup is true for Tier-1 mutating skills that typically gather fields
// and/or confirmation in-thread. slack-orchestrator stores a short-lived Redis pin on the
// thread root so plain follow-ups route to the same employee when conversations.replies
// handoff is empty.
func ToolPinsThreadSkillFollowup(toolID string) bool {
	candidate := strings.ToLower(strings.TrimSpace(toolID))
	for _, id := range ToolPinnedSkillIDs() {
		if candidate == id {
			return true
		}
	}
	return false
}
