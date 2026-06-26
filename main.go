package main

import (
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
	cfg, err := loadOrCreateConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Ensure the data directory and its subfolders exist.
	for _, d := range []string{cfg.DataDir, cfg.dataPath("maps"), cfg.dataPath("avatarcache"), cfg.dataPath("logos")} {
		if err := os.MkdirAll(d, 0755); err != nil {
			log.Fatalf("creating data dir %s: %v", d, err)
		}
	}

	db, err := openDB(cfg.dataPath("cmaps.db"))
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
		cfg:      cfg,
		db:       db,
		sessions: NewSessionStore(),
		tmpl:     tmpl,
		staticFS: staticSub,
	}

	mux := http.NewServeMux()
	app.routes(mux)

	// Background AD mirror refresh (no-op until an AD source is configured).
	app.StartADSyncScheduler(6 * time.Hour)

	// Background Robin meeting-room refresh (no-op until Robin is configured).
	app.StartRobinScheduler(5 * time.Minute)

	log.Printf("CompanyMaps 9 listening on %s (data dir: %s)", cfg.ListenAddr, cfg.DataDir)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func (app *App) routes(mux *http.ServeMux) {
	// Pages
	mux.HandleFunc("/", app.handleIndex)
	mux.HandleFunc("/login", app.handleLogin)
	mux.HandleFunc("/logout", app.handleLogout)

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
	mux.Handle("/maps/", cacheControl(24*time.Hour, http.StripPrefix("/maps/", http.FileServer(http.Dir(app.cfg.dataPath("maps"))))))
	mux.Handle("/avatarcache/", cacheControl(24*time.Hour, http.StripPrefix("/avatarcache/", http.FileServer(http.Dir(app.cfg.dataPath("avatarcache"))))))
	mux.Handle("/logos/", cacheControl(24*time.Hour, http.StripPrefix("/logos/", http.FileServer(http.Dir(app.cfg.dataPath("logos"))))))

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
	mux.HandleFunc(samlACSPath, app.handleSAMLACS)
	mux.HandleFunc(samlLogoutPath, app.handleSAMLLogout)

	// Admin-only SAML configuration REST endpoints.
	mux.HandleFunc("/rest/saml/status", app.handleSAMLStatus)
	mux.HandleFunc("/rest/saml/settings", app.requirePerm("adminpanel", 2, app.handleSAMLSettings))
	mux.HandleFunc("/rest/saml/spinfo", app.requirePerm("adminpanel", 2, app.handleSAMLSPInfo))
	mux.HandleFunc("/rest/saml/debug", app.requirePerm("adminpanel", 2, app.handleSAMLDebugLast))
	mux.HandleFunc("/rest/saml/import-metadata", app.requirePerm("adminpanel", 2, app.handleSAMLImportMetadata))
}

// dataFile resolves a path inside the data directory, guarding against traversal.
func (app *App) dataFile(sub, name string) string {
	clean := filepath.Base(name)
	return app.cfg.dataPath(sub, clean)
}
