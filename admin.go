package main

import (
	"encoding/json"
	"fmt"
	"html/template"
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
	Vips            []VIP
	LogoRegular     string
	LogoHover       string
	LdapSources     []LdapSource
	RobinSpaces     []RobinSpace
	RobinMapOptions []string
	RobinOrg        string
	RobinSet        bool
	RobinLastSync   RobinSyncResult
	RobinHasSync    bool
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
				_ = app.db.SetSetting("robintoken", tok)
			}
			_ = app.db.SetSetting("robinOrganisation", strings.TrimSpace(r.FormValue("robinOrganisation")))
			_ = app.db.AuditLog("LDAP", sess.Username, "Robin credentials updated")
			return "Robin settings saved."
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
		if del := strings.TrimSpace(r.FormValue("deleteSetting")); del != "" {
			_ = app.db.DeleteSetting(del)
			_ = app.db.AuditLog("Settings", sess.Username, "Base variable deleted ("+del+")")
			return "Variable removed."
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
		d.RobinSpaces, _ = app.db.ListRobinSpaces()
		sort.Slice(d.RobinSpaces, func(i, j int) bool { return d.RobinSpaces[i].Spacename < d.RobinSpaces[j].Spacename })
		d.RobinOrg = app.db.GetSetting("robinOrganisation")
		d.RobinSet = app.db.GetSetting("robintoken") != ""
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
