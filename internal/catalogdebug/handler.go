// Package catalogdebug serves GET /debug/capability-catalog — the same JSON embedded on NATS dispatch.
package catalogdebug

import (
	"net/http"
	"strings"

	"github.com/bimross/slack-orchestrator/internal/inbound"
)

// HTTPHandler serves GET /debug/capability-catalog. Auth matches decisionlog (/debug/decisions).
func HTTPHandler(token string, allowAnon bool) http.HandlerFunc {
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
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(inbound.DefaultCapabilityContractJSON())
	}
}
