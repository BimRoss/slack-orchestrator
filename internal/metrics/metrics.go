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
