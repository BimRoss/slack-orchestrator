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
		{"read-web", "read-web", KindTool},
		{"read internet", "", KindConversation},
		{"read_web", "read-web", KindTool},
		{"read-google", "read-web", KindTool},
		{"read google", "read-web", KindTool},
		{"read_google", "read-web", KindTool},
		{"read-skills", "read-skills", KindTool},
		{"read skills", "read-skills", KindTool},
		{"read_skills", "read-skills", KindTool},
		{"read-user", "read-user", KindTool},
		{"read user", "read-user", KindTool},
		{"read_user", "read-user", KindTool},
		{"create-email for the team", "create-email", KindTool},
		{"write-email for the team", "create-email", KindTool},
		{"please create doc", "create-doc", KindTool},
		{"please write doc", "create-doc", KindTool},
		{"create-company", "create-company", KindTool},
		{"write-company", "create-company", KindTool},
		{"write company", "create-company", KindTool},
		{"create company", "create-company", KindTool},
		{"create-issue", "create-issue", KindTool},
		{"create issue", "create-issue", KindTool},
		{"create an issue", "create-issue", KindTool},
		{"file an issue", "create-issue", KindTool},
		{"open an issue", "create-issue", KindTool},
		{"write-issue", "create-issue", KindTool},
		{"read-issue", "read-issue", KindTool},
		{"read issue", "read-issue", KindTool},
		{"read issues", "read-issue", KindTool},
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
