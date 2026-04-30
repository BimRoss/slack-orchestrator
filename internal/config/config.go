package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds runtime settings loaded from the environment.
type Config struct {
	HTTPAddr           string
	BotToken           string
	AppToken           string
	SocketModeDebug    bool
	SocketPingSec      int
	EventsAPIWorkers   int
	EventsAPIQueueSize int
	ShuffleSecret      string
	MultiagentOrder    []string
	BotUserToKey       map[string]string // Slack bot user ID -> employee key (alex, tim, …)
	EveryoneLimit      int
	ChannelLimit       int

	// DispatchEnabled publishes routing decisions to NATS JetStream (per-employee subjects).
	DispatchEnabled bool
	NatsURL         string
	NatsStream      string
	// DispatchPublishMaxAttempts is JetStream publish tries per message (1 = no retry). Clamped 1–10 in FromEnv.
	DispatchPublishMaxAttempts int
	// DispatchPublishRetryBaseMS is the base backoff in milliseconds before the first retry (exponential: base, 2*base, …, cap 2s). Clamped 0–5000 in FromEnv.
	DispatchPublishRetryBaseMS int

	// TermsRedisURL is optional. When set, human message and app_mention routing requires non-empty
	// humans_terms_accepted_at on makeacompany:user_profile (same Redis keys as /admin Slack Users).
	TermsRedisURL string

	// DebugToken enables GET /debug/decisions (Bearer). Empty = endpoint disabled unless DebugAllowAnon.
	DebugToken string
	// DebugAllowAnon allows GET /debug/decisions without Authorization (operator convenience; lock down later).
	DebugAllowAnon bool
	// DecisionLogMax is the max in-memory decision entries (ring via slice trim).
	DecisionLogMax int
}

const (
	defaultHTTPAddr      = ":8080"
	defaultEveryoneLimit = 3
	defaultChannelLimit  = 3
)

// FromEnv loads configuration. Missing SLACK_BOT_TOKEN / SLACK_APP_TOKEN is allowed for routing-only tests.
func FromEnv() Config {
	botMap := parseBotUserMap(os.Getenv("MULTIAGENT_BOT_USER_IDS"))
	shuffle := strings.TrimSpace(os.Getenv("MULTIAGENT_SHUFFLE_SECRET"))
	if shuffle == "" {
		shuffle = DerivedShuffleSeed(botMap)
	}
	explicitOrder := splitCSV(os.Getenv("MULTIAGENT_ORDER"))
	order := ResolveMultiagentOrder(explicitOrder, botMap, shuffle)
	cfg := Config{
		HTTPAddr: strings.TrimSpace(os.Getenv("HTTP_ADDR")),
		// Shared .env.dev with workers: allow orchestrator-only tokens so SLACK_BOT_TOKEN is not overloaded.
		BotToken: strings.TrimSpace(firstNonEmpty(
			os.Getenv("SLACK_BOT_TOKEN"),
			os.Getenv("ORCHESTRATOR_SLACK_BOT_TOKEN"),
		)),
		AppToken: strings.TrimSpace(firstNonEmpty(
			os.Getenv("SLACK_APP_TOKEN"),
			os.Getenv("ORCHESTRATOR_SLACK_APP_TOKEN"),
		)),
		SocketModeDebug:    parseBoolEnv("SOCKET_MODE_DEBUG", false),
		SocketPingSec:      getenvInt("SOCKET_MODE_PING_INTERVAL_SEC", 0),
		EventsAPIWorkers:   getenvInt("ORCHESTRATOR_EVENTS_API_WORKERS", 8),
		EventsAPIQueueSize: getenvInt("ORCHESTRATOR_EVENTS_API_QUEUE_SIZE", 256),
		ShuffleSecret:      shuffle,
		MultiagentOrder:    order,
		BotUserToKey:       botMap,
		EveryoneLimit:      getenvInt("EVERYONE_AGENT_LIMIT", defaultEveryoneLimit),
		ChannelLimit:       getenvInt("CHANNEL_AGENT_LIMIT", defaultChannelLimit),

		DispatchEnabled: parseBoolEnv("ORCHESTRATOR_DISPATCH_ENABLED", false),
		NatsURL:         strings.TrimSpace(os.Getenv("ORCHESTRATOR_NATS_URL")),
		NatsStream:      strings.TrimSpace(os.Getenv("ORCHESTRATOR_NATS_STREAM")),
		TermsRedisURL:   strings.TrimSpace(os.Getenv("ORCHESTRATOR_TERMS_REDIS_URL")),

		DispatchPublishMaxAttempts: getenvInt("ORCHESTRATOR_DISPATCH_PUBLISH_MAX_ATTEMPTS", 3),
		DispatchPublishRetryBaseMS: getenvInt("ORCHESTRATOR_DISPATCH_PUBLISH_RETRY_BASE_MS", 50),

		DebugToken:     strings.TrimSpace(os.Getenv("ORCHESTRATOR_DEBUG_TOKEN")),
		DebugAllowAnon: parseBoolEnv("ORCHESTRATOR_DEBUG_ALLOW_ANON", true),
		DecisionLogMax: getenvInt("ORCHESTRATOR_DECISION_LOG_MAX", defaultDecisionLogMax),
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = defaultHTTPAddr
	}
	if cfg.NatsStream == "" {
		cfg.NatsStream = "SLACK_WORK"
	}
	if cfg.DispatchPublishMaxAttempts < 1 {
		cfg.DispatchPublishMaxAttempts = 1
	}
	if cfg.DispatchPublishMaxAttempts > 10 {
		cfg.DispatchPublishMaxAttempts = 10
	}
	if cfg.DispatchPublishRetryBaseMS < 0 {
		cfg.DispatchPublishRetryBaseMS = 0
	}
	if cfg.DispatchPublishRetryBaseMS > 5000 {
		cfg.DispatchPublishRetryBaseMS = 5000
	}
	if cfg.EventsAPIWorkers < 1 {
		cfg.EventsAPIWorkers = 1
	}
	if cfg.EventsAPIQueueSize < 1 {
		cfg.EventsAPIQueueSize = 1
	}
	return cfg
}

const defaultDecisionLogMax = 500

func splitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Default squad order when MULTIAGENT_BOT_USER_IDS is a comma-separated list of Slack IDs without "name=".
var defaultSquadOrder = []string{"alex", "tim", "ross", "garth", "joanne"}

// MULTIAGENT_BOT_USER_IDS: either alex=Uxxx,tim=Uyyy,... or a comma-separated list of Slack user IDs
// in default squad order (alex → … → joanne), same length convention as employee-factory.
func parseBotUserMap(s string) map[string]string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if !strings.Contains(s, "=") {
		return parseBotUserMapPositional(s)
	}
	out := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		uid := strings.TrimSpace(kv[1])
		if key != "" && uid != "" {
			out[uid] = key
		}
	}
	return out
}

func parseBotUserMapPositional(s string) map[string]string {
	out := make(map[string]string)
	i := 0
	for _, pair := range strings.Split(s, ",") {
		uid := strings.TrimSpace(pair)
		if uid == "" {
			continue
		}
		if i >= len(defaultSquadOrder) {
			break
		}
		out[uid] = defaultSquadOrder[i]
		i++
	}
	return out
}

func parseBoolEnv(key string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func getenvInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func firstNonEmpty(vals ...string) string {
	for _, s := range vals {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
