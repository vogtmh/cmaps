package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
)

// requestIDHeader is the HTTP header carrying a correlation ID for a request.
// It is honored from the incoming request (e.g. set by the nginx reverse
// proxy) or generated when absent, and echoed back on the response so a log
// line can be tied to a specific request across the proxy and the app.
const requestIDHeader = "X-Request-ID"

type ctxKey int

const requestIDKey ctxKey = 0

// requestIDMiddleware ensures every request carries a correlation ID, stored
// in the request context and echoed in the response header.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set(requestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// newRequestID returns a short random hex correlation ID.
func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b[:])
}

// requestID returns the correlation ID stored on the request context, if any.
func requestID(r *http.Request) string {
	if id, ok := r.Context().Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// reqLogger returns a request-scoped structured logger tagged with the
// correlation ID and client IP, so handler log lines can be traced back to a
// single request.
func (app *Server) reqLogger(r *http.Request) *slog.Logger {
	return slog.Default().With("req_id", requestID(r), "ip", clientIP(r))
}
