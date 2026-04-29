// Package catalogdebug serves GET /debug/capability-catalog — the same JSON embedded on NATS dispatch.
package catalogdebug

import (
	"net/http"
	"strings"

	"github.com/bimross/slack-orchestrator/internal/inbound"
)

func writeCatalogResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(inbound.DefaultCapabilityContractJSON())
}

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
		writeCatalogResponse(w)
	}
}

// PublicHTTPHandler serves GET /v1/public/capability-catalog without debug auth requirements.
// This is the stable in-cluster source-of-truth endpoint for catalog consumers.
func PublicHTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeCatalogResponse(w)
	}
}
