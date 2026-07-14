package web

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"companymaps/internal/auth"
)

// handleLogin renders the login form (GET) and authenticates local users (POST).
func (app *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		next := safeNextPath(r.URL.Query().Get("next"))
		// SAML-only (no local password fallback): skip the login form entirely and
		// initiate SSO straight away, carrying the return path through. The local
		// form remains reachable via ?local=1 as an escape hatch (e.g. to use the
		// config.json admin password if SSO is misconfigured).
		if app.cfg.SAML.Enabled && !app.cfg.SAML.AllowLocalPasswordFallback && r.URL.Query().Get("local") != "1" {
			samlURL := "/auth/saml/login"
			if next != "/" {
				samlURL += "?next=" + url.QueryEscape(next)
			}
			http.Redirect(w, r, samlURL, http.StatusSeeOther)
			return
		}
		app.renderLogin(w, "", next)
		return
	}

	next := safeNextPath(r.FormValue("next"))
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	sess, ok := app.authenticateLocal(username, password)
	if !ok {
		time.Sleep(2 * time.Second) // throttle brute force, matching the PHP delay
		app.db.AuditLog("login", username, "failed local login from "+clientIP(r))
		app.renderLogin(w, "Invalid username or password.", next)
		return
	}

	token, err := app.sessions.Create(sess)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	app.setSessionCookie(w, token)
	app.resetUsermodeCookie(w)
	app.db.AuditLog("login", sess.Username, "local login from "+clientIP(r))
	http.Redirect(w, r, next, http.StatusSeeOther)
}

// authenticateLocal validates the admin password from config.json or a local user
// stored in the database. AD/LDAP is used only for syncing, never for login.
func (app *Server) authenticateLocal(username, password string) (Session, bool) {
	if username == "" || password == "" {
		return Session{}, false
	}

	// Break-glass local admin from config.json.
	if username == "admin" && password == app.cfg.AdminPassword {
		return Session{
			AdminPassword:  true,
			Username:       "admin",
			Samaccountname: "admin",
			Fullname:       "Administrator",
		}, true
	}

	u, found, err := app.db.GetUser(username)
	if err != nil || !found || !u.IsLocal || u.PassHash == "" {
		return Session{}, false
	}
	if !auth.CheckPassword(password, u.PassHash, u.Salt) {
		return Session{}, false
	}
	u.LastLogin = time.Now().Format("2006-01-02 15:04:05")
	_ = app.db.PutUser(u)
	return Session{
		Username:       u.Username,
		Samaccountname: u.Username,
		Fullname:       u.Fullname,
		Mail:           u.Mail,
	}, true
}

// handleLogout terminates the session.
func (app *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		app.sessions.Delete(c.Value)
	}
	app.clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// handleRestAccount handles /rest/account/?mode=logout|samllogin (login is via
// the /login form for local users; SAML via /auth/saml/login).
func (app *Server) handleRestAccount(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get("mode") {
	case "login":
		username := strings.TrimSpace(r.FormValue("user"))
		if username == "" {
			username = strings.TrimSpace(r.FormValue("username"))
		}
		password := r.FormValue("password")
		sess, ok := app.authenticateLocal(username, password)
		if !ok {
			time.Sleep(2 * time.Second) // throttle brute force, matching the PHP delay
			app.db.AuditLog("login", username, "failed local login from "+clientIP(r))
			writeJSON(w, map[string]interface{}{"status": "error", "message": "Invalid username or password."})
			return
		}
		token, err := app.sessions.Create(sess)
		if err != nil {
			writeJSON(w, map[string]interface{}{"status": "error", "message": "Session error."})
			return
		}
		app.setSessionCookie(w, token)
		app.resetUsermodeCookie(w)
		app.db.AuditLog("login", sess.Username, "local login from "+clientIP(r))
		writeJSON(w, map[string]interface{}{"status": "ok", "message": "Login successful."})
	case "logout":
		if c, err := r.Cookie(sessionCookie); err == nil {
			app.sessions.Delete(c.Value)
		}
		app.clearSessionCookie(w)
		writeJSON(w, map[string]interface{}{"status": "ok", "message": "logged out"})
	case "samllogin":
		http.Redirect(w, r, "/auth/saml/login", http.StatusSeeOther)
	default:
		sess, ok := app.currentSession(r)

		// Mirror the legacy PHP behaviour: a browser navigation to /rest/account/
		// (e.g. the Entra MyApps tile) should land the user in the app, initiating
		// SSO first if they are not yet authenticated. Only AJAX/JSON callers get
		// the status payload.
		if strings.Contains(r.Header.Get("Accept"), "text/html") {
			switch {
			case ok:
				http.Redirect(w, r, "/", http.StatusSeeOther)
			case app.cfg.SAML.Enabled:
				http.Redirect(w, r, "/auth/saml/login", http.StatusSeeOther)
			default:
				http.Redirect(w, r, "/login", http.StatusSeeOther)
			}
			return
		}

		writeJSON(w, map[string]interface{}{
			"status":   "ok",
			"loggedin": ok,
			"user":     sess.Username,
			"fullname": sess.Fullname,
		})
	}
}
