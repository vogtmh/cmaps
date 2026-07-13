package main

// Geoapify admin-facing glue. The geocoding engine and batch sync live in
// internal/integrations/geo; this file keeps the structured connection test
// used by the admin test modal.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"companymaps/internal/integrations/geo"
)

// geoEnabled reports whether the Geoapify geocoding integration is switched on.
func (app *App) geoEnabled() bool { return app.geo.Enabled() }

// geoValidate runs a structured, read-only test of the Geoapify integration and
// returns the {ok, checks} payload rendered by the admin test modal. It confirms
// an API key is configured and geocodes a fixed sample address to prove the key
// works — without touching any map coordinates.
func (app *App) geoValidate(ctx context.Context) map[string]interface{} {
	var checks []testCheck
	add := func(name, status, detail string) {
		checks = append(checks, testCheck{Name: name, Status: status, Detail: detail})
	}

	if app.geoEnabled() {
		add("Integration status", "ok", "Geocoding is enabled.")
	} else {
		add("Integration status", "warn", "Geocoding is disabled; scheduled and bulk syncs are skipped.")
	}

	apiKey := strings.TrimSpace(app.db.GetGeoSetting("geoapifyApiKey"))
	if apiKey == "" {
		add("API key", "fail", "No Geoapify API key configured. Save a key first.")
		return testResult(checks)
	}
	add("API key", "ok", "A Geoapify API key is configured.")

	const sample = "38 Upper Montagu Street, Westminster W1H 1LJ, United Kingdom"
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	lat, lon, formatted, _, _, _, err := geo.GeocodeAddress(cctx, apiKey, sample)
	// The test issues one real API request, so count it toward the estimate.
	_, _, _ = app.db.IncrGeoUsage(1)
	if err != nil {
		add("Geocoding request", "fail", "Geoapify rejected the request: "+err.Error())
		return app.geoTestPayload(checks)
	}
	if formatted == "" {
		formatted = sample
	}
	add("Geocoding request", "ok", fmt.Sprintf("Resolved %q to lat %.5f, lon %.5f.", formatted, lat, lon))

	return app.geoTestPayload(checks)
}

// geoTestPayload wraps checks into {ok, checks} and appends the current monthly
// usage estimate so the admin modal can refresh the counter after a test.
func (app *App) geoTestPayload(checks []testCheck) map[string]interface{} {
	res := testResult(checks)
	month, count := app.db.GetGeoUsage()
	res["usageMonth"] = month
	res["usageCount"] = count
	return res
}
