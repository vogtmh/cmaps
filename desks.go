package main

import (
	"net/http"
	"sort"
	"strings"
)

// deskItem is one entry in the /rest/desks response. Field names and the
// "0"/"1" booked string match the legacy PHP API consumed by user80.js.
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
					Booked: booked,
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
				}
				app.appendIfMatch(&items, item, search, fullname)
			}
			continue
		}

		// Non-AD desk (localdesk, booking, meeting, fixtures, ...).
		item := deskItem{
			Map: mapName, ID: d.ID, Desktype: d.Desktype, X: d.X, Y: d.Y,
			Dsk: d.Desknumber, Empl: d.Employee, Avtr: d.Avatar, Dept: d.Department,
			Booked: booked, Bookdata: bd,
		}
		searchName := ""
		if bd != nil {
			searchName = bd.Name
		}
		app.appendIfMatch(&items, item, search, searchName)
	}
	return items
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
