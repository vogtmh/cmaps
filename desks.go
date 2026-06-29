package main

import (
	"net/http"
	"sort"
	"strings"
)

// deskItem is one entry in the /rest/desks response. Field names and the
// "0"/"1" booked string match the legacy PHP API consumed by user.js.
type deskItem struct {
	Map      string    `json:"map"`
	ID       int       `json:"id,string"`
	Desktype string    `json:"desktype"`
	X        int       `json:"x"`
	Y        int       `json:"y"`
	Dsk      string    `json:"dsk"`
	Empl     string    `json:"empl"`
	Avtr     string    `json:"avtr"`
	Dept     string    `json:"dept"`
	Fname    string    `json:"fname"`
	Lname    string    `json:"lname"`
	Phone    string    `json:"phone"`
	Mail     string    `json:"mail"`
	Title    string    `json:"title"`
	Mobil    string    `json:"mobil"`
	Color    string    `json:"color"`
	Parsed   string    `json:"parsed"`
	Booked   string    `json:"booked"`
	Robin    string    `json:"robin,omitempty"`
	// HasAvatar tells the client whether a cached avatar image exists for this
	// desk's occupant. When false the client uses a single shared placeholder URL
	// (downloaded once) instead of requesting a unique missing image per person.
	HasAvatar bool      `json:"hasavatar"`
	Bookdata *bookData `json:"bookdata,omitempty"`
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
	ldap, _ := app.db.ListLdap()

	out := struct {
		Desks []deskItem `json:"desks"`
	}{Desks: []deskItem{}}

	if mapName != "" {
		date := userDate
		if date == "" {
			date = app.db.MapToday(mapName)
		}
		out.Desks = append(out.Desks, app.buildMapDesks(mapName, date, search, vips, bookings, ldap)...)
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
		out.Desks = append(out.Desks, app.buildMapDesks(m.Mapname, date, search, vips, bookings, ldap)...)
	}
	writeJSON(w, out)
}

// buildMapDesks expands one map's desks into output items, joining AD-mirrored
// (addesk) desks against the LDAP mirror and applying VIP border colors.
func (app *App) buildMapDesks(mapName, date, search string, vips []VIP, bookings []Booking, ldap []LdapUser) []deskItem {
	desks, _ := app.db.ListDesks(mapName)
	var items []deskItem

	// Robin live-occupancy overlay (kept fully separate from native booking).
	// Empty unless the overlay is enabled and Robin reports occupied desks for
	// this map; free desks are therefore never affected.
	robinMode := app.db.GetRobinSetting("robinDeskMode")
	robinStatus := map[string]RobinDeskStatus{}
	if robinMode == "all" || robinMode == "unused" {
		sts, _ := app.db.ListRobinDeskStatus(mapName)
		for _, s := range sts {
			robinStatus[strings.ToLower(strings.TrimSpace(s.Desknumber))] = s
		}
	}

	// Per-user avatar availability (from the DB flag set during sync) so desks
	// for people without a cached avatar can point at a single shared placeholder.
	avatarByUser := map[string]bool{}
	for _, u := range ldap {
		if u.HasAvatar {
			avatarByUser[strings.ToLower(u.Userid)] = true
		}
	}

	for _, d := range desks {
		// Booking lookup for this desk on the given date.
		var booked string = "0"
		var bd *bookData
		for _, b := range bookings {
			if b.Date == date && b.Map == mapName && b.Desk == d.Desknumber {
				booked = "1"
				bd = &bookData{Name: b.Fullname, Phone: b.Phone, Mail: b.Mail}
			}
		}

		// Robin overlay: a desk occupied in Robin right now is shown as occupied,
		// overriding its native state (including multi-user shared desks, which
		// collapse to a single occupied item). "all" overlays any occupied desk;
		// "unused" only overlays desks with no native occupant and no booking.
		if rs, ok := robinStatus[strings.ToLower(strings.TrimSpace(d.Desknumber))]; ok {
			show := robinMode == "all"
			if robinMode == "unused" {
				hasUser := strings.TrimSpace(d.Employee) != ""
				if d.Desktype == "addesk" {
					hasUser = false
					for _, u := range ldap {
						if u.Office == d.Desknumber {
							hasUser = true
							break
						}
					}
				}
				show = !hasUser && booked == "0"
			}
			// If Robin reports the very same person who is already the AD-mirrored
			// occupant of this desk, the overlay would be redundant — leave the
			// native addesk untouched. A different Robin occupant still overrides.
			if show && d.Desktype == "addesk" {
				for _, u := range ldap {
					if u.Office != d.Desknumber {
						continue
					}
					if sameRobinPerson(rs, u) {
						show = false
						break
					}
				}
			}
			if show {
				avtr := rs.Userid
				if avtr == "" {
					avtr = "noavatar"
				}
				item := deskItem{
					Map: mapName, ID: d.ID, Desktype: "occupied", X: d.X, Y: d.Y,
					Dsk: d.Desknumber, Empl: rs.Name, Avtr: avtr, Dept: d.Department,
					Phone: rs.Phone, Mail: rs.Mail, Title: rs.Title, Mobil: rs.Mobile,
					Booked: booked, Robin: "1", HasAvatar: avatarByUser[strings.ToLower(rs.Userid)],
				}
				app.appendIfMatch(&items, item, search, rs.Name)
				continue
			}
		}

		if d.Desktype == "addesk" {
			// Collect up to 4 mirrored users seated at this desk.
			var matches []LdapUser
			for _, u := range ldap {
				if u.Office == d.Desknumber {
					matches = append(matches, u)
					if len(matches) >= 4 {
						break
					}
				}
			}
			if len(matches) == 0 {
				item := deskItem{
					Map: mapName, ID: d.ID, Desktype: "addesk", X: d.X, Y: d.Y,
					Dsk: d.Desknumber, Empl: d.Employee, Avtr: d.Avatar, Dept: d.Department,
					Booked: booked, HasAvatar: true,
				}
				app.appendIfMatch(&items, item, search, "")
				continue
			}
			desktype := "addesk"
			if len(matches) > 1 {
				desktype = "shareddesk"
			}
			for _, u := range matches {
				fullname := strings.TrimSpace(u.Givenname + " " + u.Surname)
				color, parsed := vipColor(u.Description, vips)
				item := deskItem{
					Map: mapName, ID: d.ID, Desktype: desktype, X: d.X, Y: d.Y,
					Dsk: d.Desknumber, Empl: d.Employee, Avtr: u.Userid, Dept: d.Department,
					Fname: u.Givenname, Lname: u.Surname, Phone: u.Telephonenumber, Mail: u.Mail,
					Title: u.Description, Mobil: u.Mobile, Color: color, Parsed: parsed, Booked: booked,
					HasAvatar: u.HasAvatar,
				}
				app.appendIfMatch(&items, item, search, fullname)
			}
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
