package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"image/png"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
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
	Role     int
	RoleName string
}

type kv struct {
	Variable string
	Value    string
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

	GeneralVars     []kv
	LogoRegular     string
	LogoHover       string
	LdapSources     []LdapSource
	RobinSpaces     []RobinSpace
	RobinMapOptions []string
	RobinOrg        string
	RobinSet        bool
	RobinLastSync   RobinSyncResult
	RobinHasSync    bool
	RobinDeskMode   string
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
	Maps            []mapRow
	DeskMaps        []string
	Mapadmins       []adminUserRow
	Roles           []Role
	Teams           []Team
	AuditEntries    []AuditEntry
	AuditFilter     string
	AuditTypes      []string
	Countryflags    []string
	Timezones       []string
	DepartmentsJSON template.JS
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

// handleAdmin renders the admin panel (GET) and processes form submissions (POST).
func (app *App) handleAdmin(w http.ResponseWriter, r *http.Request) {
	// Static assets that live under /admin/ (backend80.js, admin80.css) are
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
	// Fall back to dashboard if the user lacks permission for the tab. The Sync
	// tab (ldap) is accessible with either the "ldap" permission (LDAP/Robin
	// subtabs) or the "adminpanel" permission (SAML subtab).
	if tab != "dashboard" {
		allowed := app.permLevel(sess, tab) > 0
		if tab == "ldap" && app.permLevel(sess, "adminpanel") > 0 {
			allowed = true
		}
		if !allowed {
			tab = "dashboard"
		}
	}

	data := app.buildAdminData(r, sess, tab, msg)
	data.SyncSub = syncSub
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
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
// message. It performs the same CRUD the legacy admin/index.php POST blocks did.
func (app *App) handleAdminPost(w http.ResponseWriter, r *http.Request, sess Session) string {
	_ = r.ParseMultipartForm(20 << 20)
	tab := r.FormValue("tab")

	switch tab {
	case "health":
		name := r.FormValue("ignoreHealthName")
		typ := r.FormValue("ignoreHealthType")
		if name != "" && typ != "" {
			_ = app.db.AddWhitelist(WhitelistEntry{Type: typ, Text: name})
			return "Whitelist updated."
		}

	case "ldap":
		if app.permLevel(sess, "ldap") < 2 {
			return ""
		}
		// Robin / meeting-room management lives on the LDAP tab.
		if name := r.FormValue("deleteRobinSpace"); name != "" {
			_ = app.db.DeleteRobinSpace(name)
			_ = app.db.AuditLog("LDAP", sess.Username, "Robin space removed ("+name+")")
			return "Robin space removed."
		}
		if sn := r.FormValue("setRobinMapSpace"); sn != "" {
			spaces, _ := app.db.ListRobinSpaces()
			for _, s := range spaces {
				if s.Spacename == sn {
					s.Mapname = strings.ToLower(strings.TrimSpace(r.FormValue("robinMapname")))
					_ = app.db.PutRobinSpace(s)
					_ = app.db.AuditLog("LDAP", sess.Username, "Robin space map updated ("+sn+")")
					break
				}
			}
			return "Robin map updated."
		}
		if r.FormValue("saveRobin") != "" {
			if tok := strings.TrimSpace(r.FormValue("robintoken")); tok != "" {
				_ = app.db.SetRobinSetting("robintoken", tok)
			}
			_ = app.db.SetRobinSetting("robinOrganisation", strings.TrimSpace(r.FormValue("robinOrganisation")))
			_ = app.db.AuditLog("LDAP", sess.Username, "Robin credentials updated")
			return "Robin settings saved."
		}
		if r.FormValue("saveRobinOptions") != "" {
			mode := strings.TrimSpace(r.FormValue("robinDeskMode"))
			switch mode {
			case "sync", "all", "unused":
			default:
				mode = "off"
			}
			_ = app.db.SetRobinSetting("robinDeskMode", mode)
			_ = app.db.SetRobinSetting("robinStripPrefixEnabled", checkboxValue(r.FormValue("robinStripPrefixEnabled")))
			_ = app.db.SetRobinSetting("robinStripPrefixList", r.FormValue("robinStripPrefixList"))
			_ = app.db.SetRobinSetting("robinStripSuffixEnabled", checkboxValue(r.FormValue("robinStripSuffixEnabled")))
			_ = app.db.SetRobinSetting("robinStripSuffixList", r.FormValue("robinStripSuffixList"))
			_ = app.db.AuditLog("LDAP", sess.Username, "Robin options updated")
			return "Robin options saved."
		}
		if r.FormValue("discoverRobin") != "" {
			summary, err := app.reconcileRobinLocations()
			if err != nil {
				return "Discovery failed: " + err.Error()
			}
			_ = app.db.AuditLog("LDAP", sess.Username, "Robin locations discovered")
			return summary
		}
		if r.FormValue("runRobinSync") != "" {
			res := app.RunRobinSyncStructured()
			_ = app.db.AuditLog("LDAP", sess.Username, "Robin meeting sync run")
			if res.Note != "" {
				return res.Note
			}
			return fmt.Sprintf("Robin sync complete: %d of %d room(s) matched a meeting desk.", res.MatchedRooms, res.TotalRooms)
		}
		if sn := strings.TrimSpace(r.FormValue("robinSpacename")); sn != "" {
			id, err := strconv.Atoi(strings.TrimSpace(r.FormValue("robinSpaceid")))
			if err != nil {
				return "Error: Robin location id must be a number."
			}
			_ = app.db.PutRobinSpace(RobinSpace{
				Spacename: strings.ToLower(sn),
				Spaceid:   id,
				Mapname:   strings.ToLower(strings.TrimSpace(r.FormValue("robinMapname"))),
			})
			_ = app.db.AuditLog("LDAP", sess.Username, "Robin space created ("+sn+")")
			return "Robin space added."
		}
		if id := r.FormValue("deleteLdapID"); id != "" {
			if n, err := strconv.Atoi(id); err == nil {
				_ = app.db.DeleteLdapSource(n)
				_ = app.db.AuditLog("LDAP", sess.Username, "LDAP sync removed (id "+id+")")
				return "LDAP source removed."
			}
		}
		desc := r.FormValue("newLdapDescription")
		server := r.FormValue("newLdapServer")
		typ := r.FormValue("newLdapType")
		ou := r.FormValue("newLdapOU")
		user := r.FormValue("newLdapUser")
		pass := r.FormValue("newLdapPass")
		if desc != "" && server != "" && typ != "" && ou != "" && user != "" && pass != "" {
			_ = app.db.PutLdapSource(LdapSource{
				ID: app.nextLdapID(), Description: desc, Server: server, Type: typ,
				OU: ou, LdapUser: user, LdapPass: pass, LastSync: "never",
			})
			_ = app.db.AuditLog("LDAP", sess.Username, "New LDAP sync created ("+desc+")")
			return "LDAP source created."
		}

	case "maps":
		if app.permLevel(sess, "maps") < 2 {
			return ""
		}
		if id := r.FormValue("deleteMapID"); id != "" {
			name := r.FormValue("deleteMapname")
			_ = app.db.DeleteMap(name)
			// Remove the desks for that map and the map image.
			if desks, _ := app.db.ListDesks(name); desks != nil {
				for _, d := range desks {
					_ = app.db.DeleteDesk(name, d.ID)
				}
			}
			_ = removeFileIfExists(app.cfg.dataPath("maps", name+".png"))
			_ = app.db.AuditLog("Maps", sess.Username, "Map deleted ("+name+")")
			return "Map deleted."
		}
		if r.FormValue("editMapOrigName") != "" {
			return app.updateMapFromForm(r, sess)
		}
		return app.createMapFromForm(r, sess)

	case "users":
		if app.permLevel(sess, "users") < 2 {
			return ""
		}
		if del := r.FormValue("deleteUser"); del != "" {
			_ = app.db.DeleteUser(del)
			_ = app.db.AuditLog("Users", sess.Username, "Admin removed ("+del+")")
			return "User removed."
		}
		if chg := r.FormValue("changeRoleUser"); chg != "" {
			roleInt, _ := strconv.Atoi(r.FormValue("changeRole"))
			if existing, found, _ := app.db.GetUser(chg); found {
				existing.Role = roleInt
				_ = app.db.PutUser(existing)
				_ = app.db.AuditLog("Users", sess.Username, "Role changed ("+chg+", role "+r.FormValue("changeRole")+")")
				return "Role updated."
			}
			return ""
		}
		user := r.FormValue("newadminuser")
		role := r.FormValue("newadminrole")
		if user != "" && role != "" {
			roleInt, _ := strconv.Atoi(role)

			// Resolve the entered value against the directory so the admin can
			// type a display name (or samaccountname) and we save the proper
			// DOMAIN\samaccountname plus the full name automatically.
			username := strings.TrimSpace(user)
			fullname := strings.TrimSpace(r.FormValue("newadminname"))
			mail := ""
			if d, ok := app.resolveDirectoryEntry(user); ok {
				domain := app.db.GetSetting("domain")
				if domain != "" {
					username = domain + "\\" + d.Userid
				} else {
					username = d.Userid
				}
				fullname = d.DisplayName()
				mail = d.Mail
			}

			existing, found, _ := app.db.GetUser(username)
			if found {
				existing.Role = roleInt
				if fullname != "" {
					existing.Fullname = fullname
				}
				if mail != "" {
					existing.Mail = mail
				}
				_ = app.db.PutUser(existing)
			} else {
				_ = app.db.PutUser(User{Username: username, Role: roleInt, Fullname: fullname, Mail: mail})
			}
			_ = app.db.AuditLog("Users", sess.Username, "New admin created ("+username+", role "+role+")")
			return "User created."
		}

	case "teams":
		if app.permLevel(sess, "teams") < 2 {
			return ""
		}
		if del := r.FormValue("deleteTeam"); del != "" {
			_ = app.db.DeleteTeam(del)
			_ = app.db.AuditLog("Teams", sess.Username, "Team removed ("+del+")")
			return "Team removed."
		}
		if orig := r.FormValue("editTeamOrigName"); orig != "" {
			name := strings.TrimSpace(r.FormValue("editTeamName"))
			if name == "" {
				return ""
			}
			members := normalizeMembers(r.FormValue("editTeamMembers"))
			if name != orig {
				_ = app.db.DeleteTeam(orig)
			}
			_ = app.db.PutTeam(Team{Teamname: name, Members: members})
			_ = app.db.AuditLog("Teams", sess.Username, "Team updated ("+name+")")
			return "Team updated."
		}
		name := strings.TrimSpace(r.FormValue("newTeam"))
		if name != "" {
			members := normalizeMembers(r.FormValue("newMembers"))
			_ = app.db.PutTeam(Team{Teamname: name, Members: members})
			_ = app.db.AuditLog("Teams", sess.Username, "New team created ("+name+")")
			return "Team created."
		}

	case "config":
		if app.permLevel(sess, "config") < 2 {
			return ""
		}
		return app.saveLogosFromForm(r, sess)
	}
	return ""
}

// normalizeMembers turns a user-entered member list (comma- and/or pipe-separated,
// possibly with surrounding spaces) into the stored format: full names joined by
// "|" with no spaces around the pipes.
func normalizeMembers(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '|' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "|")
}

// saveLogosFromForm stores any uploaded logo images and points the matching
// settings (logo_regular / logo_hover) at the served file path.
func (app *App) saveLogosFromForm(r *http.Request, sess Session) string {
	uploads := []struct{ field, setting string }{
		{"logoRegular", "logo_regular"},
		{"logoHover", "logo_hover"},
	}
	saved := 0
	for _, u := range uploads {
		if r.MultipartForm == nil || len(r.MultipartForm.File[u.field]) == 0 {
			continue
		}
		if err := app.saveLogoImage(u.setting, r.MultipartForm.File[u.field][0]); err != nil {
			return "Error saving logo: " + err.Error()
		}
		_ = app.db.SetSetting(u.setting, "/logos/"+u.setting+".png")
		_ = app.db.AuditLog("Settings", sess.Username, "Logo updated ("+u.setting+")")
		saved++
	}
	if saved == 0 {
		return ""
	}
	return "Logo updated."
}

// saveLogoImage decodes an uploaded image and writes it as a PNG into the data
// directory's logos folder, named after the setting it backs.
func (app *App) saveLogoImage(name string, fh *multipart.FileHeader) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	img, _, err := image.Decode(src)
	if err != nil {
		return err
	}

	dst, err := os.Create(app.cfg.dataPath("logos", name+".png"))
	if err != nil {
		return err
	}
	defer dst.Close()
	return png.Encode(dst, img)
}

