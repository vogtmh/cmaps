package web

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

// writeJSON writes v as a JSON response.
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// respondError writes the canonical JSON error body {"ok": false, "message":
// "..."} at HTTP 200, matching the dominant convention every REST/JSON
// endpoint and the front-end's d.ok / d.message checks already use. Transport
// failures (auth, routing) keep their HTTP status via http.Error; this helper
// is for business-logic errors surfaced inline in the UI.
func respondError(w http.ResponseWriter, msg string) {
	writeJSON(w, map[string]interface{}{"ok": false, "message": msg})
}

// clientIP returns the best-effort client IP, honoring X-Forwarded-For for use
// behind the nginx reverse proxy.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
