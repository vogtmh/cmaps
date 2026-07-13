package web

import (
	"net/http"
	"strconv"
)

// handleRestDBBuckets serves /rest/db/buckets: the list of buckets with counts.
func (app *Server) handleRestDBBuckets(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "adminpanel") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	buckets, err := app.db.BrowseBuckets()
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"buckets": buckets})
}

// handleRestDBEntries serves /rest/db/entries?bucket=&search=&offset=&limit=.
func (app *Server) handleRestDBEntries(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "adminpanel") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	q := r.URL.Query()
	bucket := q.Get("bucket")
	if bucket == "" {
		http.Error(w, "bucket required", http.StatusBadRequest)
		return
	}
	offset, _ := strconv.Atoi(q.Get("offset"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	entries, total, err := app.db.BrowseBucket(bucket, q.Get("search"), offset, limit)
	if err != nil {
		http.Error(w, "bucket not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]interface{}{
		"bucket":  bucket,
		"entries": entries,
		"total":   total,
		"offset":  offset,
		"limit":   limit,
	})
}
