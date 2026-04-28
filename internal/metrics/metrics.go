// Package metrics registers Prometheus metrics for slack-orchestrator.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// SocketModeState values for slack_orchestrator_socket_mode_state (single gauge).
const (
	SocketStateDisconnected    = 0
	SocketStateConnecting      = 1
	SocketStateConnected       = 2
	SocketStateConnectionError = 3
)

var (
	// SocketModeState is 0=disconnected/initial, 1=connecting, 2=connected, 3=connection_error.
	SocketModeState = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "slack_orchestrator",
		Name:      "socket_mode_state",
		Help:      "Socket Mode connection state: 0=disconnected/unknown, 1=connecting, 2=connected, 3=connection_error",
	})

	// EventsAPIAckedTotal counts Events API envelopes acknowledged (client.Ack) after receipt.
	EventsAPIAckedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "slack_orchestrator",
		Name:      "events_api_acked_total",
		Help:      "Total Events API Socket Mode envelopes acknowledged",
	})

	// EventsAPINilRequestTotal counts Events API envelopes where Socket Mode provided no ack request (cannot ack).
	EventsAPINilRequestTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "slack_orchestrator",
		Name:      "events_api_nil_request_total",
		Help:      "Total Events API events dropped because evt.Request was nil (cannot Ack)",
	})

	// EventsAPIHandleSeconds observes slackrun.HandleEventsAPI latency after ack (per envelope).
	EventsAPIHandleSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "slack_orchestrator",
		Name:      "events_api_handle_seconds",
		Help:      "Wall time spent in HandleEventsAPI per Events API envelope (after Ack)",
		Buckets:   []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 15, 30},
	})

	// EventsAPIQueuedTotal counts Events API envelopes enqueued for async routing/dispatch workers.
	EventsAPIQueuedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "slack_orchestrator",
		Name:      "events_api_queued_total",
		Help:      "Total Events API envelopes accepted into the async processing queue",
	})

	// EventsAPIQueueDepth is the current buffered queue depth for async Events API processing.
	EventsAPIQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "slack_orchestrator",
		Name:      "events_api_queue_depth",
		Help:      "Current queued Events API envelopes waiting for async processing",
	})

	// SocketModeBadMessageTotal counts WebSocket payloads that failed to parse (see logs for cause).
	SocketModeBadMessageTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "slack_orchestrator",
		Name:      "socket_mode_bad_message_total",
		Help:      "Total Socket Mode messages that failed parsing before events_api dispatch",
	})

	// DelegatePublishTotal counts JetStream publish attempts to worker runtimes. Labels: result=success|failure|skipped.
	DelegatePublishTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "slack_orchestrator",
		Name:      "delegate_publish_total",
		Help:      "Total JetStream publish deliveries to employee-factory workers",
	}, []string{"result"})

	// DelegatePublishErrorsTotal counts publish errors after failures.
	DelegatePublishErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "slack_orchestrator",
		Name:      "delegate_publish_errors_total",
		Help:      "Total JetStream publishes that failed",
	})

	// DelegatePublishRetriesTotal counts retry attempts after a failed JetStream publish (not the first try).
	DelegatePublishRetriesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "slack_orchestrator",
		Name:      "delegate_publish_retries_total",
		Help:      "Total extra JetStream publish attempts after a transient failure (bounded backoff)",
	})

	// DelegatePublishSeconds observes JetStream publish latency when dispatch is enabled.
	DelegatePublishSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "slack_orchestrator",
		Name:      "delegate_publish_seconds",
		Help:      "Latency of JetStream publishes to worker runtimes",
		Buckets:   prometheus.DefBuckets,
	}, []string{"result"})
)

func init() {
	SocketModeState.Set(SocketStateDisconnected)
}
