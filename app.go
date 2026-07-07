package main

import (
	"crypto/rand"
	"encoding/hex"
	"html/template"
	"io/fs"
	"net/http"
	"sync"
	"time"
)

// App is the central application container shared by all handlers.
type App struct {
	cfg        *Config
	db         *DB
	sessions   *SessionStore
	tmpl       *template.Template
	staticFS   fs.FS
	importProg ImportProgress

	syncDebugMu sync.Mutex
	syncDebug   ADSyncDebug

	robinProg syncProgress
	ldapProg  syncProgress
	entraProg syncProgress

	// migrateProg tracks the background identifier migration (samaccountname <->
	// mail) so the admin Sync > General subtab can show a live progress bar + log.
	migrateProg syncProgress

	// robinDeskProg tracks the background desk-data diagnostic so the admin Sync
	// tab can show a live progress bar + log.
	robinDeskProg syncProgress

	// robinSuggestProg tracks the background strip-pattern suggestion scan so the
	// admin Sync tab can show a live progress bar. robinSuggestResult holds the
	// suggestions from the most recent completed scan.
	robinSuggestProg   syncProgress
	robinSuggestMu     sync.Mutex
	robinSuggestResult []robinStripSuggestion

	// robinDump caches the most recent desk-data diagnostic bundle so the admin
	// "Download JSON bundle" button can export exactly what the last run captured
	// without re-hitting the Robin API.
	robinDumpMu    sync.Mutex
	robinDumpFiles []robinDumpFile
	robinDumpTime  string

	// exportProg tracks the background build of a full data export so the admin
	// Backup dialog can show a determinate progress bar. exportPath/exportName
	// point at the finished zip awaiting download.
	exportProg syncProgress
	exportMu   sync.Mutex
	exportPath string
	exportName string

	// geoProg tracks the background Geoapify batch geocode so the admin Sync
	// panel can show a live progress bar. geoResult holds the most recent
	// completed run for rendering once it finishes.
	geoProg   syncProgress
	geoMu     sync.Mutex
	geoResult GeoSyncResult

	// next*Sync hold the wall-clock time of the next scheduled automatic sync for
	// each integration, surfaced in the admin Sync tab. In-memory only (they
	// reset on restart, which is correct since the schedulers re-arm on boot).
	nextSyncMu    sync.Mutex
	nextLdapSync  time.Time
	nextEntraSync time.Time
	nextRobinSync time.Time

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

// Session holds the authenticated user's identity, mirroring the PHP $_SESSION
// values. AdminPassword is true when the user authenticated with the local admin
// password from config.json (the break-glass superadmin).
type Session struct {
	Expiry         time.Time
	AdminPassword  bool
	Samaccountname string
	Username       string // mapadmins key, e.g. "tvcorp\\INT001327" or "admin"
	Fullname       string
	Mail           string
	Phone          string
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]Session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]Session)}
}

func (s *SessionStore) Create(sess Session) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	sess.Expiry = time.Now().Add(8 * time.Hour)
	s.mu.Lock()
	s.sessions[token] = sess
	s.mu.Unlock()
	return token, nil
}

func (s *SessionStore) Get(token string) (Session, bool) {
	s.mu.RLock()
	sess, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok {
		return Session{}, false
	}
	if time.Now().After(sess.Expiry) {
		s.Delete(token)
		return Session{}, false
	}
	return sess, true
}

func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// Remap rewrites the Username of every stored session using fn, which returns
// the new username and whether it changed. It is used by the identifier
// migration so admins who are already signed in (e.g. via SAML) are not logged
// out when their map-admin record is re-keyed: permLevel() looks the user up by
// session Username, so the in-memory sessions must follow the rename.
func (s *SessionStore) Remap(fn func(username string) (string, bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, sess := range s.sessions {
		if newName, ok := fn(sess.Username); ok && newName != sess.Username {
			sess.Username = newName
			s.sessions[token] = sess
		}
	}
}

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
