// Package auth holds the authentication and authorization core: sessions,
// local-password hashing and the role/permission model.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Session holds the authenticated user's identity, mirroring the PHP $_SESSION
// values. AdminPassword is true when the user authenticated with the local
// admin password from config.json (the break-glass superadmin).
type Session struct {
	Expiry         time.Time
	AdminPassword  bool
	Samaccountname string
	Username       string // mapadmins key, e.g. "tvcorp\\INT001327" or "admin"
	Fullname       string
	Mail           string
	Phone          string
}

// SessionStore is an in-memory token -> Session map, safe for concurrent use.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]Session
}

// NewSessionStore returns an empty session store.
func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]Session)}
}

// Create stores the session under a fresh random token valid for 8 hours.
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

// Get returns the session for a token, evicting and rejecting expired ones.
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

// Delete removes a session token.
func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// Remap rewrites the Username of every stored session using fn, which returns
// the new username and whether it changed. It is used by the identifier
// migration so admins who are already signed in (e.g. via SAML) are not logged
// out when their map-admin record is re-keyed: permission checks look the user
// up by session Username, so the in-memory sessions must follow the rename.
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
