package config

import (
	"os"
	"strconv"
	"strings"
	"time"
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

	// DispatchEnabled POSTs routing decisions to per-employee worker URLs (Phase 2).
	DispatchEnabled     bool
	WorkerURLTemplate   string
	WorkerHMACSecret    string
	DispatchHTTPTimeout time.Duration
}

const (
	defaultHTTPAddr           = ":8080"
	defaultEveryoneLimit      = 5
	defaultChannelLimit       = 3
	defaultDispatchTimeoutSec = 10
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
	dispatchTimeoutSec := getenvInt("ORCHESTRATOR_DISPATCH_TIMEOUT_SEC", defaultDispatchTimeoutSec)
	if dispatchTimeoutSec < 1 {
		dispatchTimeoutSec = defaultDispatchTimeoutSec
	}
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

		DispatchEnabled:     parseBoolEnv("ORCHESTRATOR_DISPATCH_ENABLED", false),
		WorkerURLTemplate:   strings.TrimSpace(os.Getenv("ORCHESTRATOR_WORKER_URL_TEMPLATE")),
		WorkerHMACSecret:    strings.TrimSpace(os.Getenv("ORCHESTRATOR_WORKER_HMAC_SECRET")),
		DispatchHTTPTimeout: time.Duration(dispatchTimeoutSec) * time.Second,
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = defaultHTTPAddr
	}
	return cfg
}

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
