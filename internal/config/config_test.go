package config

import (
	"os"
	"reflect"
	"testing"
)

func TestParseBotUserMap_positional(t *testing.T) {
	got := parseBotUserMap("U1,U2,U3")
	want := map[string]string{"U1": "alex", "U2": "tim", "U3": "ross"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestParseBotUserMap_explicit(t *testing.T) {
	got := parseBotUserMap("garth=UG1,alex=UA1")
	want := map[string]string{"UG1": "garth", "UA1": "alex"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestFromEnv_dispatchPublishClamps(t *testing.T) {
	t.Setenv("ORCHESTRATOR_DISPATCH_PUBLISH_MAX_ATTEMPTS", "0")
	t.Setenv("ORCHESTRATOR_DISPATCH_PUBLISH_RETRY_BASE_MS", "999999")
	// Avoid picking up developer tokens from the outer environment.
	t.Setenv("SLACK_BOT_TOKEN", "")
	t.Setenv("SLACK_APP_TOKEN", "")
	t.Setenv("ORCHESTRATOR_SLACK_BOT_TOKEN", "")
	t.Setenv("ORCHESTRATOR_SLACK_APP_TOKEN", "")
	t.Setenv("MULTIAGENT_BOT_USER_IDS", "")
	_ = os.Unsetenv("MULTIAGENT_SHUFFLE_SECRET")

	cfg := FromEnv()
	if cfg.DispatchPublishMaxAttempts != 1 {
		t.Fatalf("DispatchPublishMaxAttempts: got %d want 1", cfg.DispatchPublishMaxAttempts)
	}
	if cfg.DispatchPublishRetryBaseMS != 5000 {
		t.Fatalf("DispatchPublishRetryBaseMS: got %d want 5000", cfg.DispatchPublishRetryBaseMS)
	}

	t.Setenv("ORCHESTRATOR_DISPATCH_PUBLISH_MAX_ATTEMPTS", "99")
	cfg2 := FromEnv()
	if cfg2.DispatchPublishMaxAttempts != 10 {
		t.Fatalf("DispatchPublishMaxAttempts upper clamp: got %d want 10", cfg2.DispatchPublishMaxAttempts)
	}
}
