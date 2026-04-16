package routing

import (
	"strings"
)

// ClassifyToolOrConversation maps keywords to a tool id; ambiguity or no match → conversation only.
func ClassifyToolOrConversation(text string) (toolID string, mode Kind) {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return "", KindConversation
	}
	// Order matters: more specific first.
	if containsAny(t, "read-trends", "trends on twitter", "twitter trends", "trending on") &&
		!isConversationalTrendsAsk(t) {
		return "read-trends", KindTool
	}
	if containsAny(t, "twitter", "x.com", "tweet") && containsAny(t, "search", "lookup", "find", "what's happening") {
		return "read-twitter", KindTool
	}
	if containsAny(t, "twitter", "tweet", "x.com") && strings.Contains(t, "twitter") {
		// Broad "twitter" product mention → still conversation unless clearly a search request.
		if containsAny(t, "search", "look up", "lookup", "find tweets", "min fav") {
			return "read-twitter", KindTool
		}
	}
	if containsAny(t, "write email", "send email", "draft email", "compose email", "mailto:") {
		return "write-email", KindTool
	}
	if containsAny(t, "google doc", "create doc", "write doc", "new doc") {
		return "write-doc", KindTool
	}
	if containsAny(t, "create channel", "invite me", "onboard", "make a company", "write company") {
		return "write-company", KindTool
	}
	if containsAny(t, "read company", "company digest", "channel digest") {
		return "read-company", KindTool
	}
	return "", KindConversation
}

func containsAny(hay string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(hay, n) {
			return true
		}
	}
	return false
}

func isConversationalTrendsAsk(lower string) bool {
	if !strings.Contains(lower, "trend") {
		return false
	}
	return strings.Contains(lower, "you guys") ||
		strings.Contains(lower, "have you noticed") ||
		strings.Contains(lower, "have you seen")
}