// createMapFromForm creates a new map (single-step: metadata + image upload).
func (app *App) createMapFromForm(r *http.Request, sess Session) string {
	name := strings.ToLower(strings.TrimSpace(r.FormValue("newMapName")))
	if name == "" {
		return ""
	}
	if _, found, _ := app.db.GetMap(name); found {
		return "Error: a map with that name already exists."
	}

	x, _ := strconv.Atoi(r.FormValue("newMapX"))
	y, _ := strconv.Atoi(r.FormValue("newMapY"))
	m := MapInfo{
		Mapname:     name,
		DisplayName: strings.TrimSpace(r.FormValue("newMapDisplayName")),
		Itemscale:   orDefaultStr(r.FormValue("newMapItemscale"), "1"),
		Published:   orDefaultStr(r.FormValue("newMapPublished"), "yes"),
		Country:     strings.ToLower(r.FormValue("newMapCountry")),
		Flagsize:    orDefaultStr(r.FormValue("newMapFlagsize"), "0"),
		Timezone:    orDefaultStr(r.FormValue("newMapTimezone"), "Europe/Berlin"),
		Address:     addBR(r.FormValue("newMapAddress")),
		MapX:        x,
		MapY:        y,
	}

	// Save the uploaded map image if present.
	if r.MultipartForm != nil && len(r.MultipartForm.File["image"]) > 0 {
		if err := app.saveMapImage(name, r.MultipartForm.File["image"][0]); err != nil {
			return "Error saving map image: " + err.Error()
		}
	}

	_ = app.db.PutMap(m)
	_ = app.db.AuditLog("Maps", sess.Username, "Map created ("+name+")")
	return "Map created."
}

