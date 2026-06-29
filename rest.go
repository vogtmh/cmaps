package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"sort"
	"strings"
	"time"
)

// handleRestUsers serves /rest/users?search=&title= from the LDAP mirror.
func (app *App) handleRestUsers(w http.ResponseWriter, r *http.Request) {
	search := strings.ToLower(r.URL.Query().Get("search"))
	title := strings.ToLower(r.URL.Query().Get("title"))

	users, _ := app.db.ListLdap()
	out := struct {
		Users []map[string]string `json:"users"`
	}{Users: []map[string]string{}}

	for _, u := range users {
		if search != "" {
			hay := strings.ToLower(u.Givenname + " " + u.Surname + " " + u.Office)
			if !strings.Contains(hay, search) {
				continue
			}
		}
		if title != "" && !strings.Contains(strings.ToLower(u.Description), title) {
			continue
		}
		out.Users = append(out.Users, map[string]string{
			"givenname":      u.Givenname,
			"surname":        u.Surname,
			"phone":          u.Telephonenumber,
			"mail":           u.Mail,
			"desk":           u.Office,
			"samaccountname": u.Userid,
			"title":          u.Description,
		})
	}
	writeJSON(w, out)
}

// handleRestConfig serves /rest/config?mode=maps|mapflags.
func (app *App) handleRestConfig(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get("mode") {
	case "mapflags":
		flags := []string{}
		if entries, err := fs.ReadDir(app.staticFS, "countryflags"); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				if dot := strings.IndexByte(name, '.'); dot > 0 {
					name = name[:dot]
				}
				flags = append(flags, strings.ToLower(name))
			}
		}
		sort.Strings(flags)
		writeJSON(w, map[string][]string{"mapflags": flags})

	case "maps":
		maps, _ := app.db.ListMaps()
		sort.Slice(maps, func(i, j int) bool { return maps[i].Mapname < maps[j].Mapname })
		items := make([]map[string]interface{}, 0, len(maps))
		for i, m := range maps {
			items = append(items, map[string]interface{}{
				"id":          i + 1,
				"mapname":     m.Mapname,
				"displayname": m.DisplayName,
				"itemscale":   m.Itemscale,
				"published":   m.Published,
				"country":     m.Country,
				"timezone":    m.Timezone,
				"address":     m.Address,
				"x":           m.MapX,
				"y":           m.MapY,
				"lat":         m.Lat,
				"lon":         m.Lon,
			})
		}
		writeJSON(w, map[string]interface{}{"maps": items})

	default:
		writeJSON(w, map[string]interface{}{"error": []string{"Please specify a mode"}})
	}
}

// handleRestTeams serves /rest/teams. The JSON key is "members" (PHP column was
// teammembers); members are pipe-separated full names.
func (app *App) handleRestTeams(w http.ResponseWriter, r *http.Request) {
	teams, _ := app.db.ListTeams()
	items := make([]map[string]string, 0, len(teams))
	for _, t := range teams {
		items = append(items, map[string]string{
			"teamname": t.Teamname,
			"members":  t.Members,
		})
	}
	writeJSON(w, map[string]interface{}{"teams": items})
}

// handleRestAuditlog serves /rest/auditlog with server-side pagination and
// filtering so the (100k+ on production) audit log can be browsed via lazy
// scroll without loading the whole table into memory or the browser. Query
// params: offset, limit, type, time, user, info. Response: {entries, hasMore}.
func (app *App) handleRestAuditlog(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "auditlog") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	q := r.URL.Query()
	offset := atoiDefault(q.Get("offset"), 0)
	limit := atoiDefault(q.Get("limit"), 100)
	if limit < 1 || limit > 500 {
		limit = 100
	}
	entries, hasMore, _ := app.db.ListAuditPage(offset, limit, q.Get("type"), q.Get("time"), q.Get("user"), q.Get("info"))
	items := make([]map[string]string, 0, len(entries))
	for _, e := range entries {
		items = append(items, map[string]string{
			"timestamp": e.Timestamp,
			"type":      e.Type,
			"user":      e.User,
			"info":      e.Info,
		})
	}
	writeJSON(w, map[string]interface{}{"entries": items, "hasMore": hasMore})
}

