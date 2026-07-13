package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

func (app *Server) buildAdminData(r *http.Request, sess Session, tab, msg string) adminData {
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
			// The world map has its own toggle card, so hide it from the table.
			if k == "worldmap" {
				continue
			}
			// Internal booking has its own toggle card, so hide it from the table.
			if k == "internalbooking" {
				continue
			}
			d.GeneralVars = append(d.GeneralVars, kv{Variable: k, Value: v, Description: settingDescriptions[k]})
		}
		sort.Slice(d.GeneralVars, func(i, j int) bool { return d.GeneralVars[i].Variable < d.GeneralVars[j].Variable })
		d.LogoRegular = app.settingOr("logo_regular", "/static/images/cmaps-regular.png")
		d.LogoHover = app.settingOr("logo_hover", "/static/images/cmaps-hover.png")
		d.BackupGroups = backupGroups
		d.WorldMap = app.db.GetSetting("worldmap") == "1"
		d.InternalBooking = app.internalBookingEnabled()
		d.CustomTypes, _ = app.db.ListItemTypes()
		sort.Slice(d.CustomTypes, func(i, j int) bool {
			return strings.ToLower(d.CustomTypes[i].Label) < strings.ToLower(d.CustomTypes[j].Label)
		})

	case "ldap":
		d.IdentifierMode = app.identifierMode()
		d.LdapSources, _ = app.db.ListLdapSources()
		// The demo source is a pseudo-LDAP source; keep it out of the editable
		// LDAP connections table (it still appears in the unified priority list).
		realSources := d.LdapSources[:0]
		for _, s := range d.LdapSources {
			if s.Demo {
				d.DemoSourceExists = true
				continue
			}
			realSources = append(realSources, s)
		}
		d.LdapSources = realSources
		d.HasRealSource = app.hasRealSource()
		d.UnifiedSources = app.listUnifiedSources()
		// Effective seat counts under the current priority/dedup/assign settings,
		// recomputed on every render so moving/toggling a source updates them.
		if len(d.UnifiedSources) > 0 {
			counts := app.sourceSeatCounts()
			for i := range d.UnifiedSources {
				d.UnifiedSources[i].PopulatedSeats = counts[d.UnifiedSources[i].Ref]
			}
		}
		d.RobinSpaces, _ = app.db.ListRobinSpaces()
		sort.Slice(d.RobinSpaces, func(i, j int) bool { return d.RobinSpaces[i].Spacename < d.RobinSpaces[j].Spacename })
		d.RobinOrg = app.db.GetRobinSetting("robinOrganisation")
		d.RobinSet = app.db.GetRobinSetting("robintoken") != ""
		d.RobinEnabled = app.robin.Enabled()
		d.GeoapifySet = app.db.GetGeoSetting("geoapifyApiKey") != ""
		d.GeoEnabled = app.geoEnabled()
		d.GeoUsageMonth, d.GeoUsageCount = app.db.GetGeoUsage()
		d.NextLdapSync = app.nextSyncLabel(app.dir.NextLdapSync(), app.dir.AnyLdapSourceEnabled())
		d.NextEntraSync = app.nextSyncLabel(app.dir.NextEntraSync(), app.dir.EntraHasEnabledSource())
		d.NextRobinSync = app.nextSyncLabel(app.robin.NextSync(), app.robin.Enabled() && app.db.GetRobinSetting("robintoken") != "")
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
		d.RobinLastSync, d.RobinHasSync = app.robin.LastSyncResult()
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
		if dr, ok := app.robin.LastDeskSyncResult(); ok {
			d.RobinDeskHasSync = true
			d.RobinDeskLastSyncTime = dr.Time
			d.RobinDeskCount = dr.Count
		}

		// --- AD <-> Robin overlap check (cached data only, no fresh sync) ------
		// Compares the AD mirror (who AD seats where) against the cached Robin
		// desk occupancy to surface: (1) desks where both systems name the same
		// person at the same desk, where the AD placement keeps priority and the
		// Robin overlay is dropped; and (2) people AD seats at one desk while
		// Robin seats them at a different desk on the same map, making them show
		// up twice. This only reads the two caches, mirroring buildMapDesks.
		ldapUsers, _ := app.db.ListLdap()
		robinDesks, _ := app.db.ListRobinDeskStatus("")
		// desk number -> map so an AD placement can be located on the same map as
		// a Robin reservation.
		deskToMap := map[string]string{}
		if allMaps, err := app.db.ListMaps(); err == nil {
			for _, m := range allMaps {
				if m.Mapname == "overview" {
					continue
				}
				desks, _ := app.db.ListDesks(m.Mapname)
				for _, dsk := range desks {
					if dsk.Desktype == "addesk" {
						deskToMap[dsk.Desknumber] = m.Mapname
					}
				}
			}
		}
		seenSame := map[string]bool{}
		seenDup := map[string]bool{}
		// Whether a Robin reservation is actually shown on the map now depends on
		// the unified priority engine, so ask it directly (per map, cached).
		avatarIdx := app.buildAvatarIndex()
		assignCache := map[string]map[string][]deskOccupant{}
		robinShownAt := func(m, desknumber string) bool {
			a, ok := assignCache[m]
			if !ok {
				desks, _ := app.db.ListDesks(m)
				a = app.assignMapOccupancy(m, desks, avatarIdx)
				assignCache[m] = a
			}
			for _, o := range a[strings.ToLower(strings.TrimSpace(desknumber))] {
				if o.sourceType == "robin" {
					return true
				}
			}
			return false
		}
		for _, rs := range robinDesks {
			rdesk := strings.TrimSpace(rs.Desknumber)
			rmap := strings.TrimSpace(rs.Map)
			for _, u := range ldapUsers {
				if !sameRobinPerson(rs, u) {
					continue
				}
				adesk := strings.TrimSpace(u.Office)
				if adesk == "" {
					continue
				}
				name := strings.TrimSpace(rs.Name)
				if name == "" {
					name = strings.TrimSpace(u.Givenname + " " + u.Surname)
				}
				uid := u.Userid
				if uid == "" {
					uid = rs.Userid
				}
				if adesk == rdesk {
					// Same person, same desk: AD keeps priority.
					key := rmap + "\x00" + rdesk
					if !seenSame[key] {
						seenSame[key] = true
						d.RobinAdSameDesk = append(d.RobinAdSameDesk, RobinAdOverlap{
							Map: rmap, Desknumber: rdesk, Name: name, Userid: uid,
						})
					}
					continue
				}
				// Different desks only count when both land on the same map.
				if deskToMap[adesk] != rmap {
					continue
				}
				key := rmap + "\x00" + adesk + "\x00" + rdesk + "\x00" + strings.ToLower(uid)
				if seenDup[key] {
					continue
				}
				seenDup[key] = true
				rendered := robinShownAt(rmap, rdesk)
				d.RobinAdDuplicates = append(d.RobinAdDuplicates, RobinAdDuplicate{
					Map: rmap, Name: name, Userid: uid,
					AdDesk: adesk, RobinDesk: rdesk, Rendered: rendered,
				})
			}
		}
		sort.Slice(d.RobinAdSameDesk, func(i, j int) bool {
			if d.RobinAdSameDesk[i].Map != d.RobinAdSameDesk[j].Map {
				return d.RobinAdSameDesk[i].Map < d.RobinAdSameDesk[j].Map
			}
			return d.RobinAdSameDesk[i].Desknumber < d.RobinAdSameDesk[j].Desknumber
		})
		sort.Slice(d.RobinAdDuplicates, func(i, j int) bool {
			if d.RobinAdDuplicates[i].Map != d.RobinAdDuplicates[j].Map {
				return d.RobinAdDuplicates[i].Map < d.RobinAdDuplicates[j].Map
			}
			return d.RobinAdDuplicates[i].Name < d.RobinAdDuplicates[j].Name
		})

		// --- EntraID connections + LDAP <-> EntraID mirror comparison --------
		d.EntraSources, _ = app.db.ListEntraSources()
		d.EntraSet = len(d.EntraSources) > 0
		d.EntraLastSync = app.db.GetEntraSetting("entraLastSync")
		d.EntraHasSync = d.EntraLastSync != ""
		entraUsers, _ := app.db.ListEntraLdap()
		d.EntraCount = len(entraUsers)
		// Users are matched between the two mirrors strictly by e-mail
		// (case-insensitive). People with no e-mail cannot be matched.
		type entraCmp struct {
			name  string
			mail  string
			ldap  map[string]bool
			entra map[string]bool
		}
		cmp := map[string]*entraCmp{}
		add := func(u LdapUser, fromEntra bool) {
			mail := strings.ToLower(strings.TrimSpace(u.Mail))
			if mail == "" {
				return
			}
			row := cmp[mail]
			if row == nil {
				row = &entraCmp{mail: strings.TrimSpace(u.Mail), ldap: map[string]bool{}, entra: map[string]bool{}}
				cmp[mail] = row
			}
			if row.name == "" {
				row.name = strings.TrimSpace(u.Givenname + " " + u.Surname)
			}
			off := strings.TrimSpace(u.Office)
			if off == "" {
				return
			}
			if fromEntra {
				row.entra[off] = true
			} else {
				row.ldap[off] = true
			}
		}
		for _, u := range ldapUsers {
			add(u, false)
		}
		for _, u := range entraUsers {
			add(u, true)
		}
		deskList := func(set map[string]bool) string {
			out := make([]string, 0, len(set))
			for k := range set {
				out = append(out, k)
			}
			sort.Strings(out)
			return strings.Join(out, ", ")
		}
		sameSet := func(a, b map[string]bool) bool {
			if len(a) != len(b) {
				return false
			}
			for k := range a {
				if !b[k] {
					return false
				}
			}
			return true
		}
		for _, row := range cmp {
			r := EntraLdapRow{
				Name:       row.name,
				Mail:       row.mail,
				LdapDesks:  deskList(row.ldap),
				EntraDesks: deskList(row.entra),
			}
			inLdap := len(row.ldap) > 0
			inEntra := len(row.entra) > 0
			switch {
			case inLdap && inEntra && sameSet(row.ldap, row.entra):
				d.EntraMatchedSame = append(d.EntraMatchedSame, r)
			case inLdap && inEntra:
				d.EntraMatchedDiff = append(d.EntraMatchedDiff, r)
			case inLdap:
				d.EntraOnlyLdap = append(d.EntraOnlyLdap, r)
			case inEntra:
				d.EntraOnlyEntra = append(d.EntraOnlyEntra, r)
			}
		}
		byName := func(rows []EntraLdapRow) func(i, j int) bool {
			return func(i, j int) bool {
				if rows[i].Name != rows[j].Name {
					return rows[i].Name < rows[j].Name
				}
				return rows[i].Mail < rows[j].Mail
			}
		}
		sort.Slice(d.EntraMatchedSame, byName(d.EntraMatchedSame))
		sort.Slice(d.EntraMatchedDiff, byName(d.EntraMatchedDiff))
		sort.Slice(d.EntraOnlyLdap, byName(d.EntraOnlyLdap))
		sort.Slice(d.EntraOnlyEntra, byName(d.EntraOnlyEntra))

	case "maps":
		maps, _ := app.db.ListMaps()
		sort.Slice(maps, func(i, j int) bool { return maps[i].Mapname < maps[j].Mapname })
		for _, m := range maps {
			if m.Mapname == "overview" {
				continue
			}
			row := mapRow{MapInfo: m}
			if _, err := os.Stat(app.cfg.DataPath("maps", m.Mapname+".png")); err == nil {
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
		// Drives which coordinate pair the maps tab treats as authoritative
		// (modern/world map -> lat/lon; classic -> X/Y) and whether the geocode
		// helper is offered when switching.
		d.WorldMap = app.db.GetSetting("worldmap") == "1"
		d.GeoapifyConfigured = app.db.GetGeoSetting("geoapifyApiKey") != ""

	case "desks":
		maps, _ := app.db.ListMaps()
		sort.Slice(maps, func(i, j int) bool { return maps[i].Mapname < maps[j].Mapname })
		for _, m := range maps {
			if m.Published == "yes" && m.Mapname != "overview" && !strings.Contains(m.Mapname, "-nomap") {
				d.DeskMaps = append(d.DeskMaps, m.Mapname)
			}
		}
		// deskSummary() in backend.js iterates the global "departments" object
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
		ldapMails := map[string]string{}
		if dir, err := app.db.ListDirectory(); err == nil {
			for _, d := range dir {
				ldapNames[strings.ToLower(d.Userid)] = d.DisplayName()
				ldapMails[strings.ToLower(d.Userid)] = d.Mail
			}
		}
		users, _ := app.db.ListUsers()
		for _, u := range users {
			// The users tab lists admin users only. Role 0 means "no role
			// assigned" (e.g. a freshly provisioned SAML user), so skip those.
			if u.Role == 0 {
				continue
			}
			name := roleName[u.Role]
			if name == "" {
				name = strconv.Itoa(u.Role)
			}
			// Resolve a display name: prefer a stored full name, then the cached
			// LDAP data (matched on samaccountname after stripping any DOMAIN\
			// prefix), and finally fall back to the username itself.
			sam := u.Username
			if idx := strings.LastIndex(sam, "\\"); idx >= 0 {
				sam = sam[idx+1:]
			}
			display := strings.TrimSpace(u.Fullname)
			if display == "" {
				if full, ok := ldapNames[strings.ToLower(sam)]; ok {
					display = full
				}
			}
			if display == "" {
				display = u.Username
			}
			// Resolve the e-mail: prefer the stored user mail, else the cached
			// directory mail matched on samaccountname.
			mail := strings.TrimSpace(u.Mail)
			if mail == "" {
				if m, ok := ldapMails[strings.ToLower(sam)]; ok {
					mail = strings.TrimSpace(m)
				}
			}
			d.Mapadmins = append(d.Mapadmins, adminUserRow{Username: u.Username, Name: display, Mail: mail, Role: u.Role, RoleName: name})
		}
		sort.Slice(d.Mapadmins, func(i, j int) bool {
			return strings.ToLower(d.Mapadmins[i].Name) < strings.ToLower(d.Mapadmins[j].Name)
		})

	case "teams":
		d.Teams, _ = app.db.ListTeams()
		sort.Slice(d.Teams, func(i, j int) bool { return d.Teams[i].Teamname < d.Teams[j].Teamname })

	case "itemtypes":
		d.CustomTypes, _ = app.db.ListItemTypes()
		sort.Slice(d.CustomTypes, func(i, j int) bool {
			return strings.ToLower(d.CustomTypes[i].Label) < strings.ToLower(d.CustomTypes[j].Label)
		})

	case "auditlog":
		// The audit log can hold 100k+ rows on production, so it is no longer
		// rendered server-side. The template renders the filter controls and the
		// front-end pages through entries lazily via /rest/auditlog. AuditTypes
		// feeds the Type dropdown.
		d.AuditTypes = []string{"Maps", "Desks", "Users", "Teams", "LDAP", "Settings", "Avatar", "login", "account", "setup"}
	}

	return d
}
