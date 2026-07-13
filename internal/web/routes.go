package web

import (
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"companymaps/internal/auth/saml"
)

// cacheControl wraps a handler and adds a long-lived public Cache-Control header
// so browsers cache static assets and user images instead of refetching them on
// every page load. Versioned assets (JS/CSS via ?v=) are safe to cache long-term.
func cacheControl(maxAge time.Duration, h http.Handler) http.Handler {
	value := "public, max-age=" + strconv.Itoa(int(maxAge.Seconds()))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", value)
		h.ServeHTTP(w, r)
	})
}

func (app *Server) routes(mux *http.ServeMux) {
	// Pages
	mux.HandleFunc("/", app.handleIndex)
	mux.HandleFunc("/login", app.handleLogin)
	mux.HandleFunc("/logout", app.handleLogout)

	// Dedicated touch-first mobile UI (separate layout under /m/).
	mux.HandleFunc("/m", app.handleMobile)
	mux.HandleFunc("/m/", app.handleMobile)
	// PWA endpoints under /m/ (more specific than /m/, so they take precedence).
	mux.HandleFunc("/m/sw.js", app.handleMobileServiceWorker)
	mux.HandleFunc("/m/manifest.webmanifest", app.handleMobileManifest)

	// Admin panel (GET render + POST CRUD).
	mux.HandleFunc("/admin", app.handleAdmin)
	mux.HandleFunc("/admin/", app.handleAdmin)

	// Superadmin one-time audit-log re-import from legacy MySQL.
	mux.HandleFunc("/admin/audit-reimport", app.handleAuditReimport)

	// Change-overview page (legacy changes.php).
	mux.HandleFunc("/changes", app.handleChanges)

	// Static assets (embedded)
	mux.Handle("/static/", cacheControl(24*time.Hour, http.StripPrefix("/static/", http.FileServer(http.FS(app.staticFS)))))

	// User data served from the data directory (maps, avatar cache).
	mux.Handle("/maps/", cacheControl(24*time.Hour, http.StripPrefix("/maps/", http.FileServer(http.Dir(app.cfg.DataPath("maps"))))))
	mux.HandleFunc("/avatarcache/", app.serveAvatar)
	mux.Handle("/logos/", cacheControl(24*time.Hour, http.StripPrefix("/logos/", http.FileServer(http.Dir(app.cfg.DataPath("logos"))))))
	mux.Handle("/itemicons/", cacheControl(24*time.Hour, http.StripPrefix("/itemicons/", http.FileServer(http.Dir(app.cfg.DataPath("itemtypes"))))))

	// First-run setup wizard.
	mux.HandleFunc("/setup", app.handleSetup)
	mux.HandleFunc("/setup/demo", app.handleSetupDemo)
	mux.HandleFunc("/setup/import", app.handleSetupImport)
	mux.HandleFunc("/setup/import/progress", app.handleSetupImportProgress)
	mux.HandleFunc("/setup/datacopy", app.handleSetupDataCopy)
	mux.HandleFunc("/setup/ldap", app.handleSetupLdap)
	mux.HandleFunc("/setup/robin", app.handleSetupRobin)
	mux.HandleFunc("/setup/finish", app.handleSetupFinish)

	// SAML single sign-on. The ACS/logout paths mirror the legacy SimpleSAMLphp
	// deployment so the existing Entra app registration needs no changes.
	app.registerSAMLRoutes(mux)

	// REST API.
	app.registerRESTRoutes(mux)
}

