package slackrun

import (
	"context"
	"testing"
)

func TestTermsEnforcementOutcome_BroadcastBlockedWhenNotAccepted(t *testing.T) {
	SetHumansTermsAcceptFunc(func(ctx context.Context, slackUserID string) bool {
		return false
	})
	t.Cleanup(func() { SetHumansTermsAcceptFunc(nil) })
	allow, bypass := termsEnforcementOutcome(context.Background(), "U1", "<!here> please read update-terms")
	if allow || bypass {
		t.Fatalf("expected broadcast blocked when not accepted, allow=%v bypass=%v", allow, bypass)
	}
}

func TestTermsEnforcementOutcome_UpdateTermsBypass(t *testing.T) {
	SetHumansTermsAcceptFunc(func(ctx context.Context, slackUserID string) bool {
		return false
	})
	t.Cleanup(func() { SetHumansTermsAcceptFunc(nil) })
	allow, bypass := termsEnforcementOutcome(context.Background(), "U1", "show the terms")
	if !allow || !bypass {
		t.Fatalf("expected update-terms bypass, allow=%v bypass=%v", allow, bypass)
	}
}

func TestTermsEnforcementOutcome_AcceptedNoBypass(t *testing.T) {
	SetHumansTermsAcceptFunc(func(ctx context.Context, slackUserID string) bool {
		return true
	})
	t.Cleanup(func() { SetHumansTermsAcceptFunc(nil) })
	allow, bypass := termsEnforcementOutcome(context.Background(), "U1", "hello there")
	if !allow || bypass {
		t.Fatalf("expected normal allow without bypass, allow=%v bypass=%v", allow, bypass)
	}
}
