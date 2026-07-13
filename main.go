package main

import (
	"companymaps/internal/auth/saml"
	"companymaps/internal/config"
	"companymaps/internal/store"
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// sampleFS holds the bundled demo maps and avatars used by the "set up a new
// server" wizard path (Phase 2).
//
//go:embed sample
var sampleFS embed.FS

// assetVersion is appended as a ?v= query to versioned assets (JS/CSS) so a new
// deployment busts the browser cache. It is derived from the running binary's
// modification time, which changes on every deploy/rebuild.
var assetVersion = computeAssetVersion()

func computeAssetVersion() string {
	if exe, err := os.Executable(); err == nil {
		if fi, err := os.Stat(exe); err == nil {
			return strconv.FormatInt(fi.ModTime().Unix(), 36)
		}
	}
	return strconv.FormatInt(time.Now().Unix(), 36)
}

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

func main() {
	cfg, err := config.LoadOrCreate()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Ensure the data directory and its subfolders exist.
	for _, d := range []string{cfg.DataDir, cfg.DataPath("maps"), cfg.DataPath("avatarcache"), cfg.DataPath("logos"), cfg.DataPath("itemtypes")} {
		if err := os.MkdirAll(d, 0755); err != nil {
			log.Fatalf("creating data dir %s: %v", d, err)
		}
	}

	db, err := store.Open(cfg.DataPath("cmaps.db"))
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	tmpl, err := template.New("").Funcs(template.FuncMap{
		"ucfirst":  ucfirst,
		"assetver": func() string { return assetVersion },
	}).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		log.Fatalf("templates: %v", err)
	}

	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}

	app := &App{
		cfg:       cfg,
		db:        db,
		sessions:  NewSessionStore(),
		tmpl:      tmpl,
		staticFS:  staticSub,
		startTime: time.Now(),
	}

	// Backfill newer optional settings so they appear in the admin panel on
	// installations created before the setting existed.
	if err := db.EnsureSetting("reportURL", ""); err != nil {
		log.Fatalf("ensure settings: %v", err)
	}
	if err := db.EnsureSetting("nomapText", ""); err != nil {
		log.Fatalf("ensure settings: %v", err)
	}
	if err := db.EnsureSetting("nomapLink", ""); err != nil {
		log.Fatalf("ensure settings: %v", err)
	}

	mux := http.NewServeMux()
	app.routes(mux)

	// Migrate the legacy single EntraID connection into the multi-source model.
	app.migrateEntraConfig()

	// Wrap the whole mux with gzip so text-based responses (HTML, JS, CSS, JSON
	// desk/changes payloads) are compressed for clients that support it.
	handler := gzipMiddleware(mux)

	// Background AD mirror refresh: first run 5 minutes after startup, then
	// hourly (no-op while no AD source is enabled).
	app.startPeriodicSync(firstSyncDelay, syncInterval,
		app.anyLdapSourceEnabled,
		func() {
			if n, err := app.RunADSync(); err != nil {
				log.Printf("scheduled AD sync failed: %v", err)
			} else {
				log.Printf("scheduled AD sync: %d placements mirrored", n)
			}
		},
		func(t time.Time) { app.setNextSync(&app.nextLdapSync, t) },
	)

	// Background EntraID (Microsoft Graph) mirror refresh: same cadence as AD
	// (no-op until an EntraID app registration is enabled).
	app.startPeriodicSync(firstSyncDelay, syncInterval,
		app.entraHasEnabledSource,
		func() {
			if n, err := app.RunEntraSync(); err != nil {
				log.Printf("scheduled EntraID sync failed: %v", err)
			} else {
				log.Printf("scheduled EntraID sync: %d placements mirrored", n)
			}
		},
		func(t time.Time) { app.setNextSync(&app.nextEntraSync, t) },
	)

	// Background Robin meeting-room + desk-occupancy refresh every 5 minutes
	// (first run 5 minutes after startup, no-op until Robin is configured).
	app.StartRobinScheduler(5 * time.Minute)

	// Background Robin location discovery (no-op until token + organisation set).
	app.StartRobinLocationScheduler(1 * time.Hour)

	// Hourly connectivity tests for every sync integration (LDAP, EntraID, SAML,
	// Robin), surfaced on the dashboard. First run shortly after boot.
	app.startIntegrationHealthScheduler(30*time.Second, time.Hour)

	// Discard abandoned identifier-migration staging (temp buckets) that was
	// never applied within its 1-hour TTL.
	app.startMigStageJanitor(10 * time.Minute)

	log.Printf("CompanyMaps 9 listening on %s (data dir: %s)", cfg.ListenAddr, cfg.DataDir)
	if err := http.ListenAndServe(cfg.ListenAddr, handler); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// Scheduled-sync cadence: the first automatic LDAP/EntraID sync runs shortly
// after startup, then repeats hourly. Robin keeps its own 5-minute ticker.
const (
	firstSyncDelay = 5 * time.Minute
	syncInterval   = 60 * time.Minute
)

// startPeriodicSync runs run() first after `first`, then every `interval`. It
// records the next scheduled fire time via setNext (even while disabled, so the
// admin Sync tab always shows a next time) and skips the actual run whenever
// enabled() reports false.
func (app *App) startPeriodicSync(first, interval time.Duration, enabled func() bool, run func(), setNext func(time.Time)) {
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

func (app *App) routes(mux *http.ServeMux) {
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
func (app *App) registerSAMLRoutes(mux *http.ServeMux) {
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
func (app *App) dataFile(sub, name string) string {
	clean := filepath.Base(name)
	return app.cfg.DataPath(sub, clean)
}
