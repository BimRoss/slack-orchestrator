package slackrun

import (
	"context"
)

// ThreadRootTextFetcher loads the parent message text for a Slack thread (conversations.replies limit=1).
// Nil means thread-root resolution is disabled (routing falls back to hashed plain responder).
var threadRootTextFetcher func(ctx context.Context, channelID, threadTS string) (string, error)

// SetThreadRootTextFetcher wires Slack Web API access from main (bot token). Pass nil to disable.
func SetThreadRootTextFetcher(f func(ctx context.Context, channelID, threadTS string) (string, error)) {
	threadRootTextFetcher = f
}