// stripBR converts stored HTML line breaks back to plain newlines so the admin
// panel can display/edit a map address without literal <br/> markup.
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
// image file on disk), rejecting the change when the target name already exists.
func (app *App) updateMapFromForm(r *http.Request, sess Session) string {
	orig := strings.ToLower(strings.TrimSpace(r.FormValue("editMapOrigName")))
	if orig == "" {
		return ""
	}
	m, found, _ := app.db.GetMap(orig)
	if !found {
		return "Error: map not found."
	}
	newName := strings.ToLower(strings.TrimSpace(r.FormValue("editMapName")))
	if newName == "" {
		return "Error: map name cannot be empty."
	}

	// Apply the rename first so subsequent attribute writes target the new key.
	if newName != orig {
		if _, exists, _ := app.db.GetMap(newName); exists {
			return "Error: a map with that name already exists."
		}
		if err := app.db.RenameMap(orig, newName); err != nil {
			return "Error renaming map: " + err.Error()
		}
		oldPath := app.cfg.dataPath("maps", orig+".png")
		if _, err := os.Stat(oldPath); err == nil {
			_ = os.Rename(oldPath, app.cfg.dataPath("maps", newName+".png"))
		}
		if updated, ok, _ := app.db.GetMap(newName); ok {
			m = updated
		}
	}

	m.DisplayName = strings.TrimSpace(r.FormValue("editMapDisplayName"))
	m.Itemscale = orDefaultStr(r.FormValue("editMapItemscale"), "1")
	m.Published = orDefaultStr(r.FormValue("editMapPublished"), "yes")
	m.Country = strings.ToLower(r.FormValue("editMapCountry"))
	m.Flagsize = orDefaultStr(r.FormValue("editMapFlagsize"), "0")
	m.Timezone = orDefaultStr(r.FormValue("editMapTimezone"), "Europe/Berlin")
	m.Address = addBR(r.FormValue("editMapAddress"))
	if v := r.FormValue("editMapX"); v != "" {
		if x, err := strconv.Atoi(v); err == nil {
			m.MapX = x
		}
	}
	if v := r.FormValue("editMapY"); v != "" {
		if y, err := strconv.Atoi(v); err == nil {
			m.MapY = y
		}
	}

	// Replace the image only if a new one was uploaded.
	if r.MultipartForm != nil && len(r.MultipartForm.File["editImage"]) > 0 {
		if err := app.saveMapImage(m.Mapname, r.MultipartForm.File["editImage"][0]); err != nil {
			return "Error saving map image: " + err.Error()
		}
	}

	_ = app.db.PutMap(m)
	if newName != orig {
		_ = app.db.AuditLog("Maps", sess.Username, "Map renamed ("+orig+" -> "+newName+")")
	} else {
		_ = app.db.AuditLog("Maps", sess.Username, "Map updated ("+m.Mapname+")")
	}
	return "Map updated."
}

