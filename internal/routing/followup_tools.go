package routing

import "strings"

// ToolPinsThreadSkillFollowup is true for Tier-1 mutating skills that typically gather fields
// and/or confirmation in-thread. slack-orchestrator stores a short-lived Redis pin on the
// thread root so plain follow-ups route to the same employee when conversations.replies
// handoff is empty.
func ToolPinsThreadSkillFollowup(toolID string) bool {
	switch strings.ToLower(strings.TrimSpace(toolID)) {
	case "create-email", "create-doc", "create-company", "delete-company":
		return true
	default:
		return false
	}
}
