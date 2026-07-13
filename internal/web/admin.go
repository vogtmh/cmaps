package web

import (
	"companymaps/internal/integrations"
	"companymaps/internal/integrations/robin"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

// mapRow is a maps-tab table row with derived existence indicators.
type mapRow struct {
	MapInfo
	HasFile      bool
	HasDB        bool
	HasFlag      bool
	AddressClean string // Address with <br/> stripped, for display/editing.
}

// adminUserRow is a users-tab row (a config_mapadmins entry).
type adminUserRow struct {
	Username string
	Name     string
	Mail     string
	Role     int
	RoleName string
}

type kv struct {
	Variable    string
	Value       string
	Description string
}

// RobinAdOverlap is one desk where the AD mirror and the cached Robin occupancy
// name the same person at the same desk. The AD placement keeps priority, so the
// Robin overlay for this desk is dropped (see sameRobinPerson in desks.go).
type RobinAdOverlap struct {
	Map        string
	Desknumber string
	Name       string
	Userid     string
}

// RobinAdDuplicate is one person the AD mirror seats at one desk while Robin
// seats them at a different desk on the same map, which makes them appear twice
// on that map. Rendered reflects whether the unified priority engine actually
// shows the Robin desk right now, i.e. whether the duplicate is visible.
type RobinAdDuplicate struct {
	Map       string
	Name      string
	Userid    string
	AdDesk    string
	RobinDesk string
	Rendered  bool
}

// EntraLdapRow is one person in the LDAP vs EntraID mirror comparison. Users are
// matched between the two mirrors by e-mail (case-insensitive). LdapDesks and
// EntraDesks list each side's office/desk placements for that person.
type EntraLdapRow struct {
	Name       string
	Mail       string
	LdapDesks  string
	EntraDesks string
}

// settingDescriptions maps general settings to a short explanation shown as a
// subtitle under each variable name in the admin "base variables" list.
var settingDescriptions = map[string]string{
	"apptitle":     "Application name shown in the browser tab and the top bar.",
	"domain":       "Default domain appended to usernames for login and Teams/SSO matching.",
	"reportURL":    "Link target for the \u201cReport a problem\u201d button. Leave empty to hide it.",
	"teamsContact": "Microsoft Teams user (email) opened when contacting support from an announcement.",
	"nomapText":    "Placeholder text for maps without an image. Empty uses \u201cThis map has not been added yet.\u201d Supports multiple lines.",
	"nomapLink":    "Optional link shown right below the placeholder text on maps without an image.",
}

// adminData holds everything the admin.html template needs.
type adminData struct {
	AppTitle          string
	TargetScreenWidth int
	HalfWidth         int
	Autozoom          int
	Zoom              int
	ContentScale      string
	ContentLeft       string // LeftPos / ContentScale (for the zoom-based layout)
	ContentTop        string // TopHeader / ContentScale (for the zoom-based layout)
	LeftPos           int
	TopHeader         int

	ActiveTab string
	SyncSub   string
	Username  string
	IsEditor  bool
	Token     string
	Message   string

	PermHealth     int
	PermConfig     int
	PermLdap       int
	PermMaps       int
	PermDesks      int
	PermUsers      int
	PermTeams      int
	PermStats      int
	PermAuditlog   int
	PermAdminpanel int

	GeneralVars []kv
	LogoRegular string
	LogoHover   string
	LdapSources []LdapSource
	// UnifiedSources is the ordered, priority-ranked list of every configured
	// directory source (each LDAP/EntraID config plus Robin) shown in Sync >
	// General, where the admin reorders priority and toggles assign / keep-dupes.
	UnifiedSources          []UnifiedSource
	RobinSpaces             []RobinSpace
	RobinMapOptions         []string
	RobinOrg                string
	RobinSet                bool
	RobinEnabled            bool
	GeoapifySet             bool
	GeoEnabled              bool
	GeoUsageMonth           string
	GeoUsageCount           int
	NextLdapSync            string
	NextEntraSync           string
	NextRobinSync           string
	RobinLastSync           robin.RobinSyncResult
	RobinHasSync            bool
	RobinDeskMode           string
	RobinStripPrefixEnabled bool
	RobinStripPrefixList    string
	RobinStripSuffixEnabled bool
	RobinStripSuffixList    string
	RobinLastDiscovery      string
	RobinUnmapped           int
	RobinDeskReservations   []RobinDeskStatus
	RobinDeskHasSync        bool
	RobinDeskLastSyncTime   string
	RobinDeskCount          int
	RobinAdSameDesk         []RobinAdOverlap
	RobinAdDuplicates       []RobinAdDuplicate
	EntraSources            []EntraSource
	EntraSet                bool
	EntraLastSync           string
	EntraHasSync            bool
	EntraCount              int
	EntraMatchedSame        []EntraLdapRow
	EntraMatchedDiff        []EntraLdapRow
	EntraOnlyLdap           []EntraLdapRow
	EntraOnlyEntra          []EntraLdapRow
	Maps                    []mapRow
	DeskMaps                []string
	Mapadmins               []adminUserRow
	Roles                   []Role
	Teams                   []Team
	CustomTypes             []CustomItemType
	AuditEntries            []AuditEntry
	AuditFilter             string
	AuditTypes              []string
	Countryflags            []string
	Timezones               []string
	DepartmentsJSON         template.JS
	BackupGroups            []backupGroup
	WorldMap                bool
	InternalBooking         bool
	IdentifierMode          string
	GeoapifyConfigured      bool
	// HasRealSource is true when a genuine directory source (non-demo LDAP,
	// EntraID or Robin) is configured. When false, the Sync tab offers a "Demo
	// data" subtab to create the built-in demo source. DemoSourceExists tracks
	// whether the demo source is already present (button says "recreate").
	HasRealSource    bool
	DemoSourceExists bool
}

// commonTimezones is the curated timezone list offered when creating a map. The
// legacy panel generated the full IANA list; this covers the common cases and a
// free-text fallback is accepted by the create handler.
var commonTimezones = []string{
	"Europe/Berlin", "Europe/London", "Europe/Paris", "Europe/Madrid", "Europe/Rome",
	"Europe/Moscow", "Europe/Istanbul", "America/New_York", "America/Chicago",
	"America/Denver", "America/Los_Angeles", "America/Sao_Paulo", "America/Toronto",
	"Asia/Dubai", "Asia/Kolkata", "Asia/Shanghai", "Asia/Tokyo", "Asia/Singapore",
	"Australia/Sydney", "Pacific/Auckland", "Africa/Johannesburg",
}

func (app *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	// Static assets that live under /admin/ (backend.js, admin.css) are
	// served from the embedded static FS; only /admin and /admin/ render the panel.
	if r.URL.Path != "/admin" && r.URL.Path != "/admin/" {
		app.serveStaticAsset(w, r)
		return
	}

	sess, ok := app.currentSession(r)
	if !ok {
		// Not logged in (e.g. the session was lost on a server restart): send the
		// user to the login page and remember where they were headed so they land
		// back on the admin panel afterwards instead of the map.
		http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
		return
	}
	if app.permLevel(sess, "adminpanel") < 1 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	msg := ""
	if r.Method == http.MethodPost {
		msg = app.handleAdminPost(w, r, sess)
		// Map creation with file upload renders inline; all other actions
		// fall through to a normal render of the requested tab.
	}

	tab := r.FormValue("tab")
	if tab == "" {
		tab = r.URL.Query().Get("tab")
	}
	if tab == "" {
		tab = "dashboard"
	}
	// SAML is now a subtab of the merged "Sync" tab (ActiveTab "ldap"). Keep the
	// legacy ?tab=saml link working by aliasing it to the SAML subtab.
	syncSub := r.FormValue("sub")
	if syncSub == "" {
		syncSub = r.URL.Query().Get("sub")
	}
	if tab == "saml" {
		tab = "ldap"
		if syncSub == "" {
			syncSub = "saml"
		}
	}
	// Custom item types now live on the Config tab; alias the legacy tab link so
	// old bookmarks and posts land on the Config tab.
	if tab == "itemtypes" {
		tab = "config"
	}
	// The Health tab has been merged into the Dashboard; alias the legacy link.
	if tab == "health" {
		tab = "dashboard"
	}
	// Fall back to dashboard if the user lacks permission for the tab. The Sync
	// tab (ldap) is accessible with either the "ldap" permission (LDAP/Robin
	// subtabs) or the "adminpanel" permission (SAML subtab).
	if tab != "dashboard" {
		allowed := app.permLevel(sess, tab) > 0
		if tab == "ldap" && app.permLevel(sess, "adminpanel") > 0 {
			allowed = true
		}
		// The custom item-types tab reuses the Desks permission.
		if tab == "itemtypes" && app.permLevel(sess, "desks") > 0 {
			allowed = true
		}
		if !allowed {
			tab = "dashboard"
		}
	}

	data := app.buildAdminData(r, sess, tab, msg)
	data.SyncSub = syncSub
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	// AJAX tab switches and form submits request ?partial=1 and receive only the
	// content fragment (the "admincontent" block), which the client swaps into
	// #content without a full page reload.
	tmplName := "admin.html"
	if r.URL.Query().Get("partial") == "1" {
		tmplName = "admincontent"
	}
	if err := app.tmpl.ExecuteTemplate(w, tmplName, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// handleAdminPost processes a single admin form submission and returns a status
// handleAdminPost dispatches an admin-panel POST to the per-feature handler
// for the submitting tab and returns a status message for re-rendering. Every
// handler re-checks its own permission, so a crafted tab value cannot bypass
// the feature gates.
func (app *Server) handleAdminPost(w http.ResponseWriter, r *http.Request, sess Session) string {
	_ = r.ParseMultipartForm(20 << 20)

	switch r.FormValue("tab") {
	case "dashboard":
		return app.handleAdminPostDashboard(r, sess)
	case "ldap":
		return app.handleAdminPostSync(r, sess)
	case "maps":
		return app.handleAdminPostMaps(r, sess)
	case "users":
		return app.handleAdminPostUsers(r, sess)
	case "teams":
		return app.handleAdminPostTeams(r, sess)
	case "itemtypes":
		return app.handleAdminPostItemTypes(r, sess)
	case "config":
		return app.handleAdminPostConfig(r, sess)
	}
	return ""
}

func (app *Server) flagExists(country string) bool {
	if country == "" {
		return false
	}
	_, err := fs.Stat(app.staticFS, "countryflags/"+country+".svg")
	return err == nil
}

func (app *Server) listCountryflags() []string {
	entries, err := fs.ReadDir(app.staticFS, "countryflags")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if i := strings.LastIndex(name, "."); i >= 0 {
			name = name[:i]
		}
		out = append(out, strings.ToLower(name))
	}
	sort.Strings(out)
	return out
}

type testCheck = integrations.Check

// testResult wraps a set of checks into the {ok, checks} payload the admin test
// modal renders. ok is false when any check failed.
func testResult(checks []testCheck) map[string]interface{} {
	return integrations.Result(checks)
}

// handleRestLdapTest validates a single LDAP source's connectivity and bind
// credentials without running a sync. It dials the server, binds and reports the
func orDefaultStr(v, def string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return def
	}
	return v
}

// removeFileIfExists deletes path, ignoring "not exists" errors.
func removeFileIfExists(path string) error {
	err := os.Remove(path)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

// nowTimestamp returns the current local time in the audit/sync format.
func nowTimestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// nextSyncLabel formats a scheduled next-sync time for the admin Sync tab. When
// the integration is disabled it returns a paused note; a zero time (scheduler
// not yet armed) returns an empty string.
func (app *Server) nextSyncLabel(t time.Time, enabled bool) string {
	if !enabled {
		return "Paused (disabled)"
	}
	if t.IsZero() {
		return ""
	}
	return t.In(app.db.Location()).Format("2006-01-02 15:04:05")
}
func stripBR(s string) string {
	return strings.NewReplacer("<br/>", "\n", "<br />", "\n", "<br>", "\n").Replace(s)
}

// checkboxValue normalizes an HTML checkbox form value to the "1"/"" form used
// for boolean settings (a checked box posts a non-empty value).
func checkboxValue(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	return "1"
}

// addBR normalizes a user-entered address and converts newlines to the stored
// <br/> form used by the client map plate.
func addBR(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSpace(s)
	return strings.ReplaceAll(s, "\n", "<br/>")
}

// updateMapFromForm edits an existing map. If the name changed it renames the
// map everywhere (DB records, desks, bookings, cached meeting status and the