// nextLdapID returns max(existing IDs)+1.
func (app *App) nextLdapID() int {
	srcs, _ := app.db.ListLdapSources()
	max := 0
	for _, s := range srcs {
		if s.ID > max {
			max = s.ID
		}
	}
	return max + 1
}

// buildAdminData assembles the template payload for the active tab.
func (app *App) buildAdminData(r *http.Request, sess Session, tab, msg string) adminData {
	autozoom := cookieInt(r, "autozoom", 1)
	zoom := cookieInt(r, "zoom", 100)
	if zoom < 10 || zoom > 100 {
		zoom = 100
	}
	targetWidth := 1600

	// Content scale + zoom-based positioning, mirroring the index page: the admin
	// body is shown with CSS `zoom` (instead of transform:scale) so Chart.js and
	// other pointer-driven widgets map correctly (a CSS transform on an ancestor
	// breaks Chart.js hit-testing). Because `zoom` also scales an element's
	// left/top offsets, we pre-divide them here.
	leftPos := cookieInt(r, "LeftPos", 0)
	topHeader := 69 * autozoom
	contentScale := float64(zoom) / 100 * float64(autozoom)
	if contentScale <= 0 {
		contentScale = 1
	}

	d := adminData{
		AppTitle:          app.appTitle(),
		TargetScreenWidth: targetWidth,
		HalfWidth:         targetWidth / 2,
		Autozoom:          autozoom,
		Zoom:              zoom,
		ContentScale:      strconv.FormatFloat(contentScale, 'f', -1, 64),
		ContentLeft:       strconv.FormatFloat(float64(leftPos)/contentScale, 'f', -1, 64),
		ContentTop:        strconv.FormatFloat(float64(topHeader)/contentScale, 'f', -1, 64),
		LeftPos:           leftPos,
		TopHeader:         topHeader,
		ActiveTab:         tab,
		Username:          sess.Username,
		IsEditor:          app.permLevel(sess, "desks") > 1,
		Message:           msg,
		PermHealth:        app.permLevel(sess, "health"),
		PermConfig:        app.permLevel(sess, "config"),
		PermLdap:          app.permLevel(sess, "ldap"),
		PermMaps:          app.permLevel(sess, "maps"),
		PermDesks:         app.permLevel(sess, "desks"),
		PermUsers:         app.permLevel(sess, "users"),
		PermTeams:         app.permLevel(sess, "teams"),
		PermStats:         app.permLevel(sess, "stats"),
		PermAuditlog:      app.permLevel(sess, "auditlog"),
		PermAdminpanel:    app.permLevel(sess, "adminpanel"),
	}
	if d.IsEditor {
		d.Token = "1"
	}

	switch tab {
	case "config":
		settings, _ := app.db.AllSettings()
		for k, v := range settings {
			// Logos are managed by the logo selector above, so hide them here.
			if k == "logo_regular" || k == "logo_hover" {
				continue
			}
			d.GeneralVars = append(d.GeneralVars, kv{Variable: k, Value: v})
		}
		sort.Slice(d.GeneralVars, func(i, j int) bool { return d.GeneralVars[i].Variable < d.GeneralVars[j].Variable })
		d.LogoRegular = app.settingOr("logo_regular", "/static/images/cmaps-regular.png")
		d.LogoHover = app.settingOr("logo_hover", "/static/images/cmaps-hover.png")

	case "ldap":
		d.LdapSources, _ = app.db.ListLdapSources()
		d.RobinSpaces, _ = app.db.ListRobinSpaces()
		sort.Slice(d.RobinSpaces, func(i, j int) bool { return d.RobinSpaces[i].Spacename < d.RobinSpaces[j].Spacename })
		d.RobinOrg = app.db.GetRobinSetting("robinOrganisation")
		d.RobinSet = app.db.GetRobinSetting("robintoken") != ""
		// Build the map dropdown: published maps plus any value currently in use
		// (so every row's selection stays selectable even if it isn't a real map yet).
		mapSet := map[string]bool{}
		if maps, err := app.db.ListMaps(); err == nil {
			for _, m := range maps {
				if m.Published == "yes" && m.Mapname != "overview" && !strings.Contains(m.Mapname, "-nomap") {
					mapSet[m.Mapname] = true
				}
			}
		}
		for _, s := range d.RobinSpaces {
			mapSet[s.MapName()] = true
		}
		for name := range mapSet {
			d.RobinMapOptions = append(d.RobinMapOptions, name)
		}
		sort.Strings(d.RobinMapOptions)
		d.RobinLastSync, d.RobinHasSync = app.LastRobinSyncResult()
		d.RobinDeskMode = app.db.GetRobinSetting("robinDeskMode")
		if d.RobinDeskMode == "" {
			d.RobinDeskMode = "off"
		}
		d.RobinStripPrefixEnabled = app.db.GetRobinSetting("robinStripPrefixEnabled") == "1"
		d.RobinStripPrefixList = app.db.GetRobinSetting("robinStripPrefixList")
		d.RobinStripSuffixEnabled = app.db.GetRobinSetting("robinStripSuffixEnabled") == "1"
		d.RobinStripSuffixList = app.db.GetRobinSetting("robinStripSuffixList")
		d.RobinLastDiscovery = app.db.GetRobinSetting("robinLocLastDiscovery")
		for _, s := range d.RobinSpaces {
			if strings.TrimSpace(s.Mapname) == "" {
				d.RobinUnmapped++
			}
		}
		// Desk-reservation (people) overlay: the cached occupancy is the source of
		// truth shown on the map; surface it in the Sync tab too.
		d.RobinDeskReservations, _ = app.db.ListRobinDeskStatus("")
		sort.Slice(d.RobinDeskReservations, func(i, j int) bool {
			if d.RobinDeskReservations[i].Map != d.RobinDeskReservations[j].Map {
				return d.RobinDeskReservations[i].Map < d.RobinDeskReservations[j].Map
			}
			return d.RobinDeskReservations[i].Desknumber < d.RobinDeskReservations[j].Desknumber
		})
		if dr, ok := app.LastRobinDeskSyncResult(); ok {
			d.RobinDeskHasSync = true
			d.RobinDeskLastSyncTime = dr.Time
			d.RobinDeskCount = dr.Count
		}

	case "maps":
		maps, _ := app.db.ListMaps()
		sort.Slice(maps, func(i, j int) bool { return maps[i].Mapname < maps[j].Mapname })
		for _, m := range maps {
			row := mapRow{MapInfo: m}
			if _, err := os.Stat(app.cfg.dataPath("maps", m.Mapname+".png")); err == nil {
				row.HasFile = true
			}
			if m.Mapname == "overview" {
				row.HasDB = true
			} else if desks, _ := app.db.ListDesks(m.Mapname); len(desks) > 0 {
				row.HasDB = true
			}
			if m.Country == "none" || app.flagExists(m.Country) {
				row.HasFlag = true
			}
			row.AddressClean = stripBR(m.Address)
			d.Maps = append(d.Maps, row)
		}
		d.Countryflags = app.listCountryflags()
		d.Timezones = commonTimezones

	case "desks":
		maps, _ := app.db.ListMaps()
		sort.Slice(maps, func(i, j int) bool { return maps[i].Mapname < maps[j].Mapname })
		for _, m := range maps {
			if m.Published == "yes" && m.Mapname != "overview" && !strings.Contains(m.Mapname, "-nomap") {
				d.DeskMaps = append(d.DeskMaps, m.Mapname)
			}
		}
		// deskSummary() in backend80.js iterates the global "departments" object
		// (keyed by index, matching the legacy JSON_FORCE_OBJECT output).
		deptObj := map[string]string{}
		depts, _ := app.db.ListDepartments()
		for i, dp := range depts {
			deptObj[strconv.Itoa(i)] = dp
		}
		deptJSON, _ := json.Marshal(deptObj)
		d.DepartmentsJSON = template.JS(deptJSON)

	case "users":
		d.Roles, _ = app.db.ListRoles()
		roleName := map[int]string{}
		for _, ro := range d.Roles {
			roleName[ro.ID] = ro.Rolename
		}
		// Build a lookup of full names from the cached full directory, keyed by
		// lowercased samaccountname, so we can show a friendly name for every
		// AD-derived admin (not just those with an office attribute).
		ldapNames := map[string]string{}
		if dir, err := app.db.ListDirectory(); err == nil {
			for _, d := range dir {
				ldapNames[strings.ToLower(d.Userid)] = d.DisplayName()
			}
		}
		users, _ := app.db.ListUsers()
		for _, u := range users {
			name := roleName[u.Role]
			if name == "" {
				name = strconv.Itoa(u.Role)
			}
			// Resolve a display name: prefer a stored full name, then the cached
			// LDAP data (matched on samaccountname after stripping any DOMAIN\
			// prefix), and finally fall back to the username itself.
			display := strings.TrimSpace(u.Fullname)
			if display == "" {
				sam := u.Username
				if idx := strings.LastIndex(sam, "\\"); idx >= 0 {
					sam = sam[idx+1:]
				}
				if full, ok := ldapNames[strings.ToLower(sam)]; ok {
					display = full
				}
			}
			if display == "" {
				display = u.Username
			}
			d.Mapadmins = append(d.Mapadmins, adminUserRow{Username: u.Username, Name: display, Role: u.Role, RoleName: name})
		}
		sort.Slice(d.Mapadmins, func(i, j int) bool {
			return strings.ToLower(d.Mapadmins[i].Name) < strings.ToLower(d.Mapadmins[j].Name)
		})

	case "teams":
		d.Teams, _ = app.db.ListTeams()
		sort.Slice(d.Teams, func(i, j int) bool { return d.Teams[i].Teamname < d.Teams[j].Teamname })

	case "auditlog":
		// The audit log can hold 100k+ rows on production, so it is no longer
		// rendered server-side. The template renders the filter controls and the
		// front-end pages through entries lazily via /rest/auditlog. AuditTypes
		// feeds the Type dropdown.
		d.AuditTypes = []string{"Maps", "Desks", "Users", "Teams", "LDAP", "Settings", "Avatar", "login", "account", "setup"}
	}

	return d
}

