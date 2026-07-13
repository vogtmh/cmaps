package web

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// gzipPool reuses gzip.Writer instances across requests to avoid per-request
// allocations.
var gzipPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

// gzipResponseWriter buffers the response just long enough to inspect the
// Content-Type on the first write, then transparently compresses the body when
// the content is a compressible (text-based) type. Already-compressed payloads
// such as images pass through untouched.
type gzipResponseWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	wroteHeader bool
	compress    bool
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true

	// Decide whether to compress based on the content type. Skip responses that
	// are already compressed or that explicitly set their own encoding.
	if w.Header().Get("Content-Encoding") == "" && isCompressibleType(w.Header().Get("Content-Type")) {
		w.compress = true
		w.Header().Del("Content-Length")
		w.Header().Set("Content-Encoding", "gzip")
		w.gz = gzipPool.Get().(*gzip.Writer)
		w.gz.Reset(w.ResponseWriter)
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		// If the handler never set a Content-Type, sniff it from the body so the
		// compress decision matches what the client will receive.
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", http.DetectContentType(b))
		}
		w.WriteHeader(http.StatusOK)
	}
	if w.compress {
		return w.gz.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

// Flush implements http.Flusher so streaming handlers keep working.
func (w *gzipResponseWriter) Flush() {
	if w.compress && w.gz != nil {
		_ = w.gz.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *gzipResponseWriter) close() {
	if w.compress && w.gz != nil {
		_ = w.gz.Close()
		gzipPool.Put(w.gz)
		w.gz = nil
	}
}

// isCompressibleType reports whether a Content-Type is worth gzipping. Binary
// media (images, fonts, archives) is already compressed and is skipped.
func isCompressibleType(ct string) bool {
	if ct == "" {
		return false
	}
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	ct = strings.TrimSpace(strings.ToLower(ct))
	switch {
	case strings.HasPrefix(ct, "text/"):
		return true
	case ct == "application/json",
		ct == "application/javascript",
		ct == "application/x-javascript",
		ct == "application/xml",
		ct == "application/xhtml+xml",
		ct == "image/svg+xml",
		ct == "application/manifest+json":
		return true
	}
	return false
}

// gzipMiddleware compresses text-based responses for clients that advertise
// gzip support via Accept-Encoding. Non-compressible content (images, fonts)
// passes through untouched.
func gzipMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			h.ServeHTTP(w, r)
			return
		}
		w.Header().Add("Vary", "Accept-Encoding")
		gw := &gzipResponseWriter{ResponseWriter: w}
		defer gw.close()
		h.ServeHTTP(gw, r)
	})
}
