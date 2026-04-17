package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bimross/slack-orchestrator/internal/config"
	"github.com/bimross/slack-orchestrator/internal/decisionlog"
	"github.com/bimross/slack-orchestrator/internal/inbound"
	"github.com/bimross/slack-orchestrator/internal/metrics"
	"github.com/bimross/slack-orchestrator/internal/routing"
	"github.com/nats-io/nats.go"
	"github.com/slack-go/slack/slackevents"
)

var (
	jsMu      sync.Mutex
	jsConn    *nats.Conn
	jsCtx     nats.JetStreamContext
	jsURLUsed string
)

// Decision publishes one JSON message per target employee to JetStream (subject slack.work.<employee>.events).
func Decision(ctx context.Context, cfg config.Config, outer slackevents.EventsAPIEvent, in routing.Input, d routing.Decision, innerType string) []decisionlog.DispatchResult {
	_ = ctx
	if !cfg.DispatchEnabled {
		return nil
	}
	if strings.TrimSpace(cfg.NatsURL) == "" {
		slog.Warn("orchestrator_dispatch_skip", "reason", "missing_ORCHESTRATOR_NATS_URL")
		for range d.Employees {
			metrics.DelegatePublishTotal.WithLabelValues("skipped").Inc()
		}
		return nil
	}
	if len(d.Employees) == 0 {
		return nil
	}

	js, err := jetStreamContext(cfg)
	if err != nil {
		slog.Error("orchestrator_dispatch_nats", "error", err)
		for range d.Employees {
			metrics.DelegatePublishTotal.WithLabelValues("failure").Inc()
			metrics.DelegatePublishErrorsTotal.Inc()
		}
		return nil
	}

	stream := strings.TrimSpace(cfg.NatsStream)
	if stream == "" {
		stream = "SLACK_WORK"
	}
	if err := ensureStream(js, stream); err != nil {
		slog.Error("orchestrator_dispatch_stream", "error", err)
		for range d.Employees {
			metrics.DelegatePublishTotal.WithLabelValues("failure").Inc()
			metrics.DelegatePublishErrorsTotal.Inc()
		}
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
		Capabilities: inbound.DefaultCapabilityContractJSON(),
	}

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
			metrics.DelegatePublishTotal.WithLabelValues("failure").Inc()
			metrics.DelegatePublishErrorsTotal.Inc()
			results = append(results, decisionlog.DispatchResult{Employee: emp, OK: false, Error: err.Error()})
			continue
		}

		subject := fmt.Sprintf("slack.work.%s.events", emp)
		start := time.Now()
		if _, err := js.Publish(subject, body); err != nil {
			slog.Error("orchestrator_dispatch_publish", "error", err, "employee", emp, "subject", subject)
			metrics.DelegatePublishSeconds.WithLabelValues("failure").Observe(time.Since(start).Seconds())
			metrics.DelegatePublishTotal.WithLabelValues("failure").Inc()
			metrics.DelegatePublishErrorsTotal.Inc()
			results = append(results, decisionlog.DispatchResult{Employee: emp, OK: false, Error: err.Error()})
			continue
		}
		metrics.DelegatePublishSeconds.WithLabelValues("success").Observe(time.Since(start).Seconds())
		metrics.DelegatePublishTotal.WithLabelValues("success").Inc()
		slog.Info("orchestrator_dispatch_ok", "employee", emp, "subject", subject)
		results = append(results, decisionlog.DispatchResult{Employee: emp, OK: true})
	}
	return results
}

func jetStreamContext(cfg config.Config) (nats.JetStreamContext, error) {
	want := strings.TrimSpace(cfg.NatsURL)
	if want == "" {
		return nil, fmt.Errorf("empty NATS url")
	}

	jsMu.Lock()
	defer jsMu.Unlock()

	if jsConn != nil && jsConn.IsConnected() && jsURLUsed == want && jsCtx != nil {
		return jsCtx, nil
	}
	if jsConn != nil {
		_ = jsConn.Drain()
		jsConn = nil
		jsCtx = nil
	}

	nc, err := nats.Connect(want,
		nats.Name("slack-orchestrator"),
		nats.Timeout(20*time.Second),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return nil, err
	}
	jet, err := nc.JetStream()
	if err != nil {
		_ = nc.Drain()
		return nil, err
	}
	jsConn = nc
	jsCtx = jet
	jsURLUsed = want
	return jsCtx, nil
}

func ensureStream(js nats.JetStreamContext, name string) error {
	if _, err := js.StreamInfo(name); err == nil {
		return nil
	} else if !errors.Is(err, nats.ErrStreamNotFound) {
		return fmt.Errorf("stream info %q: %w", name, err)
	}
	_, err := js.AddStream(&nats.StreamConfig{
		Name:     name,
		Subjects: []string{"slack.work.*.events"},
		Storage:  nats.FileStorage,
	})
	if err != nil {
		return fmt.Errorf("add stream %q: %w", name, err)
	}
	return nil
}

func callbackMeta(ev slackevents.EventsAPIEvent) (eventID string, eventTime int, teamID, apiAppID string) {
	if cb, ok := ev.Data.(*slackevents.EventsAPICallbackEvent); ok && cb != nil {
		return cb.EventID, cb.EventTime, cb.TeamID, cb.APIAppID
	}
	return "", 0, strings.TrimSpace(ev.TeamID), strings.TrimSpace(ev.APIAppID)
}
