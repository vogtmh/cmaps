package web

import (
	"archive/zip"
	"bytes"
	"companymaps/internal/integrations/robin"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (app *Server) handleRestRobinTest(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	_ = app.db.AuditLog("LDAP", sess.Username, "Robin credentials tested")
	writeJSON(w, app.robin.Validate())
}

// handleRestRobinSync starts a Robin meeting sync in the background (if one is
// not already running) so the admin Sync tab can poll for live progress.
func (app *Server) handleRestRobinSync(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !app.robin.Enabled() {
		writeJSON(w, map[string]interface{}{"started": false, "message": "Robin integration is disabled."})
		return
	}
	if !app.robin.Prog.Start(0, "Starting…") {
		writeJSON(w, map[string]interface{}{"started": false, "running": true})
		return
	}
	_ = app.db.AuditLog("LDAP", sess.Username, "Robin meeting sync run")
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				app.robin.Prog.Finish("", fmt.Sprintf("sync crashed: %v", rec))
			}
		}()
		res := app.robin.RunSyncStructured(&app.robin.Prog)
		// Refresh the desk-reservation overlay in the same run (no-op unless the
		// "Show Robin desk reservations" mode is enabled), exactly like the
		// 5-minute scheduler does, so one button syncs everything.
		app.robin.Prog.SetStage("Syncing desk reservations…")
		app.robin.PollDeskOccupancy(&app.robin.Prog)
		if res.Note != "" {
			app.robin.Prog.Finish(res.Note, "")
			return
		}
		app.robin.Prog.Finish(fmt.Sprintf("%d of %d room(s) matched a meeting desk.", res.MatchedRooms, res.TotalRooms), "")
	}()
	writeJSON(w, map[string]interface{}{"started": true})
}

// handleRestRobinProgress returns the current Robin sync progress snapshot.
func (app *Server) handleRestRobinProgress(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, app.robin.Prog.Snapshot())
}

// handleRestGeoTest verifies the saved Geoapify API key by geocoding a single
// address. If the request supplies an "address" parameter that is used,
// otherwise it falls back to a well-known sample address. Read-only: it never
func (app *Server) handleRestRobinEnabled(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	enabled := r.URL.Query().Get("enabled") == "1" || r.FormValue("enabled") == "1"
	val := "1"
	action := "enabled"
	if !enabled {
		val = "0"
		action = "disabled"
	}
	_ = app.db.SetRobinSetting("robinEnabled", val)
	_ = app.db.AuditLog("LDAP", sess.Username, "Robin integration "+action)
	writeJSON(w, map[string]interface{}{"ok": true, "enabled": enabled})
}

// handleRestRobinDelete clears the saved Robin token and organisation id so the
// integration returns to its unconfigured state.
func (app *Server) handleRestRobinDelete(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	_ = app.db.SetRobinSetting("robintoken", "")
	_ = app.db.SetRobinSetting("robinOrganisation", "")
	_ = app.db.AuditLog("LDAP", sess.Username, "Robin credentials deleted")
	writeJSON(w, map[string]interface{}{"ok": true})
}

// handleRestGeoEnabled switches the Geoapify geocoding integration on or off.
// Disabling it blocks manual geocode syncs/tests while leaving the saved API key
func (app *Server) handleRestRobinDeskTest(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !app.robin.Enabled() {
		writeJSON(w, map[string]interface{}{"started": false, "message": "Robin integration is disabled."})
		return
	}
	if !app.robin.DeskProg.Start(0, "Starting…") {
		writeJSON(w, map[string]interface{}{"started": false, "running": true})
		return
	}
	_ = app.db.AuditLog("LDAP", sess.Username, "Robin desk-data diagnostic run")
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				app.robin.DeskProg.Finish("", fmt.Sprintf("diagnostic crashed: %v", rec))
			}
		}()
		_, files, res := app.robin.RunDeskDump(&app.robin.DeskProg)
		app.robin.SetDump(files, time.Now().Format("2006-01-02 15:04:05"))
		app.robin.DeskProg.Finish(fmt.Sprintf("%d desk(s) occupied now matched (%d unmatched). %d JSON file(s) captured.",
			res.Matched, res.Unmatched, res.Files), "")
	}()
	writeJSON(w, map[string]interface{}{"started": true})
}

// handleRestRobinDeskProgress returns the current desk-diagnostic progress.
func (app *Server) handleRestRobinDeskProgress(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, app.robin.DeskProg.Snapshot())
}

