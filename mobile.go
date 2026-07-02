package main

import (
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// mobileUARegex matches the same device families the legacy client-side
// detectMobile() checked, EXCEPT iPad: tablets keep the full desktop site (they
// can still open /m/ manually). Phones are auto-redirected to the mobile UI.
var mobileUARegex = regexp.MustCompile(`(?i)(android.+mobile|iphone|ipod|blackberry|opera mini|iemobile|wpdesktop)`)

// isMobileUA reports whether the request's User-Agent looks like a phone.
func isMobileUA(r *http.Request) bool {
	return mobileUARegex.MatchString(r.Header.Get("User-Agent"))
}

// wantsDesktop reports whether the visitor opted out of the mobile UI via the
// "Full site" escape link (which sets force_desktop=1).
func wantsDesktop(r *http.Request) bool {
	c, err := r.Cookie("force_desktop")
	return err == nil && c.Value == "1"
}

// mobileBootstrap is the JSON payload injected into the mobile shell so the SPA
// can render the first view without an extra round trip.
type mobileBootstrap struct {
	LoggedIn           bool           `json:"loggedIn"`
	User               string         `json:"user"`
	Fullname           string         `json:"fullname"`
	Mail               string         `json:"mail"`
	Phone              string         `json:"phone"`
	AvatarURL          string         `json:"avatarURL"`
	Perms              map[string]int `json:"perms"`
	SAMLEnabled        bool           `json:"samlEnabled"`
	AllowLocalFallback bool           `json:"allowLocalFallback"`
	DefaultMap         string         `json:"defaultMap"`
	AppTitle           string         `json:"appTitle"`
}

// handleMobile renders the separate, touch-first mobile shell for any /m/ path.
// View routing happens client-side (hash based), so every sub-path returns the
// same shell; the injected bootstrap tells the SPA whether the user is logged in
// and which admin sections they may view.
func (app *App) handleMobile(w http.ResponseWriter, r *http.Request) {
	// Not configured yet: fall back to the normal first-run flow.
	if !app.db.IsConfigured() {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	sess, ok := app.currentSession(r)

	bs := mobileBootstrap{
		LoggedIn:           ok,
		AppTitle:           app.appTitle(),
		AvatarURL:          "/images/noavatar2.png",
		Perms:              map[string]int{},
		SAMLEnabled:        app.cfg.SAML.Enabled,
		AllowLocalFallback: app.cfg.SAML.AllowLocalPasswordFallback,
		DefaultMap:         "overview",
	}

	if ok {
		bs.User = sess.Username
		bs.Fullname = sess.Fullname
		bs.Mail = sess.Mail
		bs.Phone = sess.Phone
		for _, f := range []string{"health", "stats", "auditlog", "users", "teams", "maps", "config", "ldap", "desks", "adminpanel"} {
			bs.Perms[f] = app.permLevel(sess, f)
		}
		// Avatar lookup mirrors the desktop index page.
		userid := sess.Username
		if i := strings.LastIndex(userid, "\\"); i >= 0 {
			userid = userid[i+1:]
		}
		if _, err := os.Stat(app.cfg.dataPath("avatarcache", userid+".jpg")); err == nil {
			bs.AvatarURL = "/avatarcache/" + userid + ".jpg?time=" + strconv.FormatInt(time.Now().Unix(), 10)
		}
		if c, err := r.Cookie("map"); err == nil && c.Value != "" {
			bs.DefaultMap = strings.ToLower(c.Value)
		}
		app.db.AddVisit()
	}

	payload, _ := json.Marshal(bs)
	data := map[string]interface{}{
		"AppTitle":  app.appTitle(),
		"Bootstrap": template.JS(payload),
	}
	w.Header().Set("Cache-Control", "no-store")
	if err := app.tmpl.ExecuteTemplate(w, "mobile.html", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// handleMobileServiceWorker serves the PWA service worker at /m/sw.js. Serving
// it under /m/ (rather than /static/) lets it control the whole /m/ scope
// without a Service-Worker-Allowed override. The asset-version placeholder is
// substituted so each deploy yields new bytes (triggering an SW update) and a
// fresh cache name.
func (app *App) handleMobileServiceWorker(w http.ResponseWriter, r *http.Request) {
	b, err := fs.ReadFile(app.staticFS, "m/sw.js")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	body := strings.ReplaceAll(string(b), "__ASSET_VERSION__", assetVersion)
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	// Let the SW control the whole /m/ scope even though the app is a subtree.
	w.Header().Set("Service-Worker-Allowed", "/m/")
	// The browser revalidates the SW script on each load; no-cache keeps updates
	// prompt while still allowing a conditional 304.
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "sw.js", time.Time{}, strings.NewReader(body))
}

// handleMobileManifest serves the web app manifest at /m/manifest.webmanifest.
// It is generated so the installed app name tracks the configured app title.
func (app *App) handleMobileManifest(w http.ResponseWriter, r *http.Request) {
	title := app.appTitle()
	short := title
	if len(short) > 12 {
		short = short[:12]
	}
	manifest := map[string]interface{}{
		"id":               "/m/",
		"name":             title,
		"short_name":       short,
		"start_url":        "/m/",
		"scope":            "/m/",
		"display":          "standalone",
		"orientation":      "portrait",
		"background_color": "#1b2a4a",
		"theme_color":      "#0979D8",
		"icons": []map[string]string{
			{"src": "/favicons/android-chrome-192x192.png", "sizes": "192x192", "type": "image/png", "purpose": "any"},
			{"src": "/favicons/android-chrome-512x512.png", "sizes": "512x512", "type": "image/png", "purpose": "any"},
			{"src": "/favicons/android-chrome-512x512.png", "sizes": "512x512", "type": "image/png", "purpose": "maskable"},
		},
	}
	w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_ = json.NewEncoder(w).Encode(manifest)
}
