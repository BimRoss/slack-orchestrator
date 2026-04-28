package slackrun

import (
	"context"
	"strings"
	"time"

	"github.com/bimross/slack-orchestrator/internal/routing"
)

// humansTermsAccept returns true when the Slack user may receive orchestrator routing (Joanne #humans terms confirmed).
// When nil, enforcement is off (local dev without Redis).
var humansTermsAccept func(ctx context.Context, slackUserID string) bool

// SetHumansTermsAcceptFunc wires a checker (e.g. Redis profile fields). Pass nil to disable enforcement.
func SetHumansTermsAcceptFunc(f func(ctx context.Context, slackUserID string) bool) {
	humansTermsAccept = f
}

// termsEnforcementOutcome applies #humans terms gating for human-authored messages.
// When enforcement is on and the user has not accepted, the only allowed pass-through is
// update-terms-shaped text (see routing.UpdateTermsIntentText), which is delivered to Joanne only
// (routing.Input.PreAcceptanceTermsBypass). Broadcast @here/@channel roots are never allowed before acceptance.
func termsEnforcementOutcome(ctx context.Context, slackUserID, text string) (allow bool, preAcceptanceBypass bool) {
	if humansTermsAccept == nil {
		return true, false
	}
	uid := strings.TrimSpace(slackUserID)
	if uid == "" {
		return false, false
	}
	t := strings.TrimSpace(text)
	if routing.ClassifyBroadcastTrigger(t) != routing.BroadcastNone {
		checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if humansTermsAccept(checkCtx, uid) {
			return true, false
		}
		return false, false
	}
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if humansTermsAccept(checkCtx, uid) {
		return true, false
	}
	if routing.UpdateTermsIntentText(t) {
		return true, true
	}
	return false, false
}
