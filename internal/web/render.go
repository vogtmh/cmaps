package web

import (
	"net/http"
	"net/url"
	"strings"
)

// appTitle returns the configured application title (defaults to "CompanyMaps").
func (app *Server) appTitle() string {
	if t := app.db.GetSetting("apptitle"); t != "" {
		return t
	}
	return "CompanyMaps"
}

// render executes a named template as an HTML page with no-cache headers so the
// versioned (?v=) asset URLs are always revalidated after a deploy.
func (app *Server) render(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// HTML pages must always be revalidated so the freshly versioned (?v=)
	// asset URLs are picked up after a deploy. Without this, browsers (notably
	// Safari) heuristically cache the page and keep referencing stale JS/CSS.
	w.Header().Set("Cache-Control", "no-cache")
	if err := app.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (app *Server) renderLogin(w http.ResponseWriter, errMsg, next string) {
	next = safeNextPath(next)
	app.render(w, "login.html", map[string]interface{}{
		"AppTitle":    app.appTitle(),
		"SAMLEnabled": app.cfg.SAML.Enabled,
		"Error":       errMsg,
		"Next":        next,
		"LogoRegular": app.settingOr("logo_regular", "/static/images/cmaps-regular.png"),
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
	// Strip the AJAX fragment flag: a next path captured from a background
	// tab-switch request (e.g. /admin/?tab=config&partial=1) would otherwise
	// render only the bare content fragment after login and look broken. Force
	// a full-page render by removing the partial marker.
	if u, err := url.Parse(next); err == nil && u.Query().Has("partial") {
		q := u.Query()
		q.Del("partial")
		u.RawQuery = q.Encode()
		next = u.String()
	}
	return next
}
