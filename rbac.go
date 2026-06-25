package main

import "net/http"

// permLevel returns the permission level (0=none, 1=read, 2=write) the session's
// user holds for a feature. The break-glass config.json admin always gets 2.
func (app *App) permLevel(sess Session, feature string) int {
	if sess.AdminPassword {
		return 2
	}
	if sess.Username == "" {
		return 0
	}
	u, ok, err := app.db.GetUser(sess.Username)
	if err != nil || !ok {
		return 0
	}
	role, ok, err := app.db.GetRole(u.Role)
	if err != nil || !ok {
		return 0
	}
	return role.Perms[feature]
}

// requirePerm wraps a handler, enforcing a minimum permission level on a feature.
func (app *App) requirePerm(feature string, min int, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := app.currentSession(r)
		if !ok {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		if app.permLevel(sess, feature) < min {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// adminPage wraps an admin HTML handler, redirecting unauthenticated users to the
// login page and rejecting users without admin-panel access.
func (app *App) adminPage(feature string, min int, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := app.currentSession(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if app.permLevel(sess, feature) < min {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// isEditor reports whether the session may edit desks (level >= 2).
func (app *App) isEditor(sess Session) bool {
	return app.permLevel(sess, "desks") >= 2
}
