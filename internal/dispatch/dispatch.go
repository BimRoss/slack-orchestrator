package dispatch

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bimross/slack-orchestrator/internal/config"
	"github.com/bimross/slack-orchestrator/internal/decisionlog"
	"github.com/bimross/slack-orchestrator/internal/inbound"
	"github.com/bimross/slack-orchestrator/internal/metrics"
	"github.com/bimross/slack-orchestrator/internal/routing"
	"github.com/slack-go/slack/slackevents"
)

const (
	pathOrchestratorEvent = "/internal/slack/event"
	headerSignature       = "X-BimRoss-Orchestrator-Signature"
	signaturePrefix       = "v1="
)

// Decision posts one JSON body per target employee in d.Employees.
func Decision(ctx context.Context, cfg config.Config, outer slackevents.EventsAPIEvent, in routing.Input, d routing.Decision, innerType string) []decisionlog.DispatchResult {
	if !cfg.DispatchEnabled {
		return nil
	}
	tpl := strings.TrimSpace(cfg.WorkerURLTemplate)
	if tpl == "" {
		slog.Warn("orchestrator_dispatch_skip", "reason", "missing_ORCHESTRATOR_WORKER_URL_TEMPLATE")
		for range d.Employees {
			metrics.DelegatePostTotal.WithLabelValues("skipped").Inc()
		}
		return nil
	}
	if len(d.Employees) == 0 {
		return nil
	}
	var results []decisionlog.DispatchResult

	eventID, eventTime, teamID, apiAppID := callbackMeta(outer)
	payloadBase := inbound.EventV1{
		SchemaVersion:  inbound.SchemaVersion,
		SlackEventID:   eventID,
		SlackEventTime: eventTime,
		TeamID:         teamID,
		APIAppID:       apiAppID,
		InnerType:      innerType,
		Decision:       d,
		Message: inbound.MessageV1{
			ChannelID: in.ChannelID,
			ThreadTS:  in.ThreadTS,
			MessageTS: in.MessageTS,
			UserID:    in.UserID,
			Text:      in.Text,
		},
	}

	client := &http.Client{Timeout: cfg.DispatchHTTPTimeout}
	for _, emp := range d.Employees {
		emp = strings.ToLower(strings.TrimSpace(emp))
		if emp == "" {
			continue
		}
		payload := payloadBase
		payload.TargetEmployee = emp

		body, err := json.Marshal(payload)
		if err != nil {
			slog.Error("orchestrator_dispatch_marshal", "error", err, "employee", emp)
			metrics.DelegatePostTotal.WithLabelValues("failure").Inc()
			metrics.DelegatePostErrorsTotal.Inc()
			results = append(results, decisionlog.DispatchResult{Employee: emp, OK: false, Error: err.Error()})
			continue
		}

		baseURL := strings.TrimRight(expandTemplate(tpl, emp), "/")
		url := baseURL + pathOrchestratorEvent

		start := time.Now()
		status, err := postWithRetries(ctx, client, url, body, cfg.WorkerHMACSecret)
		elapsed := time.Since(start).Seconds()
		if err != nil {
			slog.Error("orchestrator_dispatch_post", "error", err, "employee", emp, "url", url)
			metrics.DelegateHTTPRequestSeconds.WithLabelValues("failure").Observe(elapsed)
			metrics.DelegatePostTotal.WithLabelValues("failure").Inc()
			metrics.DelegatePostErrorsTotal.Inc()
			results = append(results, decisionlog.DispatchResult{Employee: emp, OK: false, HTTPStatus: status, Error: err.Error()})
			continue
		}
		metrics.DelegateHTTPRequestSeconds.WithLabelValues("success").Observe(elapsed)
		metrics.DelegatePostTotal.WithLabelValues("success").Inc()
		slog.Info("orchestrator_dispatch_ok", "employee", emp, "http_status", status)
		results = append(results, decisionlog.DispatchResult{Employee: emp, OK: true, HTTPStatus: status})
	}
	return results
}

func callbackMeta(ev slackevents.EventsAPIEvent) (eventID string, eventTime int, teamID, apiAppID string) {
	if cb, ok := ev.Data.(*slackevents.EventsAPICallbackEvent); ok && cb != nil {
		return cb.EventID, cb.EventTime, cb.TeamID, cb.APIAppID
	}
	return "", 0, strings.TrimSpace(ev.TeamID), strings.TrimSpace(ev.APIAppID)
}

func expandTemplate(tpl, employeeKey string) string {
	return strings.ReplaceAll(tpl, "{employee}", employeeKey)
}

func postWithRetries(ctx context.Context, client *http.Client, url string, body []byte, secret string) (httpStatus int, err error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(time.Duration(50*(1<<uint(attempt-1))) * time.Millisecond):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return 0, err
		}
		req.Header.Set("Content-Type", "application/json")
		if secret != "" {
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(body)
			req.Header.Set(headerSignature, signaturePrefix+hex.EncodeToString(mac.Sum(nil)))
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusServiceUnavailable && attempt < maxAttempts-1 {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return resp.StatusCode, nil
	}
	if lastErr != nil {
		return 0, lastErr
	}
	return 0, fmt.Errorf("exhausted retries")
}
