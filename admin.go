package main

import (
	"image"
	"image/png"
	"io/fs"
	"mime/multipart"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// mapRow is a maps-tab table row with derived existence indicators.
type mapRow struct {
	MapInfo
	HasFile bool
	HasDB   bool
	HasFlag bool
}

// adminUserRow is a users-tab row (a config_mapadmins entry).
type adminUserRow struct {
	Username string
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
	LeftPos           int
	TopHeader         int

	ActiveTab string
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

	GeneralVars  []kv
	Vips         []VIP
	LogoRegular  string
	LogoHover    string
	LdapSources  []LdapSource
	Maps         []mapRow
	DeskMaps     []string
	Mapadmins    []adminUserRow
	Roles        []Role
	Teams        []Team
	AuditEntries []AuditEntry
	AuditFilter  string
	AuditTypes   []string
	Countryflags []string
	Timezones    []string
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
	if !ok || app.permLevel(sess, "adminpanel") < 1 {
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
	// Fall back to dashboard if the user lacks permission for the tab. The SAML
	// tab is gated by the same "adminpanel" permission its REST endpoints use.
	permKey := tab
	if tab == "saml" {
		permKey = "adminpanel"
	}
	if tab != "dashboard" && app.permLevel(sess, permKey) == 0 {
		tab = "dashboard"
	}

	data := app.buildAdminData(r, sess, tab, msg)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := app.tmpl.ExecuteTemplate(w, "admin.html", data); err != nil {
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
		if id := r.FormValue("toggleMapID"); id != "" {
			name := r.FormValue("toggleMapname")
			status := r.FormValue("toggleMapStatus")
			if m, found, _ := app.db.GetMap(name); found {
				m.Published = status
				_ = app.db.PutMap(m)
			}
			return "Map visibility updated."
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
		user := r.FormValue("newadminuser")
		role := r.FormValue("newadminrole")
		if user != "" && role != "" {
			roleInt, _ := strconv.Atoi(role)
			existing, found, _ := app.db.GetUser(user)
			if found {
				existing.Role = roleInt
				_ = app.db.PutUser(existing)
			} else {
				_ = app.db.PutUser(User{Username: user, Role: roleInt})
			}
			_ = app.db.AuditLog("Users", sess.Username, "New admin created ("+user+", role "+role+")")
			return "User created."
		}

	case "teams":
		if del := r.FormValue("deleteTeam"); del != "" {
			_ = app.db.DeleteTeam(del)
			_ = app.db.AuditLog("Teams", sess.Username, "Team removed ("+del+")")
			return "Team removed."
		}
		name := r.FormValue("newTeam")
		members := r.FormValue("newMembers")
		if name != "" && members != "" {
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
		Mapname:   name,
		Itemscale: orDefaultStr(r.FormValue("newMapItemscale"), "1"),
		Published: orDefaultStr(r.FormValue("newMapPublished"), "yes"),
		Country:   strings.ToLower(r.FormValue("newMapCountry")),
		Flagsize:  orDefaultStr(r.FormValue("newMapFlagsize"), "0"),
		Timezone:  orDefaultStr(r.FormValue("newMapTimezone"), "Europe/Berlin"),
		Address:   r.FormValue("newMapAddress"),
		MapX:      x,
		MapY:      y,
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
	targetWidth, _ := strconv.Atoi(app.settingOr("targetScreenWidth", "1600"))
	if targetWidth == 0 {
		targetWidth = 1600
	}

	d := adminData{
		AppTitle:          app.appTitle(),
		TargetScreenWidth: targetWidth,
		HalfWidth:         targetWidth / 2,
		Autozoom:          autozoom,
		Zoom:              zoom,
		ContentScale:      strconv.FormatFloat(float64(zoom)/100*float64(autozoom), 'f', -1, 64),
		LeftPos:           cookieInt(r, "LeftPos", 0),
		TopHeader:         69 * autozoom,
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
			d.GeneralVars = append(d.GeneralVars, kv{Variable: k, Value: v})
		}
		sort.Slice(d.GeneralVars, func(i, j int) bool { return d.GeneralVars[i].Variable < d.GeneralVars[j].Variable })
		d.Vips, _ = app.db.ListVips()
		d.LogoRegular = app.settingOr("logo_regular", "/static/images/cmaps-regular.png")
		d.LogoHover = app.settingOr("logo_hover", "/static/images/cmaps-hover.png")

	case "ldap":
		d.LdapSources, _ = app.db.ListLdapSources()

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

	case "users":
		d.Roles, _ = app.db.ListRoles()
		roleName := map[int]string{}
		for _, ro := range d.Roles {
			roleName[ro.ID] = ro.Rolename
		}
		users, _ := app.db.ListUsers()
		for _, u := range users {
			name := roleName[u.Role]
			if name == "" {
				name = strconv.Itoa(u.Role)
			}
			d.Mapadmins = append(d.Mapadmins, adminUserRow{Username: u.Username, Role: u.Role, RoleName: name})
		}
		sort.Slice(d.Mapadmins, func(i, j int) bool { return d.Mapadmins[i].Username < d.Mapadmins[j].Username })

	case "teams":
		d.Teams, _ = app.db.ListTeams()
		sort.Slice(d.Teams, func(i, j int) bool { return d.Teams[i].Teamname < d.Teams[j].Teamname })

	case "auditlog":
		d.AuditFilter = r.FormValue("auditlogEventType")
		d.AuditTypes = []string{"All", "Access", "Users", "Desks", "Teams", "LDAP"}
		all, _ := app.db.ListAudit(0)
		for _, e := range all {
			if d.AuditFilter == "" || e.Type == d.AuditFilter {
				d.AuditEntries = append(d.AuditEntries, e)
			}
			if len(d.AuditEntries) >= 200 {
				break
			}
		}
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
