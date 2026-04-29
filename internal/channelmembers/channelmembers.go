// Package channelmembers serves human Slack user IDs in a channel (conversations.members +
// users.info, excluding bots).
package channelmembers

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

const maxMemberIDs = 2000
const maxHumanUserIDs = 200

func serveChannelMembers(w http.ResponseWriter, r *http.Request, botToken string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
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
	chID := strings.TrimSpace(r.URL.Query().Get("channel_id"))
	if chID == "" {
		http.Error(w, "missing channel_id", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	api := slack.New(botToken)

	var all []string
	cursor := ""
	for {
		members, next, err := api.GetUsersInConversationContext(ctx, &slack.GetUsersInConversationParameters{
			ChannelID: chID,
			Cursor:    cursor,
			Limit:     200,
		})
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":   "slack_api_error",
				"message": err.Error(),
			})
			return
		}
		for _, uid := range members {
			uid = strings.TrimSpace(uid)
			if uid == "" {
				continue
			}
			all = append(all, uid)
			if len(all) >= maxMemberIDs {
				break
			}
		}
		if len(all) >= maxMemberIDs {
			break
		}
		if strings.TrimSpace(next) == "" {
			break
		}
		cursor = next
	}

	truncatedMembers := len(all) >= maxMemberIDs
	human := filterHumanUsers(ctx, api, all)
	sort.Strings(human)
	truncatedHumans := len(human) > maxHumanUserIDs
	if len(human) > maxHumanUserIDs {
		human = human[:maxHumanUserIDs]
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"schema_version":      1,
		"channel_id":          chID,
		"human_user_ids":      human,
		"truncated_members":   truncatedMembers,
		"truncated_human_ids": truncatedHumans,
	})
}

// HTTPHandler serves GET /debug/channel-members?channel_id=C.... Auth matches member-channels.
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
		serveChannelMembers(w, r, botToken)
	}
}

// PublicHTTPHandler serves GET /v1/public/channel-members?channel_id=... without debug auth checks.
func PublicHTTPHandler(botToken string) http.HandlerFunc {
	botToken = strings.TrimSpace(botToken)
	return func(w http.ResponseWriter, r *http.Request) {
		serveChannelMembers(w, r, botToken)
	}
}

func filterHumanUsers(ctx context.Context, api *slack.Client, ids []string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, uid := range ids {
		uid = strings.TrimSpace(uid)
		if uid == "" {
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		u, err := api.GetUserInfoContext(ctx, uid)
		if err != nil {
			continue
		}
		if u == nil {
			continue
		}
		if u.IsBot || u.Deleted {
			continue
		}
		out = append(out, u.ID)
	}
	return out
}
