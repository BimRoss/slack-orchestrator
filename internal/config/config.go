package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds runtime settings loaded from the environment.
type Config struct {
	HTTPAddr        string
	BotToken        string
	AppToken        string
	ShuffleSecret   string
	MultiagentOrder []string
	BotUserToKey    map[string]string // Slack bot user ID -> employee key (alex, tim, …)
	EveryoneLimit   int
	ChannelLimit    int
	LogJSON         bool

	// DispatchEnabled publishes routing decisions to NATS JetStream (per-employee subjects).
	DispatchEnabled bool
	NatsURL         string
	NatsStream      string

	// DebugToken enables GET /debug/decisions (Bearer). Empty = endpoint disabled unless DebugAllowAnon.
	DebugToken string
	// DebugAllowAnon allows GET /debug/decisions without Authorization (operator convenience; lock down later).
	DebugAllowAnon bool
	// DecisionLogMax is the max in-memory decision entries (ring via slice trim).
	DecisionLogMax int
}

const (
	defaultHTTPAddr      = ":8080"
	defaultEveryoneLimit = 5
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
		HTTPAddr:        strings.TrimSpace(os.Getenv("HTTP_ADDR")),
		BotToken:        strings.TrimSpace(os.Getenv("SLACK_BOT_TOKEN")),
		AppToken:        strings.TrimSpace(os.Getenv("SLACK_APP_TOKEN")),
		ShuffleSecret:   shuffle,
		MultiagentOrder: order,
		BotUserToKey:    botMap,
		EveryoneLimit:   getenvInt("EVERYONE_AGENT_LIMIT", defaultEveryoneLimit),
		ChannelLimit:    getenvInt("CHANNEL_AGENT_LIMIT", defaultChannelLimit),
		LogJSON:         logJSONDefaultTrue(os.Getenv("LOG_JSON")),

		DispatchEnabled: parseBoolEnv("ORCHESTRATOR_DISPATCH_ENABLED", false),
		NatsURL:         strings.TrimSpace(os.Getenv("ORCHESTRATOR_NATS_URL")),
		NatsStream:      strings.TrimSpace(os.Getenv("ORCHESTRATOR_NATS_STREAM")),

		DebugToken:     strings.TrimSpace(os.Getenv("ORCHESTRATOR_DEBUG_TOKEN")),
		DebugAllowAnon: parseBoolEnv("ORCHESTRATOR_DEBUG_ALLOW_ANON", false),
		DecisionLogMax: getenvInt("ORCHESTRATOR_DECISION_LOG_MAX", defaultDecisionLogMax),
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = defaultHTTPAddr
	}
	if cfg.NatsStream == "" {
		cfg.NatsStream = "SLACK_WORK"
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

// logJSONDefaultTrue: JSON structured logs on by default; set LOG_JSON=false, 0, no, or off to disable.
func logJSONDefaultTrue(raw string) bool {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return true
	}
	switch s {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
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
