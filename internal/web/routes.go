package web

import (
	"log/slog"
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
				slog.Default().Error("scheduled AD sync failed", "err", err)
			} else {
				slog.Default().Info("scheduled AD sync complete", "placements", n)
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
				slog.Default().Error("scheduled EntraID sync failed", "err", err)
			} else {
				slog.Default().Info("scheduled EntraID sync complete", "placements", n)
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

// registerRESTRoutes wires up the REST API. Both the bare and trailing-slash
// forms are registered so the legacy front-end URLs (rest/desks/) keep working.
func (app *Server) registerRESTRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/rest/account/", app.handleRestAccount)

	rest := func(path string, h http.HandlerFunc) {
		mux.HandleFunc(path, h)
		mux.HandleFunc(path+"/", h)
	}
	rest("/rest/desks", app.handleRestDesks)
	rest("/rest/users", app.handleRestUsers)
	rest("/rest/config", app.handleRestConfig)
	rest("/rest/teams", app.handleRestTeams)
	rest("/rest/auditlog", app.handleRestAuditlog)
	rest("/rest/booking", app.handleRestBooking)
	rest("/rest/changes", app.handleRestChanges)
	rest("/rest/stats", app.handleRestStats)
	rest("/rest/avatar", app.handleRestAvatar)
	rest("/rest/avatar-orientation", app.handleRestAvatarOrientation)
	rest("/rest/update", app.handleRestUpdate)
	rest("/rest/meeting", app.handleRestMeeting)
	rest("/rest/system", app.handleRestSystem)
	rest("/rest/dashboard", app.handleRestDashboard)
	rest("/rest/dashboard/overview", app.handleRestDashboardOverview)
	rest("/rest/dashboard/system", app.handleRestDashboardSystem)
	rest("/rest/dashboard/integrations", app.handleRestDashboardIntegrations)
	rest("/rest/dashboard/visitors", app.handleRestDashboardVisitors)
	rest("/rest/ldap", app.handleRestLdap)
	rest("/rest/ldap/debug", app.handleRestLdapDebug)
	rest("/rest/ldap/sync", app.handleRestLdapSync)
	rest("/rest/ldap/progress", app.handleRestLdapProgress)
	rest("/rest/ldap/test", app.handleRestLdapTest)
	rest("/rest/sourceseats", app.handleRestSourceSeats)
	rest("/rest/entra/sync", app.handleRestEntraSync)
	rest("/rest/entra/progress", app.handleRestEntraProgress)
	rest("/rest/entra/test", app.handleRestEntraTest)
	rest("/rest/entra/syncone", app.handleRestEntraSyncOne)
	rest("/rest/entra/gencert", app.handleRestEntraGenCert)
	rest("/rest/directory/search", app.handleRestDirectorySearch)
	rest("/rest/directory/match", app.handleRestDirectoryMatch)
	rest("/rest/identifier/analyze", app.handleRestIdentifierAnalyze)
	rest("/rest/identifier/create", app.handleRestIdentifierCreate)
	rest("/rest/identifier/stageresult", app.handleRestIdentifierStageResult)
	rest("/rest/identifier/cancel", app.handleRestIdentifierCancel)
	rest("/rest/identifier/apply", app.handleRestIdentifierApply)
	rest("/rest/identifier/progress", app.handleRestIdentifierProgress)
	rest("/rest/vips", app.handleRestVips)
	rest("/rest/setting", app.handleRestSetting)
	rest("/rest/maps/coords", app.handleRestMapCoords)
	rest("/rest/robin/test", app.handleRestRobinTest)
	rest("/rest/robin/sync", app.handleRestRobinSync)
	rest("/rest/robin/progress", app.handleRestRobinProgress)
	rest("/rest/robin/desktest", app.handleRestRobinDeskTest)
	rest("/rest/robin/desktest/progress", app.handleRestRobinDeskProgress)
	rest("/rest/robin/deskdump", app.handleRestRobinDeskDump)
	rest("/rest/robin/suggestions", app.handleRestRobinSuggestions)
	rest("/rest/robin/suggestions/progress", app.handleRestRobinSuggestionsProgress)
	rest("/rest/robin/strip/add", app.handleRestRobinStripAdd)
	rest("/rest/robin/enabled", app.handleRestRobinEnabled)
	rest("/rest/robin/delete", app.handleRestRobinDelete)
	rest("/rest/geo/test", app.handleRestGeoTest)
	rest("/rest/geo/sync", app.handleRestGeoSync)
	rest("/rest/geo/progress", app.handleRestGeoProgress)
	rest("/rest/geo/enabled", app.handleRestGeoEnabled)
	rest("/rest/geo/delete", app.handleRestGeoDelete)
	rest("/rest/export/start", app.handleRestExportStart)
	rest("/rest/export/progress", app.handleRestExportProgress)
	rest("/rest/export/download", app.handleRestExportDownload)
	rest("/rest/import", app.handleRestImport)
	rest("/rest/db/buckets", app.handleRestDBBuckets)
	rest("/rest/db/entries", app.handleRestDBEntries)
}
