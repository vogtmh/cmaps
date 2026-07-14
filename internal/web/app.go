package web

import (
	"companymaps/internal/store"
	"fmt"

	"companymaps/internal/auth"
	"companymaps/internal/config"
	"companymaps/internal/directory"
	"companymaps/internal/integrations/geo"
	"companymaps/internal/integrations/robin"
	"companymaps/internal/progress"

	"html/template"
	"io/fs"
	"net/http"
	"sync"
	"time"
)

// Server is the HTTP layer: it owns the parsed templates, the session store
// and references to the domain services, and hosts every handler as a method.
type Server struct {
	cfg        *config.Config
	db         *store.DB
	sessions   *SessionStore
	tmpl       *template.Template
	staticFS   fs.FS
	importProg ImportProgress

	// dir owns the LDAP/EntraID directory-sync engine: progress trackers,
	// sync diagnostics and the next scheduled run times.
	dir *directory.Syncer

	// migrateProg tracks the background identifier migration (samaccountname <->
	// mail) so the admin Sync > General subtab can show a live progress bar + log.
	migrateProg progress.Progress

	// robin owns the Robin Powered integration: meeting/desk syncs, their
	// progress trackers, the diagnostic dump cache and the scheduler state.
	robin *robin.Service

	// exportProg tracks the background build of a full data export so the admin
	// Backup dialog can show a determinate progress bar. exportPath/exportName
	// point at the finished zip awaiting download.
	exportProg progress.Progress
	exportMu   sync.Mutex
	exportPath string
	exportName string

	// geo owns the Geoapify integration: enablement, batch geocode runs and
	// the progress/result pair polled by the admin Sync panel.
	geo *geo.Service

	// startTime is the process boot time, used to compute uptime on the dashboard.
	startTime time.Time

	// intgHealth caches the most recent hourly connectivity-test result for each
	// sync integration ("ldap", "entra", "saml", "robin"), rendered on the
	// dashboard. The whole map is replaced on each run, so readers copy the
	// reference under the mutex and never mutate it in place.
	intgHealthMu sync.Mutex
	intgHealth   map[string]intgHealthResult
}

// NewServer builds the web layer: it parses the embedded templates, prepares
// the static file system and wires the domain services. The returned Server
// exposes Routes() for mux registration and the scheduler start methods.
func NewServer(cfg *config.Config, db *store.DB, dirSvc *directory.Syncer, robinSvc *robin.Service, geoSvc *geo.Service) (*Server, error) {
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"ucfirst":  ucfirst,
		"assetver": func() string { return assetVersion },
	}).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("templates: %w", err)
	}
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, fmt.Errorf("static fs: %w", err)
	}
	app := &Server{
		cfg:       cfg,
		db:        db,
		sessions:  auth.NewSessionStore(),
		tmpl:      tmpl,
		staticFS:  staticSub,
		startTime: time.Now(),
		dir:       dirSvc,
		robin:     robinSvc,
		geo:       geoSvc,
	}
	// The demo source's employees + avatars are generated from data owned by
	// this layer, so they are injected after construction.
	dirSvc.DemoDirectory = app.demoDirectoryUsers
	dirSvc.EnsureDemoAvatars = app.ensureDemoAvatars
	return app, nil
}

// Handler returns the fully wired HTTP handler: all routes registered on a
// fresh mux, wrapped with gzip compression.
func (app *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	app.routes(mux)
	return gzipMiddleware(mux)
}

// Session aliases the auth session types for the handlers in this package.
type (
	Session      = auth.Session
	SessionStore = auth.SessionStore
)

const sessionCookie = "cmaps_session"

// currentSession returns the Session for the request, if authenticated.
func (app *Server) currentSession(r *http.Request) (Session, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return Session{}, false
	}
	return app.sessions.Get(c.Value)
}

func (app *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   8 * 60 * 60,
	})
}

func (app *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// resetUsermodeCookie forces edit mode on a fresh login instead of inheriting the
// previously persisted setting_usermode value.
func (app *Server) resetUsermodeCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "setting_usermode",
		Value:    "edit",
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})
}
