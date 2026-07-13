package main

import (
	"companymaps/internal/integrations/geo"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (app *App) handleRestGeoTest(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || (app.permLevel(sess, "ldap") < 1 && app.permLevel(sess, "maps") < 1) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	// Summary mode: the "Test" button geocodes a fixed sample address and
	// returns a structured {ok, checks} report for the test modal.
	if r.URL.Query().Get("summary") == "1" {
		_ = app.db.AuditLog("LDAP", sess.Username, "Geoapify API key tested")
		writeJSON(w, app.geoValidate(r.Context()))
		return
	}
	apiKey := app.db.GetGeoSetting("geoapifyApiKey")
	if strings.TrimSpace(apiKey) == "" {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "No Geoapify API key configured. Save a key first."})
		return
	}
	text := strings.TrimSpace(r.URL.Query().Get("address"))
	if text == "" {
		text = "38 Upper Montagu Street, Westminster W1H 1LJ, United Kingdom"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	lat, lon, formatted, country, city, timezone, err := geo.GeocodeAddress(ctx, apiKey, text)
	// The test issues one real API request, so count it toward the monthly estimate.
	_, _, _ = app.db.IncrGeoUsage(1)
	if err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "message": err.Error(), "address": text})
		return
	}
	_ = app.db.AuditLog("LDAP", sess.Username, "Geoapify API key tested")
	month, count := app.db.GetGeoUsage()
	writeJSON(w, map[string]interface{}{
		"ok":         true,
		"address":    text,
		"lat":        lat,
		"lon":        lon,
		"formatted":  formatted,
		"country":    country,
		"city":       city,
		"timezone":   timezone,
		"usageMonth": month,
		"usageCount": count,
	})
}

// handleRestGeoSync geocodes every map's address and stores the resulting
// lat/lon. Manual only (no scheduler). Requires ldap permission level 2 since it
// writes map records. The work runs in the background so the admin panel can
// poll handleRestGeoProgress for a live progress bar.
func (app *App) handleRestGeoSync(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !app.geoEnabled() {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "Geocoding integration is disabled."})
		return
	}
	if strings.TrimSpace(app.db.GetGeoSetting("geoapifyApiKey")) == "" {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "No Geoapify API key configured. Save a key first."})
		return
	}
	if !app.geo.Prog.Start(0, "Starting…") {
		writeJSON(w, map[string]interface{}{"ok": false, "started": false, "running": true, "message": "A geocode sync is already running."})
		return
	}
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				app.geo.Prog.Finish("", fmt.Sprintf("sync crashed: %v", rec))
			}
		}()
		res := app.geo.RunSync(&app.geo.Prog)
		_ = app.db.AuditLog("LDAP", sess.Username, fmt.Sprintf("Geoapify batch sync (%d updated, %d skipped, %d failed)", res.Updated, res.Skipped, res.Failed))
		app.geo.Prog.Finish(fmt.Sprintf("%d updated, %d skipped, %d failed.", res.Updated, res.Skipped, res.Failed), "")
	}()
	writeJSON(w, map[string]interface{}{"ok": true, "started": true})
}

// handleRestGeoProgress returns the current Geoapify batch-sync progress
// snapshot, plus the full result set once the run has finished.
func (app *App) handleRestGeoProgress(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	snap := app.geo.Prog.Snapshot()
	if done, _ := snap["done"].(bool); done {
		snap["result"] = app.geo.Result()
	}
	writeJSON(w, snap)
}

// handleRestRobinEnabled switches the Robin integration on or off. Disabling it
// stops the schedulers and blocks manual syncs/tests while leaving the saved
func (app *App) handleRestGeoEnabled(w http.ResponseWriter, r *http.Request) {
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
	_ = app.db.SetGeoSetting("geoEnabled", val)
	_ = app.db.AuditLog("LDAP", sess.Username, "Geocoding integration "+action)
	writeJSON(w, map[string]interface{}{"ok": true, "enabled": enabled})
}

// handleRestGeoDelete clears the saved Geoapify API key so the geocoding
// integration returns to its unconfigured state.
func (app *App) handleRestGeoDelete(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	_ = app.db.SetGeoSetting("geoapifyApiKey", "")
	_ = app.db.AuditLog("LDAP", sess.Username, "Geoapify API key deleted")
	writeJSON(w, map[string]interface{}{"ok": true})
}

// handleRestRobinDeskTest starts the read-only Robin desk-data diagnostic in
// the background (if one is not already running) so the admin Sync tab can poll
// for a live progress bar + log. The diagnostic walks every configured location
// (spaces → state, events, seats, seat reservations for today), captures the raw
// JSON, logs every seat reservation active right now matched to a CompanyMaps
// desk, and caches the bundle for download. It never writes to the meeting
