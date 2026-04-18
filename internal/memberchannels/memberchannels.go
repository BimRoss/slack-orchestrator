// Package memberchannels serves GET /debug/member-channels — Slack channels the orchestrator bot is in
// (users.conversations), matching channel-knowledge discovery.
package memberchannels

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

const maxChannels = 500

// HTTPHandler serves GET /debug/member-channels. Auth matches decisionlog (/debug/decisions).
func HTTPHandler(botToken string, token string, allowAnon bool) http.HandlerFunc {
	botToken = strings.TrimSpace(botToken)
	token = strings.TrimSpace(token)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !allowAnon {
			if token == "" {
				http.Error(w, "debug endpoint disabled", http.StatusServiceUnavailable)
				return
			}
			auth := strings.TrimSpace(r.Header.Get("Authorization"))
			const p = "Bearer "
			if !strings.HasPrefix(auth, p) || strings.TrimSpace(auth[len(p):]) != token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		if botToken == "" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":   "slack_bot_token_missing",
				"message": "Set SLACK_BOT_TOKEN (or ORCHESTRATOR_SLACK_BOT_TOKEN) on slack-orchestrator.",
			})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
		defer cancel()
		api := slack.New(botToken)
		channels, truncated, err := listMemberChannels(ctx, api)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":   "slack_api_error",
				"message": err.Error(),
			})
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"schema_version": 1,
			"channels":       channels,
			"truncated":      truncated,
		})
	}
}

type row struct {
	ChannelID string `json:"channel_id"`
	Name      string `json:"name"`
	IsPrivate bool   `json:"is_private"`
}

func listMemberChannels(ctx context.Context, api *slack.Client) ([]row, bool, error) {
	var out []row
	seen := make(map[string]struct{})
	cursor := ""
	for {
		ch, next, err := api.GetConversationsForUserContext(ctx, &slack.GetConversationsForUserParameters{
			Cursor:          cursor,
			Types:           []string{"public_channel", "private_channel"},
			Limit:           200,
			ExcludeArchived: true,
		})
		if err != nil {
			return nil, false, err
		}
		for _, c := range ch {
			id := strings.TrimSpace(c.ID)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			name := strings.TrimSpace(c.Name)
			if name == "" {
				name = id
			}
			out = append(out, row{
				ChannelID: id,
				Name:      name,
				IsPrivate: c.IsPrivate,
			})
			if len(out) >= maxChannels {
				sortRows(out)
				return out, true, nil
			}
		}
		if strings.TrimSpace(next) == "" {
			break
		}
		cursor = next
	}
	sortRows(out)
	return out, false, nil
}

func sortRows(rows []row) {
	sort.SliceStable(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
	})
}
