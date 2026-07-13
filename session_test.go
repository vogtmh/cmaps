package main

import (
	"testing"
	"time"
)

func TestSessionStoreCreateGetDelete(t *testing.T) {
	s := NewSessionStore()

	token, err := s.Create(Session{Username: "jdoe", Fullname: "John Doe"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if token == "" {
		t.Fatalf("Create returned empty token")
	}

	sess, ok := s.Get(token)
	if !ok {
		t.Fatalf("Get did not find the freshly created session")
	}
	if sess.Username != "jdoe" || sess.Fullname != "John Doe" {
		t.Errorf("session fields mismatch: %+v", sess)
	}
	if time.Until(sess.Expiry) <= 7*time.Hour {
		t.Errorf("expiry not ~8h in the future: %v", sess.Expiry)
	}

	if _, ok := s.Get("nonexistent"); ok {
		t.Errorf("Get returned a session for an unknown token")
	}

	s.Delete(token)
	if _, ok := s.Get(token); ok {
		t.Errorf("session still retrievable after Delete")
	}
}

func TestSessionStoreUniqueTokens(t *testing.T) {
	s := NewSessionStore()
	t1, _ := s.Create(Session{Username: "a"})
	t2, _ := s.Create(Session{Username: "b"})
	if t1 == t2 {
		t.Errorf("two sessions received the same token")
	}
}

func TestSessionStoreExpiry(t *testing.T) {
	s := NewSessionStore()
	token, err := s.Create(Session{Username: "jdoe"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Force the session to be expired.
	s.mu.Lock()
	sess := s.sessions[token]
	sess.Expiry = time.Now().Add(-time.Minute)
	s.sessions[token] = sess
	s.mu.Unlock()

	if _, ok := s.Get(token); ok {
		t.Errorf("Get returned an expired session")
	}
	// Expired session must have been evicted.
	s.mu.RLock()
	_, still := s.sessions[token]
	s.mu.RUnlock()
	if still {
		t.Errorf("expired session was not evicted from the store")
	}
}

func TestSessionStoreRemap(t *testing.T) {
	s := NewSessionStore()
	tokOld, _ := s.Create(Session{Username: "corp\\alice"})
	tokKeep, _ := s.Create(Session{Username: "admin"})

	s.Remap(func(username string) (string, bool) {
		if username == "corp\\alice" {
			return "alice@example.com", true
		}
		return "", false
	})

	if sess, ok := s.Get(tokOld); !ok || sess.Username != "alice@example.com" {
		t.Errorf("Remap did not rewrite username: ok=%v sess=%+v", ok, sess)
	}
	if sess, ok := s.Get(tokKeep); !ok || sess.Username != "admin" {
		t.Errorf("Remap changed an unrelated session: ok=%v sess=%+v", ok, sess)
	}
}
