package routing

import "testing"

func TestClassifyToolOrConversation_ExplicitTier1(t *testing.T) {
	tests := []struct {
		text     string
		wantID   string
		wantKind Kind
	}{
		{"read-company", "read-company", KindTool},
		{"read company", "read-company", KindTool},
		{"read_company", "read-company", KindTool},
		{"Read Company", "read-company", KindTool},
		{"read-skills", "read-skills", KindTool},
		{"read skills", "read-skills", KindTool},
		{"read_skills", "read-skills", KindTool},
		{"write-email for the team", "write-email", KindTool},
		{"please write doc", "write-doc", KindTool},
		{"write-company", "write-company", KindTool},
		{"write company", "write-company", KindTool},
		{"read-twitter", "read-twitter", KindTool},
		{"read twitter", "read-twitter", KindTool},
		{"read-trends", "read-trends", KindTool},
		{"<@UGARTH> read-twitter", "read-twitter", KindTool},
		{"search twitter for bitcoin", "", KindConversation},
		{"they will onboard next week", "", KindConversation},
		{"create channel please", "", KindConversation},
		{"we have email and twitter tooling", "", KindConversation},
		{"", "", KindConversation},
	}
	for _, tc := range tests {
		gotID, gotK := ClassifyToolOrConversation(tc.text)
		if gotID != tc.wantID || gotK != tc.wantKind {
			t.Fatalf("ClassifyToolOrConversation(%q) = (%q, %s), want (%q, %s)",
				tc.text, gotID, gotK, tc.wantID, tc.wantKind)
		}
	}
}