func (app *App) flagExists(country string) bool {
	if country == "" {
		return false
	}
	_, err := fs.Stat(app.staticFS, "countryflags/"+country+".svg")
	return err == nil
}

func (app *App) listCountryflags() []string {
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

// handleRestLdap triggers an AD sync (used by the admin LDAP "Sync now" button).
func (app *App) handleRestLdap(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	idStr := r.URL.Query().Get("ldapid")
	if idStr != "" {
		if id, err := strconv.Atoi(idStr); err == nil {
			srcs, _ := app.db.ListLdapSources()
			for _, s := range srcs {
				if s.ID == id {
					users, dbg, err := app.syncOneSource(s)
					app.setSyncDebug(ADSyncDebug{
						When:    nowTimestamp(),
						Total:   len(users),
						Sources: []SourceDebug{dbg},
					})
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					s.LastSync = nowTimestamp()
					_ = app.db.PutLdapSource(s)
					_ = app.db.ReplaceLdap(users)
					_ = app.db.AuditLog("LDAP", sess.Username, "Manual sync of source "+idStr)
					writeJSON(w, map[string]interface{}{"status": "ok", "count": len(users)})
					return
				}
			}
			http.Error(w, "ldap source not found", http.StatusNotFound)
			return
		}
	}

	count, err := app.RunADSync()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "count": count})
}

