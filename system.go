package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// handleRestSystem serves /rest/system?healthdetails=. It reports host load,
// memory and disk usage plus data-consistency checks (LDAP mirror desks shared
// by more than four people and duplicate desk names). Requires the "health"
// feature permission.
func (app *App) handleRestSystem(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "health") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	details := r.URL.Query().Get("healthdetails") != ""

	out := map[string]interface{}{
		"cpuload":    cpuLoad(),
		"memoryused": memUsed(),
		"diskused":   diskUsed(app.cfg.DataDir),
	}

	ldapErrs, ldapList, ignoredLdap := app.checkLdapConsistency()
	deskErrs, deskList, ignoredDesks := app.checkDeskConsistency()
	out["consistency_ldap"] = ldapErrs
	out["consistency_desks"] = deskErrs

	if details {
		out["health"] = map[string]interface{}{
			"ldap":  ldapList,
			"desks": deskList,
		}
		out["ignoredLdap"] = ignoredLdap
		out["ignoredDesks"] = ignoredDesks
	}

	writeJSON(w, out)
}

func cpuLoad() string {
	pct, err := cpu.Percent(500*time.Millisecond, false)
	if err != nil || len(pct) == 0 {
		return "0.00"
	}
	return fmt.Sprintf("%.2f", pct[0])
}

func memUsed() string {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return "0.00"
	}
	return fmt.Sprintf("%.2f", vm.UsedPercent)
}

func diskUsed(path string) string {
	if path == "" {
		path = "/"
	}
	u, err := disk.Usage(path)
	if err != nil {
		if u2, err2 := disk.Usage("/"); err2 == nil {
			return fmt.Sprintf("%.2f", u2.UsedPercent)
		}
		return "0.00"
	}
	return fmt.Sprintf("%.2f", u.UsedPercent)
}

// checkLdapConsistency reports LDAP-mirror offices occupied by more than four
// people. Each over-occupied office is reported once; the error count is the
// number of people sharing it (so the dashboard total stays meaningful).
func (app *App) checkLdapConsistency() (int, []map[string]interface{}, []string) {
	whitelist := app.whitelistFor("ldap")
	mirror, _ := app.db.ListLdap()

	byOffice := map[string][]LdapUser{}
	for _, u := range mirror {
		if u.Office == "" {
			continue
		}
		byOffice[u.Office] = append(byOffice[u.Office], u)
	}

	list := []map[string]interface{}{}
	ldapErrors := 0
	for office, users := range byOffice {
		if contains(whitelist, office) {
			continue
		}
		if len(users) > 4 {
			ldapErrors += len(users)
			names := []string{}
			for _, u := range users {
				n := strings.TrimSpace(u.Givenname + " " + u.Surname)
				if n != "" {
					names = append(names, n)
				}
			}
			sort.Strings(names)
			list = append(list, map[string]interface{}{
				"desk":  office,
				"count": len(users),
				"name":  strings.Join(names, ", "),
				"names": names,
			})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i]["desk"].(string) < list[j]["desk"].(string)
	})
	return ldapErrors, list, whitelist
}

// deskGroup is a set of desks on the same map that share a (non-whitelisted)
// desk number. Callers report each group once and can highlight its members.
type deskGroup struct {
	Map     string
	Desk    string
	Members []Desk
}

// duplicateDeskGroups returns every group of desks that share the same desk
// number within a map (excluding the overview and whitelisted names). The group
// is the single source of truth for both the health report and the in-map
// highlight, so the two never disagree.
func (app *App) duplicateDeskGroups() ([]deskGroup, []string) {
	whitelist := app.whitelistFor("desks")
	maps, _ := app.db.ListMaps()

	var groups []deskGroup
	for _, m := range maps {
		if m.Mapname == "overview" {
			continue
		}
		desks, _ := app.db.ListDesks(m.Mapname)
		byName := map[string][]Desk{}
		order := []string{}
		for _, d := range desks {
			if _, seen := byName[d.Desknumber]; !seen {
				order = append(order, d.Desknumber)
			}
			byName[d.Desknumber] = append(byName[d.Desknumber], d)
		}
		for _, name := range order {
			members := byName[name]
			if contains(whitelist, name) || len(members) < 2 {
				continue
			}
			groups = append(groups, deskGroup{Map: m.Mapname, Desk: name, Members: members})
		}
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Map != groups[j].Map {
			return groups[i].Map < groups[j].Map
		}
		return groups[i].Desk < groups[j].Desk
	})
	return groups, whitelist
}

// checkDeskConsistency reports duplicate desk names within each map. Each
// duplicate name is reported once per map; the error count is the number of
// desks involved in the duplication (e.g. four desks named "Desk" => 4, not 16).
func (app *App) checkDeskConsistency() (int, []map[string]interface{}, []string) {
	groups, whitelist := app.duplicateDeskGroups()

	list := []map[string]interface{}{}
	deskErrors := 0
	for _, g := range groups {
		deskErrors += len(g.Members)
		members := []map[string]interface{}{}
		for _, d := range g.Members {
			members = append(members, map[string]interface{}{
				"id":         d.ID,
				"employee":   d.Employee,
				"department": d.Department,
				"x":          d.X,
				"y":          d.Y,
			})
		}
		list = append(list, map[string]interface{}{
			"desk":    g.Desk,
			"count":   len(g.Members),
			"map":     g.Map,
			"members": members,
		})
	}
	return deskErrors, list, whitelist
}

func (app *App) whitelistFor(kind string) []string {
	entries, _ := app.db.ListWhitelist()
	var out []string
	for _, e := range entries {
		if e.Type == kind {
			out = append(out, e.Text)
		}
	}
	return out
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
