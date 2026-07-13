package main

import (
	"companymaps/internal/auth"
	"companymaps/internal/config"
	"companymaps/internal/integrations/geo"
	"companymaps/internal/integrations/robin"
	"companymaps/internal/progress"

	"html/template"
	"io/fs"
	"net/http"
	"sync"
	"time"
)

// App is the central application container shared by all handlers.
type App struct {
	cfg        *config.Config
	db         *DB
	sessions   *SessionStore
	tmpl       *template.Template
	staticFS   fs.FS
	importProg ImportProgress

	syncDebugMu sync.Mutex
	syncDebug   ADSyncDebug

	ldapProg  progress.Progress
	entraProg progress.Progress

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

	// next*Sync hold the wall-clock time of the next scheduled automatic sync
	// for the LDAP/Entra schedulers, surfaced in the admin Sync tab. In-memory
	// only (they reset on restart, which is correct since the schedulers re-arm
	// on boot). Robin's equivalent lives on robin.Service.
	nextSyncMu    sync.Mutex
	nextLdapSync  time.Time
	nextEntraSync time.Time

	// startTime is the process boot time, used to compute uptime on the dashboard.
	startTime time.Time

	// intgHealth caches the most recent hourly connectivity-test result for each
	// sync integration ("ldap", "entra", "saml", "robin"), rendered on the
	// dashboard. The whole map is replaced on each run, so readers copy the
	// reference under the mutex and never mutate it in place.
	intgHealthMu sync.Mutex
	intgHealth   map[string]intgHealthResult
}

// setNextSync records the next scheduled sync time for one integration.
func (app *App) setNextSync(dst *time.Time, t time.Time) {
	app.nextSyncMu.Lock()
	*dst = t
	app.nextSyncMu.Unlock()
}

// getNextSync returns the next scheduled sync time for one integration.
func (app *App) getNextSync(src *time.Time) time.Time {
	app.nextSyncMu.Lock()
	defer app.nextSyncMu.Unlock()
	return *src
}

// Session management lives in internal/auth; these aliases keep the root
// package handlers compiling until they move into internal/web (Phase 4).
type (
	Session      = auth.Session
	SessionStore = auth.SessionStore
)

// NewSessionStore returns an empty session store.
func NewSessionStore() *SessionStore { return auth.NewSessionStore() }

const sessionCookie = "cmaps_session"

// currentSession returns the Session for the request, if authenticated.
func (app *App) currentSession(r *http.Request) (Session, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return Session{}, false
	}
	return app.sessions.Get(c.Value)
}

func (app *App) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   8 * 60 * 60,
	})
}

func (app *App) clearSessionCookie(w http.ResponseWriter) {
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
func (app *App) resetUsermodeCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "setting_usermode",
		Value:    "edit",
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})
}
