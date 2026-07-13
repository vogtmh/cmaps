package main

import (
	"net/http"
	"sort"
	"strings"
)

// deskItem is one entry in the /rest/desks response. Field names and the
// "0"/"1" booked string match the legacy PHP API consumed by user.js.
type deskItem struct {
	Map      string `json:"map"`
	ID       int    `json:"id,string"`
	Desktype string `json:"desktype"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Dsk      string `json:"dsk"`
	Empl     string `json:"empl"`
	Avtr     string `json:"avtr"`
	Dept     string `json:"dept"`
	Fname    string `json:"fname"`
	Lname    string `json:"lname"`
	Phone    string `json:"phone"`
	Mail     string `json:"mail"`
	Title    string `json:"title"`
	Mobil    string `json:"mobil"`
	Color    string `json:"color"`
	Parsed   string `json:"parsed"`
	Booked   string `json:"booked"`
	// Source identifies the directory a desk occupancy came from: "ldap",
	// "entraid" or "robin". Empty for manually placed / non-directory desks.
	// Drives the harmonized (blue) ball color and the per-source name badge.
	Source string `json:"source,omitempty"`
	// Config* preserve the underlying STORED desk configuration on synthetic
	// overlay items (e.g. a Robin-occupied desk is shown as "occupied" with the
	// live occupant in Empl). The editor uses these to edit the real item instead
	// of the transient occupancy values. Empty/omitted on normal items.
	ConfigType string `json:"configtype,omitempty"`
	ConfigEmpl string `json:"configempl,omitempty"`
	ConfigAvtr string `json:"configavtr,omitempty"`
	// HasAvatar tells the client whether a cached avatar image exists for this
	// desk's occupant. When false the client uses a single shared placeholder URL
	// (downloaded once) instead of requesting a unique missing image per person.
	HasAvatar bool      `json:"hasavatar"`
	Bookdata  *bookData `json:"bookdata,omitempty"`
}

type bookData struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Mail  string `json:"mail"`
}

// handleRestDesks serves /rest/desks?map=&search=&date=. With no map it iterates
// all published maps (except "overview").
func (app *App) handleRestDesks(w http.ResponseWriter, r *http.Request) {
	mapName := r.URL.Query().Get("map")
	search := r.URL.Query().Get("search")
	userDate := r.URL.Query().Get("date")

	vips, _ := app.db.ListVips()
	bookings, _ := app.db.ListBookings()
	avatarByUser := app.buildAvatarIndex()

	out := struct {
		Desks []deskItem `json:"desks"`
	}{Desks: []deskItem{}}

	if mapName != "" {
		date := userDate
		if date == "" {
			date = app.db.MapToday(mapName)
		}
		out.Desks = append(out.Desks, app.buildMapDesks(mapName, date, search, vips, bookings, avatarByUser)...)
		writeJSON(w, out)
		return
	}

	maps, _ := app.db.ListMaps()
	sort.Slice(maps, func(i, j int) bool { return maps[i].Mapname < maps[j].Mapname })
	for _, m := range maps {
		if m.Published == "no" || m.Mapname == "overview" {
			continue
		}
		date := userDate
		if date == "" {
			date = app.db.MapToday(m.Mapname)
		}
		out.Desks = append(out.Desks, app.buildMapDesks(m.Mapname, date, search, vips, bookings, avatarByUser)...)
	}
	writeJSON(w, out)
}

// buildAvatarIndex returns a lowercased-userid -> hasAvatar map spanning both
// the LDAP and EntraID combined mirrors (where the sync sets the HasAvatar
// flag). The per-source mirrors do not carry that flag, so occupancy from any
// source resolves its avatar availability through this shared index.
func (app *App) buildAvatarIndex() map[string]bool {
	idx := map[string]bool{}
	add := func(users []LdapUser) {
		for _, u := range users {
			if u.HasAvatar {
				idx[strings.ToLower(strings.TrimSpace(u.Userid))] = true
			}
		}
	}
	ldap, _ := app.db.ListLdap()
	add(ldap)
	entra, _ := app.db.ListEntraLdap()
	add(entra)
	return idx
}

// buildMapDesks expands one map's desks into output items. Desk occupancy is
// resolved by the unified, priority-ordered source engine (assignMapOccupancy):
// each configured source (LDAP/EntraID configs and Robin) fills desks in
// priority order, higher-priority sources own a desk outright and the per-source
// "keep duplicates" flag decides whether a lower-priority source may show a
// person again on the same map. VIP border colors and bookings are then applied.
func (app *App) buildMapDesks(mapName, date, search string, vips []VIP, bookings []Booking, avatarByUser map[string]bool) []deskItem {
	desks, _ := app.db.ListDesks(mapName)
	var items []deskItem

	occupancy := app.assignMapOccupancy(mapName, desks, avatarByUser)

	for _, d := range desks {
		// Booking lookup for this desk on the given date.
		var booked = "0"
		var bd *bookData
		for _, b := range bookings {
			if b.Date == date && b.Map == mapName && b.Desk == d.Desknumber {
				booked = "1"
				bd = &bookData{Name: b.Fullname, Phone: b.Phone, Mail: b.Mail}
			}
		}

		occ := occupancy[strings.ToLower(strings.TrimSpace(d.Desknumber))]
		if len(occ) > 0 {
			// All occupants of a desk come from the same (winning) source. A
			// directory occupant (LDAP/EntraID, carries first/last name) renders
			// as the native addesk/shareddesk; a Robin occupant (display name
			// only) renders as an "occupied" overlay that preserves the desk's
			// stored config for the editor.
			directory := occ[0].sourceType == "ldap" || occ[0].sourceType == "entra"
			desktype := "occupied"
			if directory {
				desktype = "addesk"
				if len(occ) > 1 {
					desktype = "shareddesk"
				}
			}
			for _, o := range occ {
				avtr := o.userid
				if avtr == "" {
					avtr = "noavatar"
				}
				fullname := strings.TrimSpace(o.fname + " " + o.lname)
				item := deskItem{
					Map: mapName, ID: d.ID, Desktype: desktype, X: d.X, Y: d.Y,
					Dsk: d.Desknumber, Avtr: avtr, Dept: d.Department,
					Fname: o.fname, Lname: o.lname, Phone: o.phone, Mail: o.mail,
					Title: o.title, Mobil: o.mobile, Booked: booked,
					HasAvatar: o.hasAvatar, Source: sourceLabel(o.sourceType),
				}
				if directory {
					item.Empl = d.Employee
					item.Color, item.Parsed = vipColor(o.title, vips)
				} else {
					// Robin overlay: show the live occupant and keep the stored
					// desk configuration so the editor edits the real item.
					item.Empl = o.name
					if item.Empl == "" {
						item.Empl = fullname
					}
					item.ConfigType = d.Desktype
					item.ConfigEmpl = d.Employee
					item.ConfigAvtr = d.Avatar
				}
				searchName := o.name
				if searchName == "" {
					searchName = fullname
				}
				app.appendIfMatch(&items, item, search, searchName)
			}
			continue
		}

		// Empty AD-mirrored desk: keep the stored placeholder (renders as free).
		if d.Desktype == "addesk" {
			item := deskItem{
				Map: mapName, ID: d.ID, Desktype: "addesk", X: d.X, Y: d.Y,
				Dsk: d.Desknumber, Empl: d.Employee, Avtr: d.Avatar, Dept: d.Department,
				Booked: booked, HasAvatar: true,
			}
			app.appendIfMatch(&items, item, search, "")
			continue
		}

		// Non-AD desk (localdesk, booking, meeting, fixtures, ...).
		item := deskItem{
			Map: mapName, ID: d.ID, Desktype: d.Desktype, X: d.X, Y: d.Y,
			Dsk: d.Desknumber, Empl: d.Employee, Avtr: d.Avatar, Dept: d.Department,
			Booked: booked, Bookdata: bd, HasAvatar: true,
		}
		searchName := ""
		if bd != nil {
			searchName = bd.Name
		}
		app.appendIfMatch(&items, item, search, searchName)
	}
	return items
}

// sourceLabel maps an internal source type to the label the client uses for the
// per-source name badge ("ldap"/"entraid"/"robin").
func sourceLabel(sourceType string) string {
	switch sourceType {
	case "ldap":
		return "ldap"
	case "entra":
		return "entraid"
	case "robin":
		return "robin"
	}
	return ""
}

// sameRobinPerson reports whether the Robin desk occupant is the same individual
// as an AD-mirrored user, comparing the LDAP userid first and then the primary
// mail or any of its aliases (case-insensitively).
func sameRobinPerson(rs RobinDeskStatus, u LdapUser) bool {
	if id := strings.ToLower(strings.TrimSpace(rs.Userid)); id != "" {
		if id == strings.ToLower(strings.TrimSpace(u.Userid)) {
			return true
		}
	}
	mail := strings.ToLower(strings.TrimSpace(rs.Mail))
	if mail == "" {
		return false
	}
	if mail == strings.ToLower(strings.TrimSpace(u.Mail)) {
		return true
	}
	for _, a := range u.Aliases {
		if mail == strings.ToLower(strings.TrimSpace(a)) {
			return true
		}
	}
	return false
}

// appendIfMatch appends item unless a search filter is set and none of its
// pipe-separated tokens match the desk's searchable fields.
func (app *App) appendIfMatch(items *[]deskItem, item deskItem, search, extraName string) {
	if search == "" {
		*items = append(*items, item)
		return
	}
	fullname := strings.TrimSpace(item.Fname + " " + item.Lname)
	combined := strings.ToLower(strings.Join([]string{item.Desktype, item.Dsk, item.Empl, fullname, extraName}, ","))
	for _, token := range strings.Split(search, "|") {
		token = strings.ToLower(strings.TrimSpace(token))
		if token != "" && strings.Contains(combined, token) {
			*items = append(*items, item)
			return
		}
	}
}

// vipColor returns the border color (and optional "parsed" label for Directors)
// for a job title, honoring the configured VIP rules.
func vipColor(title string, vips []VIP) (color, parsed string) {
	for _, v := range vips {
		switch v.Type {
		case "TeamManager":
			if v.Title != "" && strings.Contains(strings.ToLower(title), strings.ToLower(v.Title)) {
				color = "#00CC00"
			}
		case "Director":
			if v.Title != "" && strings.Contains(strings.ToLower(title), strings.ToLower(v.Title)) {
				color = "#00bbff"
				parsed = v.Title + " / " + title
			}
		case "VP":
			if v.Title != "" && strings.Contains(strings.ToLower(title), strings.ToLower(v.Title)) {
				color = "#800080"
			}
		case "Board":
			if title == v.Title {
				color = "#ffa500"
			}
		}
	}
	return color, parsed
}
