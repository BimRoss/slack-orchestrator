// Package inbound defines the JSON contract from slack-orchestrator to employee-factory workers.
package inbound

import "github.com/bimross/slack-orchestrator/internal/routing"

const SchemaVersion = "1"

// EventV1 is POSTed to each target worker (one HTTP request per employee in the routing decision).
type EventV1 struct {
	SchemaVersion string `json:"schema_version"`

	SlackEventID   string `json:"slack_event_id,omitempty"`
	SlackEventTime int    `json:"slack_event_time,omitempty"`
	TeamID         string `json:"team_id,omitempty"`
	APIAppID       string `json:"api_app_id,omitempty"`

	// InnerType is the Slack inner event type (e.g. message, app_mention).
	InnerType string `json:"inner_type"`

	TargetEmployee string           `json:"target_employee"`
	Decision       routing.Decision `json:"decision"`

	Message MessageV1 `json:"message"`
}

// MessageV1 is normalized text-bearing payload for message / app_mention paths.
type MessageV1 struct {
	ChannelID string `json:"channel_id"`
	ThreadTS  string `json:"thread_ts"`
	MessageTS string `json:"message_ts"`
	UserID    string `json:"user_id"`
	Text      string `json:"text"`
}
