package slackrun

import (
	"context"
	"strings"
	"time"
)

// humansTermsAccept returns true when the Slack user may receive orchestrator routing (Joanne #humans terms confirmed).
// When nil, enforcement is off (local dev without Redis).
var humansTermsAccept func(ctx context.Context, slackUserID string) bool

// SetHumansTermsAcceptFunc wires a checker (e.g. Redis profile fields). Pass nil to disable enforcement.
func SetHumansTermsAcceptFunc(f func(ctx context.Context, slackUserID string) bool) {
	humansTermsAccept = f
}

func posterMayUseOrchestratorRouting(ctx context.Context, slackUserID string) bool {
	if humansTermsAccept == nil {
		return true
	}
	uid := strings.TrimSpace(slackUserID)
	if uid == "" {
		return false
	}
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return humansTermsAccept(checkCtx, uid)
}