// handleRestLdapDebug returns diagnostics from the most recent AD sync so the
// admin panel can show why a sync mirrored few/no users.
func (app *App) handleRestLdapDebug(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, app.SyncDebug())
}

// handleRestDirectorySearch returns directory users matching a query, for the
// admin add-user autocomplete. Each result includes the resolved username
// (DOMAIN\samaccountname) so the caller never has to know the samaccountname.
func (app *App) handleRestDirectorySearch(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "users") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	q := r.URL.Query().Get("q")
	results, _ := app.db.SearchDirectory(q, 20)
	domain := app.db.GetSetting("domain")
	out := make([]map[string]string, 0, len(results))
	for _, d := range results {
		username := d.Userid
		if domain != "" {
			username = domain + "\\" + d.Userid
		}
		out = append(out, map[string]string{
			"name":     d.DisplayName(),
			"sam":      d.Userid,
			"username": username,
			"mail":     d.Mail,
			"office":   d.Office,
		})
	}
	writeJSON(w, out)
}

// handleRestDirectoryMatch re-resolves the full name (and mail) of every
// existing admin against the cached AD directory. This is useful for accounts
// that were created before the directory cache existed, so their names get
// populated without waiting for the next scheduled sync.
func (app *App) handleRestDirectoryMatch(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "users") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	dir, _ := app.db.ListDirectory()
	if len(dir) == 0 {
		writeJSON(w, map[string]interface{}{
			"matched": 0, "updated": 0, "directory": 0,
			"message": "The directory cache is empty. Run an LDAP sync first.",
		})
		return
	}
	matched, updated := app.refreshAdminNames(dir)
	_ = app.db.AuditLog("Users", sess.Username, fmt.Sprintf("Matched names from directory (%d matched, %d updated)", matched, updated))
	writeJSON(w, map[string]interface{}{
		"matched":   matched,
		"updated":   updated,
		"directory": len(dir),
		"message":   fmt.Sprintf("%d user(s) matched, %d name(s) updated.", matched, updated),
	})
}

// handleRestSetting saves a single base variable and returns the stored value
// so the config tab can update it in place without a full page reload.
func (app *App) handleRestSetting(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "config") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" || name == "logo_regular" || name == "logo_hover" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	value := r.FormValue("value")
	_ = app.db.SetSetting(name, value)
	_ = app.db.AuditLog("Settings", sess.Username, "Base variable updated ("+name+")")
	writeJSON(w, map[string]string{"name": name, "value": value})
}

// vipCategoryList defines the fixed VIP categories and the border colors the
// maps use for each. Order matters for display.
var vipCategoryList = []struct{ Type, Color string }{
	{"Board", "#ffa500"},
	{"VP", "#800080"},
	{"Director", "#00bbff"},
	{"TeamManager", "#00CC00"},
}

// vipCategoriesPayload groups the stored VIP tags into the fixed categories so
// the admin chips UI can render (and the JS can re-render after edits).
func (app *App) vipCategoriesPayload() []map[string]interface{} {
	vips, _ := app.db.ListVips()
	byType := map[string][]string{}
	for _, v := range vips {
		if v.Title == "" {
			continue
		}
		byType[v.Type] = append(byType[v.Type], v.Title)
	}
	out := make([]map[string]interface{}, 0, len(vipCategoryList))
	for _, c := range vipCategoryList {
		tags := byType[c.Type]
		sort.Slice(tags, func(i, j int) bool { return strings.ToLower(tags[i]) < strings.ToLower(tags[j]) })
		out = append(out, map[string]interface{}{
			"type":  c.Type,
			"color": c.Color,
			"tags":  tags,
		})
	}
	return out
}

// handleRestVips powers the VIP chips UI: GET returns the grouped categories,
// POST adds or removes a tag and returns the updated grouping (so the page
// never has to reload).
func (app *App) handleRestVips(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "config") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if r.Method == http.MethodPost {
		if app.permLevel(sess, "config") < 2 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		action := r.FormValue("action")
		typ := strings.TrimSpace(r.FormValue("type"))
		tag := strings.TrimSpace(r.FormValue("tag"))
		valid := false
		for _, c := range vipCategoryList {
			if c.Type == typ {
				valid = true
				break
			}
		}
		if !valid || tag == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch action {
		case "add":
			_ = app.db.AddVipTag(typ, tag)
			_ = app.db.AuditLog("Settings", sess.Username, "VIP tag added ("+typ+": "+tag+")")
		case "remove":
			_ = app.db.DeleteVipTag(typ, tag)
			_ = app.db.AuditLog("Settings", sess.Username, "VIP tag removed ("+typ+": "+tag+")")
		default:
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
	}
	writeJSON(w, app.vipCategoriesPayload())
}

