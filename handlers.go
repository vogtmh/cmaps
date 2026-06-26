package main

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// writeJSON writes v as a JSON response.
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// clientIP returns the best-effort client IP, honoring X-Forwarded-For for use
// behind the nginx reverse proxy.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// appTitle returns the configured application title (defaults to "CompanyMaps").
func (app *App) appTitle() string {
	if t := app.db.GetSetting("apptitle"); t != "" {
		return t
	}
	return "CompanyMaps"
}

// registerRESTRoutes wires up the REST API. Both the bare and trailing-slash
// forms are registered so the legacy front-end URLs (rest/desks/) keep working.
func (app *App) registerRESTRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/rest/account/", app.handleRestAccount)

	rest := func(path string, h http.HandlerFunc) {
		mux.HandleFunc(path, h)
		mux.HandleFunc(path+"/", h)
	}
	rest("/rest/desks", app.handleRestDesks)
	rest("/rest/users", app.handleRestUsers)
	rest("/rest/config", app.handleRestConfig)
	rest("/rest/teams", app.handleRestTeams)
	rest("/rest/booking", app.handleRestBooking)
	rest("/rest/changes", app.handleRestChanges)
	rest("/rest/stats", app.handleRestStats)
	rest("/rest/avatar", app.handleRestAvatar)
	rest("/rest/update", app.handleRestUpdate)
	rest("/rest/meeting", app.handleRestMeeting)
	rest("/rest/system", app.handleRestSystem)
	rest("/rest/ldap", app.handleRestLdap)
	rest("/rest/ldap/debug", app.handleRestLdapDebug)
	rest("/rest/ldap/sync", app.handleRestLdapSync)
	rest("/rest/ldap/progress", app.handleRestLdapProgress)
	rest("/rest/directory/search", app.handleRestDirectorySearch)
	rest("/rest/directory/match", app.handleRestDirectoryMatch)
	rest("/rest/vips", app.handleRestVips)
	rest("/rest/setting", app.handleRestSetting)
	rest("/rest/robin/test", app.handleRestRobinTest)
	rest("/rest/robin/sync", app.handleRestRobinSync)
	rest("/rest/robin/progress", app.handleRestRobinProgress)
}

// handleIndex serves the main client UI (or the setup wizard on first run).
func (app *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Any non-root path is treated as a request for an embedded static asset
	// (the legacy front-end references assets with root-relative paths such as
	// "cmaps80.css", "user80.js", "images/add.png" and "tools/jquery3.js").
	if r.URL.Path != "/" {
		app.serveStaticAsset(w, r)
		return
	}

	sess, ok := app.currentSession(r)

	// Until initial setup is complete, only an authenticated admin may proceed
	// (to the setup wizard, added in Phase 2). Everyone else sees the login page.
	if !app.db.IsConfigured() {
		if !ok {
			app.renderLogin(w, "", "/")
			return
		}
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}

	if ok {
		app.db.TrackVisit(sess.Username)
	}

	app.renderIndex(w, r, sess, ok)
}

// serveStaticAsset serves a file from the embedded static FS using the cleaned
// request path. Returns 404 when the asset does not exist.
func (app *App) serveStaticAsset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" || strings.HasPrefix(name, "..") {
		http.NotFound(w, r)
		return
	}
	f, err := app.staticFS.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.NotFound(w, r)
		return
	}
	// Embedded files carry a zero modtime, so ServeContent emits no
	// Last-Modified/ETag and browsers refetch on every load. Set an explicit
	// Cache-Control so static assets (CSS/JS/images, the map background) are
	// cached. JS/CSS are versioned with ?v= so deploys still bust the cache.
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(w, r, info.Name(), info.ModTime(), rs)
}

// handleChanges renders the avatar/LDAP change-overview page (legacy changes.php).
func (app *App) handleChanges(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.currentSession(r); !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := app.tmpl.ExecuteTemplate(w, "changes.html", map[string]string{"AppTitle": app.appTitle()}); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// handleLogin renders the login form (GET) and authenticates local users (POST).
func (app *App) handleLogin(w http.ResponseWriter, r *http.Request) {
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
	app.db.AuditLog("login", sess.Username, "local login from "+clientIP(r))
	http.Redirect(w, r, next, http.StatusSeeOther)
}

// authenticateLocal validates the admin password from config.json or a local user
// stored in the database. AD/LDAP is used only for syncing, never for login.
func (app *App) authenticateLocal(username, password string) (Session, bool) {
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
	if !checkPassword(password, u.PassHash, u.Salt) {
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
func (app *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		app.sessions.Delete(c.Value)
	}
	app.clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// handleRestAccount handles /rest/account/?mode=logout|samllogin (login is via
// the /login form for local users; SAML via /auth/saml/login).
func (app *App) handleRestAccount(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, map[string]interface{}{
			"status":   "ok",
			"loggedin": ok,
			"user":     sess.Username,
			"fullname": sess.Fullname,
		})
	}
}

// --- rendering helpers ---

func (app *App) render(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := app.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (app *App) renderLogin(w http.ResponseWriter, errMsg, next string) {
	next = safeNextPath(next)
	samlLoginURL := "/auth/saml/login"
	if next != "/" {
		samlLoginURL += "?next=" + url.QueryEscape(next)
	}
	app.render(w, "login.html", map[string]interface{}{
		"AppTitle":     app.appTitle(),
		"SAMLEnabled":  app.cfg.SAML.Enabled,
		"Error":        errMsg,
		"Next":         next,
		"SAMLLoginURL": samlLoginURL,
	})
}

// safeNextPath returns a post-login redirect target that is guaranteed to be a
// local path, defending against open-redirect abuse via the "next" parameter. It
// only accepts paths that start with a single "/" (not "//", which browsers treat
// as a protocol-relative absolute URL) and contain no control characters.
func safeNextPath(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return "/"
	}
	if strings.ContainsAny(next, "\\\r\n\t") {
		return "/"
	}
	return next
}
