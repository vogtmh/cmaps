package web

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// handleIndex serves the main client UI (or the setup wizard on first run).
func (app *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Any non-root path is treated as a request for an embedded static asset
	// (the legacy front-end references assets with root-relative paths such as
	// "cmaps.css", "user.js", "images/add.png" and "tools/jquery.js").
	if r.URL.Path != "/" {
		app.serveStaticAsset(w, r)
		return
	}

	// "Full site" escape link from the mobile UI: remember the opt-out so this
	// phone keeps the desktop layout, then drop back to a clean URL.
	if r.Method == http.MethodGet && r.URL.Query().Get("desktop") == "1" {
		http.SetCookie(w, &http.Cookie{
			Name:     "force_desktop",
			Value:    "1",
			Path:     "/",
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().AddDate(1, 0, 0),
		})
		http.Redirect(w, r, "/", http.StatusSeeOther)
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

	// Persist the Extras settings panel (POST from index.html). Checkbox values
	// are only submitted when checked, so each setting cookie is written
	// explicitly ("1" on / "0" off), then we redirect (PRG) to avoid a resubmit.
	if r.Method == http.MethodPost && r.FormValue("applysettings") == "1" {
		_ = r.ParseForm()
		settings := []string{
			"setting_nodescription",
			"setting_printmode",
			"setting_desknumbers",
			"setting_shownames",
			"setting_highlightleaders",
			"setting_noanimation",
			"setting_dailyvisitors",
			"setting_saml",
		}
		for _, name := range settings {
			val := "0"
			if r.FormValue(name) == "1" {
				val = "1"
			}
			http.SetCookie(w, &http.Cookie{
				Name:     name,
				Value:    val,
				Path:     "/",
				SameSite: http.SameSiteLaxMode,
				Expires:  time.Now().AddDate(5, 0, 0),
			})
		}
		dest := "/"
		if m := r.FormValue("map"); m != "" {
			dest = "/?map=" + url.QueryEscape(m)
		}
		http.Redirect(w, r, dest, http.StatusSeeOther)
		return
	}

	if ok {
		app.db.AddVisit()
	}

	// Phone visitors get the dedicated mobile UI unless they opted out via the
	// "Full site" link (force_desktop=1). Reached only once setup is complete.
	if r.Method == http.MethodGet && isMobileUA(r) && !wantsDesktop(r) {
		http.Redirect(w, r, "/m/", http.StatusSeeOther)
		return
	}

	app.renderIndex(w, r, sess, ok)
}

// serveStaticAsset serves a file from the embedded static FS using the cleaned
// request path. Returns 404 when the asset does not exist.
func (app *Server) serveStaticAsset(w http.ResponseWriter, r *http.Request) {
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

// serveAvatar serves a cached employee avatar from the data directory. Many
// employees have no avatar file; instead of returning a 404 (which the browser
// logs as a console error for every missing picture) we redirect to a single
// shared placeholder URL. That URL's bytes are cached once and reused for every
// missing avatar, avoiding hundreds of duplicate cache entries.
func (app *Server) serveAvatar(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/avatarcache/")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	if name != "" && !strings.HasPrefix(name, "..") && !strings.ContainsAny(name, "/\\") {
		if f, err := os.Open(filepath.Join(app.cfg.DataPath("avatarcache"), name)); err == nil {
			defer f.Close()
			if info, err := f.Stat(); err == nil && !info.IsDir() {
				http.ServeContent(w, r, info.Name(), info.ModTime(), f)
				return
			}
		}
	}
	// Missing → point everyone at the same cached placeholder.
	http.Redirect(w, r, "/static/images/noavatar2.png?v="+assetVersion, http.StatusFound)
}

// handleChanges renders the avatar/LDAP change-overview page (legacy changes.php).
func (app *Server) handleChanges(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.currentSession(r); !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	if err := app.tmpl.ExecuteTemplate(w, "changes.html", map[string]string{"AppTitle": app.appTitle()}); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}
