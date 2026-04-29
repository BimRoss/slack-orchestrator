// Package catalogdebug serves GET /debug/capability-catalog — the same JSON embedded on NATS dispatch.
package catalogdebug

import (
	"net/http"

	"github.com/bimross/slack-orchestrator/internal/inbound"
)

func writeCatalogResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(inbound.DefaultCapabilityContractJSON())
}

// PublicHTTPHandler serves GET /v1/public/capability-catalog without debug auth requirements.
// This is the stable source-of-truth endpoint for catalog consumers.
func PublicHTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeCatalogResponse(w)
	}
}