// resolveDirectoryEntry maps an entered value (samaccountname, DOMAIN\sam, or a
// display name) to a directory user. It returns false when nothing matches, in
// which case the caller keeps the raw input (e.g. a manual local username).
func (app *App) resolveDirectoryEntry(input string) (DirectoryUser, bool) {
	v := strings.TrimSpace(input)
	if v == "" {
		return DirectoryUser{}, false
	}
	// Strip any DOMAIN\ prefix and try a direct samaccountname lookup.
	sam := v
	if idx := strings.LastIndex(sam, "\\"); idx >= 0 {
		sam = sam[idx+1:]
	}
	if d, found, _ := app.db.GetDirectoryUser(sam); found {
		return d, true
	}
	// Fall back to an exact (case-insensitive) display-name match.
	dir, err := app.db.ListDirectory()
	if err != nil {
		return DirectoryUser{}, false
	}
	lv := strings.ToLower(v)
	var match DirectoryUser
	count := 0
	for _, d := range dir {
		if strings.ToLower(d.DisplayName()) == lv {
			match = d
			count++
		}
	}
	if count == 1 {
		return match, true
	}
	return DirectoryUser{}, false
}

// handleRestRobinTest runs a full Robin meeting sync and returns the step-by-step
// log so the admin panel can show exactly what happened during the sync.
func (app *App) handleRestRobinTest(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	_ = app.db.AuditLog("LDAP", sess.Username, "Robin meeting sync test run")
	writeJSON(w, map[string]interface{}{"log": app.RunRobinSyncVerbose()})
}

// handleRestRobinSync starts a Robin meeting sync in the background (if one is
// not already running) so the admin Sync tab can poll for live progress.
func (app *App) handleRestRobinSync(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !app.robinProg.start(0, "Starting…") {
		writeJSON(w, map[string]interface{}{"started": false, "running": true})
		return
	}
	_ = app.db.AuditLog("LDAP", sess.Username, "Robin meeting sync run")
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				app.robinProg.finish("", fmt.Sprintf("sync crashed: %v", rec))
			}
		}()
		res := app.runRobinSyncStructured(&app.robinProg)
		// Refresh the desk-reservation overlay in the same run (no-op unless the
		// "Show Robin desk reservations" mode is enabled), exactly like the
		// 5-minute scheduler does, so one button syncs everything.
		app.robinProg.setStage("Syncing desk reservations…")
		app.pollRobinDeskOccupancy(&app.robinProg)
		if res.Note != "" {
			app.robinProg.finish(res.Note, "")
			return
		}
		app.robinProg.finish(fmt.Sprintf("%d of %d room(s) matched a meeting desk.", res.MatchedRooms, res.TotalRooms), "")
	}()
	writeJSON(w, map[string]interface{}{"started": true})
}

// handleRestRobinProgress returns the current Robin sync progress snapshot.
func (app *App) handleRestRobinProgress(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, app.robinProg.snapshot())
}

// handleRestRobinDeskTest starts the read-only Robin desk-data diagnostic in
// the background (if one is not already running) so the admin Sync tab can poll
// for a live progress bar + log. The diagnostic walks every configured location
// (spaces → state, events, seats, seat reservations for today), captures the raw
// JSON, logs every seat reservation active right now matched to a CompanyMaps
// desk, and caches the bundle for download. It never writes to the meeting
// cache, the booking feature, or the map.
func (app *App) handleRestRobinDeskTest(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !app.robinDeskProg.start(0, "Starting…") {
		writeJSON(w, map[string]interface{}{"started": false, "running": true})
		return
	}
	_ = app.db.AuditLog("LDAP", sess.Username, "Robin desk-data diagnostic run")
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				app.robinDeskProg.finish("", fmt.Sprintf("diagnostic crashed: %v", rec))
			}
		}()
		_, files, res := app.runRobinDeskDump(&app.robinDeskProg)
		app.robinDumpMu.Lock()
		app.robinDumpFiles = files
		app.robinDumpTime = time.Now().Format("2006-01-02 15:04:05")
		app.robinDumpMu.Unlock()
		app.robinDeskProg.finish(fmt.Sprintf("%d desk(s) occupied now matched (%d unmatched). %d JSON file(s) captured.",
			res.Matched, res.Unmatched, res.Files), "")
	}()
	writeJSON(w, map[string]interface{}{"started": true})
}

// handleRestRobinDeskProgress returns the current desk-diagnostic progress.
func (app *App) handleRestRobinDeskProgress(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, app.robinDeskProg.snapshot())
}

// handleRestRobinDeskDump streams the most recently captured desk-data
// diagnostic bundle as a zip. If no bundle has been captured yet (or it is
// empty) it runs a fresh diagnostic first.
func (app *App) handleRestRobinDeskDump(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	app.robinDumpMu.Lock()
	files := app.robinDumpFiles
	when := app.robinDumpTime
	app.robinDumpMu.Unlock()

	var logs []string
	if len(files) == 0 {
		logs, files = app.RunRobinDeskDump()
		when = time.Now().Format("2006-01-02 15:04:05")
		app.robinDumpMu.Lock()
		app.robinDumpFiles = files
		app.robinDumpTime = when
		app.robinDumpMu.Unlock()
	}
	_ = app.db.AuditLog("LDAP", sess.Username, "Robin desk-data diagnostic download")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// summary.json: metadata plus the run log (if we have one).
	summary := map[string]interface{}{
		"generated":    time.Now().Format(time.RFC3339),
		"captured_at":  when,
		"organisation": app.db.GetRobinSetting("robinOrganisation"),
		"file_count":   len(files),
	}
	if logs != nil {
		summary["log"] = logs
	}
	if sb, err := json.MarshalIndent(summary, "", "  "); err == nil {
		if fw, err := zw.Create("summary.json"); err == nil {
			_, _ = fw.Write(sb)
		}
	}

	for _, f := range files {
		fw, err := zw.Create(f.Name)
		if err != nil {
			continue
		}
		_, _ = fw.Write(f.Data)
	}
	if err := zw.Close(); err != nil {
		http.Error(w, "could not build zip", http.StatusInternalServerError)
		return
	}

	fname := "robin-desk-dump-" + time.Now().Format("20060102-150405") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+fname+"\"")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	_, _ = w.Write(buf.Bytes())
}