// handleRestRobinDeskDump streams the most recently captured desk-data
// diagnostic bundle as a zip. If no bundle has been captured yet (or it is
// empty) it runs a fresh diagnostic first.
func (app *Server) handleRestRobinDeskDump(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	files, when := app.robin.Dump()

	var logs []string
	if len(files) == 0 {
		logs, files = app.robin.RunDeskDumpNow()
		when = time.Now().Format("2006-01-02 15:04:05")
		app.robin.SetDump(files, when)
	}
	_ = app.db.AuditLog("LDAP", sess.Username, "Robin desk-data diagnostic download")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// summary.json: metadata plus the run log (if we have one).
	summary := map[string]interface{}{
		"generated":    time.Now().Format(time.RFC3339),
		"captured_at":  when,
		"organisation": app.db.GetRobinSetting("robinOrganisation"),
		"file_count":   len(files),
	}
	if logs != nil {
		summary["log"] = logs
	}
	if sb, err := json.MarshalIndent(summary, "", "  "); err == nil {
		if fw, err := zw.Create("summary.json"); err == nil {
			_, _ = fw.Write(sb)
		}
	}

	for _, f := range files {
		fw, err := zw.Create(f.Name)
		if err != nil {
			continue
		}
		_, _ = fw.Write(f.Data)
	}
	if err := zw.Close(); err != nil {
		http.Error(w, "could not build zip", http.StatusInternalServerError)
		return
	}

	fname := "robin-desk-dump-" + time.Now().Format("20060102-150405") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+fname+"\"")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	_, _ = w.Write(buf.Bytes())
}

// handleRestRobinSuggestions starts a read-only background scan of every mapped
// Robin location's seats, proposing strip prefixes/suffixes that would make a
// near-miss seat name match a CompanyMaps desk number. The admin Sync tab polls
// handleRestRobinSuggestionsProgress for a live progress bar and the results.
func (app *Server) handleRestRobinSuggestions(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !app.robin.SuggestProg.Start(0, "Starting…") {
		writeJSON(w, map[string]interface{}{"started": false, "running": true})
		return
	}
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				app.robin.SuggestProg.Finish("", fmt.Sprintf("scan crashed: %v", rec))
			}
		}()
		suggestions, err := app.robin.CollectStripSuggestions(&app.robin.SuggestProg)
		if err != nil {
			app.robin.SuggestProg.Finish("", err.Error())
			return
		}
		app.robin.SetSuggestResult(suggestions)
		app.robin.SuggestProg.Finish(fmt.Sprintf("%d suggestion(s) found.", len(suggestions)), "")
	}()
	writeJSON(w, map[string]interface{}{"started": true})
}

// handleRestRobinSuggestionsProgress returns the current strip-suggestion scan
// progress. Once the scan is done it also includes the suggestions list.
func (app *Server) handleRestRobinSuggestionsProgress(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	snap := app.robin.SuggestProg.Snapshot()
	if done, _ := snap["done"].(bool); done {
		snap["suggestions"] = app.robin.SuggestResult()
	}
	writeJSON(w, snap)
}

// handleRestRobinStripAdd appends a single strip prefix/suffix pattern to the
// configured list (enabling that strip type) so a suggestion can be applied with
// one click.
func (app *Server) handleRestRobinStripAdd(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	pat := r.FormValue("pattern")
	if strings.TrimSpace(pat) == "" {
		respondError(w, "empty pattern")
		return
	}
	var listKey, enKey string
	switch r.FormValue("type") {
	case "prefix":
		listKey, enKey = "robinStripPrefixList", "robinStripPrefixEnabled"
	case "suffix":
		listKey, enKey = "robinStripSuffixList", "robinStripSuffixEnabled"
	default:
		respondError(w, "invalid type")
		return
	}
	existing := robin.SplitList(app.db.GetRobinSetting(listKey))
	for _, e := range existing {
		if e == pat {
			writeJSON(w, map[string]interface{}{"ok": true, "already": true})
			return
		}
	}
	existing = append(existing, pat)
	_ = app.db.SetRobinSetting(listKey, strings.Join(existing, "\n"))
	_ = app.db.SetRobinSetting(enKey, "1")
	_ = app.db.AuditLog("LDAP", sess.Username, "Robin strip "+r.FormValue("type")+" added: "+pat)
	writeJSON(w, map[string]interface{}{"ok": true})
}

// handleRestLdapSync starts an AD sync of all sources in the background so the
