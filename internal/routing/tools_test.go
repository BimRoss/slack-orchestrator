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
		{"read-google", "", KindConversation},
		{"read google", "", KindConversation},
		{"read_google", "", KindConversation},
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
		{"including read web and create email tools", "", KindConversation},
		{"read web is live in prod", "", KindConversation},
		{"we have read-web in prod", "", KindConversation},
		{"the read web skill is useful", "", KindConversation},
		{"can you read-web about layoffs?", "read-web", KindTool},
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

func TestClassifyToolOrConversationWithReason(t *testing.T) {
	tests := []struct {
		text       string
		wantID     string
		wantKind   Kind
		wantReason string
	}{
		{"read-web", "read-web", KindTool, ClassificationReasonTier1SingleMatch},
		{"read web skill is live", "", KindConversation, ClassificationReasonTier1DescriptiveReference},
		{"including read web and create email tools", "", KindConversation, ClassificationReasonTier1MultiMatch},
		{"hello there", "", KindConversation, ClassificationReasonTier1NoMatch},
	}

	for _, tc := range tests {
		gotID, gotK, gotReason := ClassifyToolOrConversationWithReason(tc.text)
		if gotID != tc.wantID || gotK != tc.wantKind || gotReason != tc.wantReason {
			t.Fatalf("ClassifyToolOrConversationWithReason(%q) = (%q, %s, %q), want (%q, %s, %q)",
				tc.text, gotID, gotK, gotReason, tc.wantID, tc.wantKind, tc.wantReason)
		}
	}
}
