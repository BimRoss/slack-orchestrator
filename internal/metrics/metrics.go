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

	// DelegatePostTotal counts outbound delegate HTTP attempts (Phase 2+). Labels: result=success|failure|skipped.
	DelegatePostTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "slack_orchestrator",
		Name:      "delegate_post_total",
		Help:      "Total delegate HTTP delivery attempts to worker runtimes",
	}, []string{"result"})

	// DelegatePostErrorsTotal counts delegate HTTP errors after retries or non-retryable failures.
	DelegatePostErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "slack_orchestrator",
		Name:      "delegate_post_errors_total",
		Help:      "Total delegate HTTP deliveries that failed (non-success path)",
	})

	// DelegateHTTPRequestSeconds observes delegate HTTP request duration when dispatch is enabled.
	DelegateHTTPRequestSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "slack_orchestrator",
		Name:      "delegate_http_request_seconds",
		Help:      "Latency of delegate HTTP POSTs to worker runtimes",
		Buckets:   prometheus.DefBuckets,
	}, []string{"result"})
)

func init() {
	SocketModeState.Set(SocketStateDisconnected)
}