// handleRestRobinSuggestions starts a read-only background scan of every mapped
// Robin location's seats, proposing strip prefixes/suffixes that would make a
// near-miss seat name match a CompanyMaps desk number. The admin Sync tab polls
// handleRestRobinSuggestionsProgress for a live progress bar and the results.
func (app *App) handleRestRobinSuggestions(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !app.robinSuggestProg.start(0, "Starting…") {
		writeJSON(w, map[string]interface{}{"started": false, "running": true})
		return
	}
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				app.robinSuggestProg.finish("", fmt.Sprintf("scan crashed: %v", rec))
			}
		}()
		suggestions, err := app.collectRobinStripSuggestions(&app.robinSuggestProg)
		if err != nil {
			app.robinSuggestProg.finish("", err.Error())
			return
		}
		app.robinSuggestMu.Lock()
		app.robinSuggestResult = suggestions
		app.robinSuggestMu.Unlock()
		app.robinSuggestProg.finish(fmt.Sprintf("%d suggestion(s) found.", len(suggestions)), "")
	}()
	writeJSON(w, map[string]interface{}{"started": true})
}

// handleRestRobinSuggestionsProgress returns the current strip-suggestion scan
// progress. Once the scan is done it also includes the suggestions list.
func (app *App) handleRestRobinSuggestionsProgress(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	snap := app.robinSuggestProg.snapshot()
	if done, _ := snap["done"].(bool); done {
		app.robinSuggestMu.Lock()
		snap["suggestions"] = app.robinSuggestResult
		app.robinSuggestMu.Unlock()
	}
	writeJSON(w, snap)
}

// handleRestRobinStripAdd appends a single strip prefix/suffix pattern to the
// configured list (enabling that strip type) so a suggestion can be applied with
// one click.
func (app *App) handleRestRobinStripAdd(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	pat := r.FormValue("pattern")
	if strings.TrimSpace(pat) == "" {
		writeJSON(w, map[string]interface{}{"error": "empty pattern"})
		return
	}
	var listKey, enKey string
	switch r.FormValue("type") {
	case "prefix":
		listKey, enKey = "robinStripPrefixList", "robinStripPrefixEnabled"
	case "suffix":
		listKey, enKey = "robinStripSuffixList", "robinStripSuffixEnabled"
	default:
		writeJSON(w, map[string]interface{}{"error": "invalid type"})
		return
	}
	existing := splitRobinList(app.db.GetRobinSetting(listKey))
	for _, e := range existing {
		if e == pat {
			writeJSON(w, map[string]interface{}{"ok": true, "already": true})
			return
		}
	}
	existing = append(existing, pat)
	_ = app.db.SetRobinSetting(listKey, strings.Join(existing, "\n"))
	_ = app.db.SetRobinSetting(enKey, "1")
	_ = app.db.AuditLog("LDAP", sess.Username, "Robin strip "+r.FormValue("type")+" added: "+pat)
	writeJSON(w, map[string]interface{}{"ok": true})
}

// handleRestLdapSync starts an AD sync of all sources in the background so the
// admin Sync tab can poll for live progress.
func (app *App) handleRestLdapSync(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !app.ldapProg.start(0, "Starting…") {
		writeJSON(w, map[string]interface{}{"started": false, "running": true})
		return
	}
	_ = app.db.AuditLog("LDAP", sess.Username, "Manual AD sync (all sources)")
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				app.ldapProg.finish("", fmt.Sprintf("sync crashed: %v", rec))
			}
		}()
		count, err := app.runADSync(&app.ldapProg)
		if err != nil {
			app.ldapProg.finish("", err.Error())
			return
		}
		app.ldapProg.finish(fmt.Sprintf("Mirrored %d placement(s).", count), "")
	}()
	writeJSON(w, map[string]interface{}{"started": true})
}

// handleRestLdapProgress returns the current AD sync progress snapshot.
func (app *App) handleRestLdapProgress(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, app.ldapProg.snapshot())
}

// handleAuditReimport is the superadmin-only one-time action that re-imports the
// historical audit log from the legacy CompanyMaps 8 MySQL database. The original
// migration omitted the `auditlog` table, so production instances are missing
// their pre-migration history. This clears the local audit bucket, imports the
// old log (oldest-first) and lets live events continue to append on top. MySQL
// credentials are not stored anywhere, so they are supplied with the request.
func (app *App) handleAuditReimport(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	if app.permLevel(sess, "adminpanel") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c := MySQLConfig{
		Host:     orDefault(r.FormValue("host"), "localhost"),
		Port:     orDefault(r.FormValue("port"), "3306"),
		Database: strings.TrimSpace(r.FormValue("database")),
		User:     strings.TrimSpace(r.FormValue("user")),
		Password: r.FormValue("password"),
	}
	if c.Database == "" || c.User == "" {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "Database and user are required."})
		return
	}
	count, err := app.ImportAuditOnly(c)
	if err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "message": err.Error()})
		return
	}
	_ = app.db.AuditLog("auditlog", sess.Username, fmt.Sprintf("Audit log re-imported from MySQL (%d historical entries)", count))
	writeJSON(w, map[string]interface{}{"ok": true, "count": count,
		"message": fmt.Sprintf("Imported %d historical audit entries.", count)})
}


// orDefaultStr returns v trimmed, or def when empty.
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
