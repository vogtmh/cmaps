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
