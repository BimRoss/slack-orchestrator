package routing

import (
	"regexp"
	"strings"
)

// BroadcastTrigger classifies @everyone / @channel style roots.
type BroadcastTrigger int

const (
	BroadcastNone BroadcastTrigger = iota
	BroadcastEveryone
	BroadcastChannel
)

var (
	reEveryoneAlias = regexp.MustCompile(`(?i)(?:^|\s)@everyone\b`)
	reChannelAlias  = regexp.MustCompile(`(?i)(?:^|\s)@channel\b`)
	reHereAlias     = regexp.MustCompile(`(?i)(?:^|\s)@here\b`)
)

// ClassifyBroadcastTrigger mirrors employee-factory broadcast detection: <!everyone>, <!channel>,
// <!here>, and plain @everyone / @channel / @here. @here uses the same limits and fan-out as @everyone.
func ClassifyBroadcastTrigger(rawText string) BroadcastTrigger {
	lower := strings.ToLower(rawText)
	if strings.Contains(lower, "<!everyone") || reEveryoneAlias.MatchString(rawText) ||
		strings.Contains(lower, "<!here") || reHereAlias.MatchString(rawText) {
		return BroadcastEveryone
	}
	if strings.Contains(lower, "<!channel") || reChannelAlias.MatchString(rawText) {
		return BroadcastChannel
	}
	return BroadcastNone
}
