package routing

import "testing"

func TestToolPinsThreadSkillFollowup(t *testing.T) {
	if !ToolPinsThreadSkillFollowup("create-email") {
		t.Fatal("create-email should pin")
	}
	if ToolPinsThreadSkillFollowup("read-company") {
		t.Fatal("read-company should not pin")
	}
	if !ToolPinsThreadSkillFollowup("  DELETE-COMPANY ") {
		t.Fatal("trim/case insensitive delete-company should pin")
	}
	if !ToolPinsThreadSkillFollowup("update-company") {
		t.Fatal("update-company should pin")
	}
}
