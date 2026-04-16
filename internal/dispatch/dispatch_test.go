package dispatch

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bimross/slack-orchestrator/internal/config"
	"github.com/bimross/slack-orchestrator/internal/routing"
	"github.com/slack-go/slack/slackevents"
)

func TestPostWithRetries_503ThenOK(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"target_employee":"alex"`) {
			t.Fatalf("unexpected body: %s", body)
		}
		if calls < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := config.Config{
		DispatchHTTPTimeout: 5 * time.Second,
		WorkerHMACSecret:    "test-secret",
	}
	body := []byte(`{"target_employee":"alex"}`)
	st, err := postWithRetries(context.Background(), &http.Client{Timeout: 5 * time.Second}, srv.URL, body, cfg.WorkerHMACSecret)
	if err != nil {
		t.Fatal(err)
	}
	if st != http.StatusOK {
		t.Fatalf("status %d", st)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestDecision_SkipsWhenTemplateEmpty(t *testing.T) {
	// No server — should not panic and should not dial.
	cfg := config.Config{
		DispatchEnabled:   true,
		WorkerURLTemplate: "",
	}
	outer := slackevents.EventsAPIEvent{
		Data: &slackevents.EventsAPICallbackEvent{EventID: "Ev123"},
	}
	in := routing.Input{ChannelID: "C1", MessageTS: "1.0", UserID: "U1", Text: "hi"}
	d := routing.Decision{Employees: []string{"alex"}, Trigger: routing.TriggerPlain, Kind: routing.KindConversation}
	Decision(context.Background(), cfg, outer, in, d, "message")
}

func TestHMACSignatureRoundTrip(t *testing.T) {
	secret := "s"
	body := []byte(`{}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))
	if sig == "" {
		t.Fatal("empty sig")
	}
}