// handleRestChanges serves /rest/changes (Title/Employee only). It supports two
// paging styles:
//   - legacy ?maxresults=N  : the sidebar fetches the N most recent changes.
//   - ?offset=O&limit=L&q=  : the full-screen modal lazy-loads pages of L rows,
//     optionally filtered by a server-side search term q.
//
// A hard 24-month cap is always applied: per GDPR we never expose personnel
// change history older than two years, regardless of the requested window. The
// response includes "hasMore" so the client knows whether to keep loading.
func (app *App) handleRestChanges(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	offset := atoiDefault(q.Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}
	// limit defaults to maxresults for backward compatibility (0 = no limit).
	limit := atoiDefault(q.Get("limit"), 0)
	if limit == 0 {
		limit = atoiDefault(q.Get("maxresults"), 0)
	}
	search := strings.ToLower(strings.TrimSpace(q.Get("q")))

	entries, _ := app.db.ListChangelog(0) // newest first
	cutoff := time.Now().AddDate(-2, 0, 0)
	out := struct {
		Changes []map[string]interface{} `json:"changes"`
		HasMore bool                     `json:"hasMore"`
	}{Changes: []map[string]interface{}{}}

	matched := 0 // count of rows passing all filters (across the whole set)
	for i, e := range entries {
		if e.Type != "Title" && e.Type != "Employee" {
			continue
		}
		// GDPR hard cap: skip anything older than 24 months.
		ts := time.Date(e.Year, time.Month(e.Month), e.Day, e.Hour, e.Minute, 0, 0, cutoff.Location())
		if ts.Before(cutoff) {
			continue
		}
		if search != "" {
			hay := strings.ToLower(e.Name + " " + e.Newvalue + " " + e.Oldvalue + " " + e.Type)
			if !strings.Contains(hay, search) {
				continue
			}
		}
		// Window the matched rows by offset/limit.
		if matched < offset {
			matched++
			continue
		}
		matched++
		if limit > 0 && len(out.Changes) >= limit {
			// There is at least one more matching row beyond this window.
			out.HasMore = true
			break
		}
		out.Changes = append(out.Changes, map[string]interface{}{
			"fullname":  e.Name,
			"avatar":    e.Avatar,
			"type":      e.Type,
			"oldvalue":  e.Oldvalue,
			"newvalue":  e.Newvalue,
			"timestamp": formatChangeTimestamp(e),
			"id":        len(entries) - i, // stable descending id
		})
	}
	writeJSON(w, out)
}

// formatChangeTimestamp renders "YYYY.MM.DD HH AM/PM" matching the PHP format.
func formatChangeTimestamp(e ChangelogEntry) string {
	hour := e.Hour
	suffix := "AM"
	if hour >= 12 {
		suffix = "PM"
		hour -= 12
	}
	return fmt.Sprintf("%04d.%02d.%02d %02d %s", e.Year, e.Month, e.Day, hour, suffix)
}

// handleRestStats serves /rest/stats. With ?interval=day|month|year it returns
// [{period,count}] newest-first; a POST records today's visit.
func (app *App) handleRestStats(w http.ResponseWriter, r *http.Request) {
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		// Write path: record a page-view visit.
		if _, ok := app.currentSession(r); ok {
			_ = app.db.AddVisit()
		}
		writeJSON(w, map[string]string{"stats added": "ok"})
		return
	}

	limit := atoiDefault(r.URL.Query().Get("limit"), 48)
	stats, _ := app.db.ListStats() // ascending by date
	if len(stats) == 0 {
		writeJSON(w, []interface{}{time.Now().Format("2006-01-02"), 0})
		return
	}

	// Sum counts into period buckets.
	sums := make(map[string]int64)
	var layout func(StatEntry) string
	switch interval {
	case "year":
		layout = func(s StatEntry) string { return fmt.Sprintf("%04d", s.Year) }
	case "month":
		layout = func(s StatEntry) string { return fmt.Sprintf("%04d-%02d", s.Year, s.Month) }
	default: // day
		layout = func(s StatEntry) string { return fmt.Sprintf("%04d-%02d-%02d", s.Year, s.Month, s.Day) }
	}
	earliest := stats[0]
	for _, s := range stats {
		sums[layout(s)] += s.Count
	}

	// Walk from today backwards, emitting contiguous periods (incl. zeros).
	type statItem struct {
		Period string `json:"period"`
		Count  int64  `json:"count"`
	}
	out := []statItem{}
	now := time.Now()
	switch interval {
	case "year":
		for y := now.Year(); y >= earliest.Year && len(out) < limit; y-- {
			out = append(out, statItem{fmt.Sprintf("%04d", y), sums[fmt.Sprintf("%04d", y)]})
		}
	case "month":
		y, m := now.Year(), int(now.Month())
		for len(out) < limit {
			if y < earliest.Year || (y == earliest.Year && m < earliest.Month) {
				break
			}
			p := fmt.Sprintf("%04d-%02d", y, m)
			out = append(out, statItem{p, sums[p]})
			m--
			if m == 0 {
				m = 12
				y--
			}
		}
	default: // day
		d := now
		earliestDate := time.Date(earliest.Year, time.Month(earliest.Month), earliest.Day, 0, 0, 0, 0, time.UTC)
		for len(out) < limit {
			cur := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
			if cur.Before(earliestDate) {
				break
			}
			p := d.Format("2006-01-02")
			out = append(out, statItem{p, sums[p]})
			d = d.AddDate(0, 0, -1)
		}
	}
	writeJSON(w, out)
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	return n
}
