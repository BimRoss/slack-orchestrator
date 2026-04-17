package slackrun

import "context"

// ThreadRoutingFetcher loads thread replies and derives ThreadPlainHandoffKey (last squad @mention
// before the current message). Nil disables enrichment (plain thread routing uses the hashed picker).
var threadRoutingFetcher func(ctx context.Context, channelID, threadTS, currentMessageTS string) (handoffKey string, err error)

// SetThreadRoutingFetcher wires Slack Web API access from main (bot token). Pass nil to disable.
func SetThreadRoutingFetcher(f func(ctx context.Context, channelID, threadTS, currentMessageTS string) (string, error)) {
	threadRoutingFetcher = f
}
