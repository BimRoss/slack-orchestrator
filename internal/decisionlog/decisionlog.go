// Package decisionlog holds a bounded in-memory log of routing + dispatch outcomes for debugging.
package decisionlog

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bimross/slack-orchestrator/internal/routing"
)

const defaultMax = 500

// DispatchResult is one worker POST outcome.
type DispatchResult struct {
	Employee   string `json:"employee"`
	OK         bool   `json:"ok"`
	HTTPStatus int    `json:"http_status,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Entry is one orchestrator decision (and optional dispatch results).
type Entry struct {
	Time            time.Time        `json:"time"`
	InnerType       string           `json:"inner_type"`
	ChannelID       string           `json:"channel_id"`
	ThreadTS        string           `json:"thread_ts"`
	MessageTS       string           `json:"message_ts"`
	UserID          string           `json:"user_id"`
	TextPreview     string           `json:"text_preview"`
	Decision        routing.Decision `json:"decision"`
	DispatchNote    string           `json:"dispatch_note,omitempty"`
	DispatchResults []DispatchResult `json:"dispatch_results,omitempty"`
}

// Store is a bounded in-memory log (oldest → newest).
type Store struct {
	mu      sync.Mutex
	max     int
	entries []Entry
}

// New returns a Store keeping at most maxN entries (clamped).
func New(maxN int) *Store {
	if maxN < 10 {
		maxN = defaultMax
	}
	if maxN > 5000 {
		maxN = 5000
	}
	return &Store{max: maxN}
}

// Append adds an entry.
func (s *Store) Append(e Entry) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e.Time = e.Time.UTC()
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	e.TextPreview = truncateRunes(e.TextPreview, 400)
	s.entries = append(s.entries, e)
	if len(s.entries) > s.max {
		drop := len(s.entries) - s.max
		s.entries = s.entries[drop:]
	}
}

// Snapshot returns the newest `limit` entries, oldest first.
func (s *Store) Snapshot(limit int) []Entry {
	if s == nil || limit <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.entries)
	if n == 0 {
		return nil
	}
	if limit > n {
		limit = n
	}
	if limit > s.max {
		limit = s.max
	}
	out := make([]Entry, limit)
	copy(out, s.entries[n-limit:])
	return out
}

func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	if len(runes) > max {
		runes = runes[:max]
	}
	return string(runes) + "…"
}

// HTTPHandler serves GET /debug/decisions?limit=100.
// If allowAnon is false, requires Authorization: Bearer <token> and token must be non-empty.
// If allowAnon is true, no auth (use only on trusted networks; prefer token in production).
func HTTPHandler(store *Store, token string, allowAnon bool) http.HandlerFunc {
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
		limit := 100
		if q := strings.TrimSpace(r.URL.Query().Get("limit")); q != "" {
			if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 500 {
				limit = n
			}
		}
		entries := store.Snapshot(limit)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"schema_version": 2,
			"entries":        entries,
		})
	}
}