// registerSAMLRoutes wires up the SAML SSO endpoints, including the legacy
// SimpleSAMLphp ACS/logout paths the Entra app registration points at.
func (app *Server) registerSAMLRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/auth/saml/metadata", app.handleSAMLMetadata)
	mux.HandleFunc("/auth/saml/login", app.handleSAMLLogin)
	mux.HandleFunc("/auth/saml/logout", app.handleSAMLLogout)
	mux.HandleFunc(saml.ACSPath, app.handleSAMLACS)
	mux.HandleFunc(saml.LogoutPath, app.handleSAMLLogout)

	// Admin-only SAML configuration REST endpoints.
	mux.HandleFunc("/rest/saml/status", app.handleSAMLStatus)
	mux.HandleFunc("/rest/saml/settings", app.requirePerm("adminpanel", 2, app.handleSAMLSettings))
	mux.HandleFunc("/rest/saml/validate", app.requirePerm("adminpanel", 2, app.handleSAMLValidate))
	mux.HandleFunc("/rest/saml/spinfo", app.requirePerm("adminpanel", 2, app.handleSAMLSPInfo))
	mux.HandleFunc("/rest/saml/debug", app.requirePerm("adminpanel", 2, app.handleSAMLDebugLast))
	mux.HandleFunc("/rest/saml/mycapture", app.requirePerm("adminpanel", 1, app.handleSAMLMyCapture))
	mux.HandleFunc("/rest/saml/import-metadata", app.requirePerm("adminpanel", 2, app.handleSAMLImportMetadata))
}

// dataFile resolves a path inside the data directory, guarding against traversal.
func (app *Server) dataFile(sub, name string) string {
	clean := filepath.Base(name)
	return app.cfg.DataPath(sub, clean)
}

// StartSchedulers launches every background job: the LDAP/EntraID/Robin sync
// timers, the hourly integration health checks and the identifier-migration
// staging janitor.
func (app *Server) StartSchedulers() {
	const (
		firstSyncDelay = 5 * time.Minute
		syncInterval   = 60 * time.Minute
	)

	// Background AD mirror refresh: first run 5 minutes after startup, then
	// hourly (no-op while no AD source is enabled).
	app.startPeriodicSync(firstSyncDelay, syncInterval,
		app.dir.AnyLdapSourceEnabled,
		func() {
			if n, err := app.dir.RunADSync(); err != nil {
				log.Printf("scheduled AD sync failed: %v", err)
			} else {
				log.Printf("scheduled AD sync: %d placements mirrored", n)
			}
		},
		app.dir.SetNextLdapSync,
	)

	// Background EntraID (Microsoft Graph) mirror refresh: same cadence as AD
	// (no-op until an EntraID app registration is enabled).
	app.startPeriodicSync(firstSyncDelay, syncInterval,
		app.dir.EntraHasEnabledSource,
		func() {
			if n, err := app.dir.RunEntraSync(); err != nil {
				log.Printf("scheduled EntraID sync failed: %v", err)
			} else {
				log.Printf("scheduled EntraID sync: %d placements mirrored", n)
			}
		},
		app.dir.SetNextEntraSync,
	)

	// Background Robin meeting-room + desk-occupancy refresh every 5 minutes
	// (first run 5 minutes after startup, no-op until Robin is configured).
	app.robin.StartScheduler(5 * time.Minute)

	// Background Robin location discovery (no-op until token + organisation set).
	app.robin.StartLocationScheduler(1 * time.Hour)

	// Hourly connectivity tests for every sync integration (LDAP, EntraID,
	// SAML, Robin), surfaced on the dashboard. First run shortly after boot.
	app.startIntegrationHealthScheduler(30*time.Second, time.Hour)

	// Discard abandoned identifier-migration staging (temp buckets) that was
	// never applied within its 1-hour TTL.
	app.startMigStageJanitor(10 * time.Minute)
}

// startPeriodicSync runs run() first after `first`, then every `interval`. It
// records the next scheduled fire time via setNext (even while disabled, so the
// admin Sync tab always shows a next time) and skips the actual run whenever
// enabled() reports false.
func (app *Server) startPeriodicSync(first, interval time.Duration, enabled func() bool, run func(), setNext func(time.Time)) {
	go func() {
		setNext(time.Now().Add(first))
		timer := time.NewTimer(first)
		defer timer.Stop()
		for range timer.C {
			if enabled() {
				run()
			}
			setNext(time.Now().Add(interval))
			timer.Reset(interval)
		}
	}()
}
