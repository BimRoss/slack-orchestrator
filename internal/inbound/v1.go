// Package inbound defines the JSON contract from slack-orchestrator to employee-factory workers.
//
// The default capability contract (see capability_contract.go) is defined and versioned here as the
// source of truth for what JetStream publishes; workers apply the inlined JSON per message.
package inbound

import (
	"encoding/json"

	"github.com/bimross/slack-orchestrator/internal/routing"
)

// SchemaVersion 3 adds capabilities (capability contract JSON on every dispatch).
// Schema 2: dispatch_mode and primary_employee on routing.Decision (single-target plain thread).
const SchemaVersion = "3"

// EventV1 is published to JetStream per target employee (subject slack.work.<employee>.events).
type EventV1 struct {
	SchemaVersion string `json:"schema_version"`

	TraceID string `json:"trace_id,omitempty"`

	SlackEventID   string `json:"slack_event_id,omitempty"`
	SlackEventTime int    `json:"slack_event_time,omitempty"`
	TeamID         string `json:"team_id,omitempty"`
	APIAppID       string `json:"api_app_id,omitempty"`

	// InnerType is the Slack inner event type (e.g. message, app_mention).
	InnerType string `json:"inner_type"`

	TargetEmployee string           `json:"target_employee"`
	Decision       routing.Decision `json:"decision"`

	Message MessageV1 `json:"message"`

	// Capabilities is the full runtime capability catalog (JSON). Workers use this instead of
	// fetching makeacompany when present (schema_version 3+).
	Capabilities json.RawMessage `json:"capabilities,omitempty"`
}

// MessageV1 is normalized text-bearing payload for message / app_mention paths.
type MessageV1 struct {
	ChannelID string `json:"channel_id"`
	ThreadTS  string `json:"thread_ts"`
	MessageTS string `json:"message_ts"`
	UserID    string `json:"user_id"`
	Text      string `json:"text"`
}
