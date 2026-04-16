package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds runtime settings loaded from the environment.
type Config struct {
	HTTPAddr         string
	BotToken         string
	AppToken         string
	ShuffleSecret    string
	MultiagentOrder  []string
	BotUserToKey     map[string]string // Slack bot user ID -> employee key (alex, tim, …)
	EveryoneLimit    int
	ChannelLimit     int
	LogJSON          bool
}

const (
	defaultHTTPAddr      = ":8080"
	defaultEveryoneLimit = 5
	defaultChannelLimit  = 3
)

// FromEnv loads configuration. Missing SLACK_BOT_TOKEN / SLACK_APP_TOKEN is allowed for routing-only tests.
func FromEnv() Config {
	order := splitCSV(os.Getenv("MULTIAGENT_ORDER"))
	if len(order) == 0 {
		order = []string{"alex", "tim", "ross", "garth", "joanne"}
	}
	cfg := Config{
		HTTPAddr:        strings.TrimSpace(os.Getenv("HTTP_ADDR")),
		BotToken:        strings.TrimSpace(os.Getenv("SLACK_BOT_TOKEN")),
		AppToken:        strings.TrimSpace(os.Getenv("SLACK_APP_TOKEN")),
		ShuffleSecret:   strings.TrimSpace(os.Getenv("MULTIAGENT_SHUFFLE_SECRET")),
		MultiagentOrder: order,
		BotUserToKey:    parseBotUserMap(os.Getenv("MULTIAGENT_BOT_USER_IDS")),
		EveryoneLimit:   getenvInt("EVERYONE_AGENT_LIMIT", defaultEveryoneLimit),
		ChannelLimit:    getenvInt("CHANNEL_AGENT_LIMIT", defaultChannelLimit),
		LogJSON:         strings.EqualFold(strings.TrimSpace(os.Getenv("LOG_JSON")), "1") ||
			strings.EqualFold(strings.TrimSpace(os.Getenv("LOG_JSON")), "true"),
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = defaultHTTPAddr
	}
	if cfg.ShuffleSecret == "" {
		cfg.ShuffleSecret = "dev-insecure-change-me"
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

// MULTIAGENT_BOT_USER_IDS format: alex=Uxxx,tim=Uyyy,...
func parseBotUserMap(s string) map[string]string {
	out := make(map[string]string)
	s = strings.TrimSpace(s)
	if s == "" {
		return out
	}
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
