package main

import (
	"path/filepath"
	"testing"

	"companymaps/internal/store"
)

// newTestApp builds a minimal App backed by a temp database, sufficient for
// permission checks.
func newTestApp(t *testing.T) *App {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &App{
		db:       db,
		sessions: NewSessionStore(),
	}
}

func TestPermLevelBreakGlassAdmin(t *testing.T) {
	app := newTestApp(t)
	sess := Session{AdminPassword: true, Username: "admin"}
	for _, feature := range []string{"desks", "ldap", "config", "unknown-feature"} {
		if got := app.permLevel(sess, feature); got != 2 {
			t.Errorf("break-glass admin permLevel(%q) = %d, want 2", feature, got)
		}
	}
}

func TestPermLevelAnonymous(t *testing.T) {
	app := newTestApp(t)
	if got := app.permLevel(Session{}, "desks"); got != 0 {
		t.Errorf("empty session permLevel = %d, want 0", got)
	}
}

func TestPermLevelUnknownUser(t *testing.T) {
	app := newTestApp(t)
	sess := Session{Username: "ghost"}
	if got := app.permLevel(sess, "desks"); got != 0 {
		t.Errorf("unknown user permLevel = %d, want 0", got)
	}
}

func TestPermLevelRoleMatrix(t *testing.T) {
	app := newTestApp(t)
	if err := app.db.PutRole(Role{ID: 5, Rolename: "editor", Perms: map[string]int{
		"desks": 2,
		"maps":  1,
	}}); err != nil {
		t.Fatalf("PutRole: %v", err)
	}
	if err := app.db.PutUser(User{Username: "jdoe", Role: 5}); err != nil {
		t.Fatalf("PutUser: %v", err)
	}

	sess := Session{Username: "jdoe"}
	cases := []struct {
		feature string
		want    int
	}{
		{"desks", 2},  // write granted
		{"maps", 1},   // read granted
		{"config", 0}, // absent from Perms map -> zero value
	}
	for _, c := range cases {
		if got := app.permLevel(sess, c.feature); got != c.want {
			t.Errorf("permLevel(%q) = %d, want %d", c.feature, got, c.want)
		}
	}
}

func TestPermLevelUserWithMissingRole(t *testing.T) {
	app := newTestApp(t)
	if err := app.db.PutUser(User{Username: "orphan", Role: 99}); err != nil {
		t.Fatalf("PutUser: %v", err)
	}
	if got := app.permLevel(Session{Username: "orphan"}, "desks"); got != 0 {
		t.Errorf("user with missing role permLevel = %d, want 0", got)
	}
}

func TestIsEditor(t *testing.T) {
	app := newTestApp(t)
	if err := app.db.PutRole(Role{ID: 1, Rolename: "viewer", Perms: map[string]int{"desks": 1}}); err != nil {
		t.Fatalf("PutRole: %v", err)
	}
	if err := app.db.PutUser(User{Username: "viewer1", Role: 1}); err != nil {
		t.Fatalf("PutUser: %v", err)
	}
	if app.isEditor(Session{Username: "viewer1"}) {
		t.Errorf("read-only user reported as editor")
	}
	if !app.isEditor(Session{AdminPassword: true}) {
		t.Errorf("break-glass admin not reported as editor")
	}
}
