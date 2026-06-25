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

// checkLdapConsistency reports LDAP-mirror desks occupied by more than four
// people. Mirrors the legacy per-row behaviour (one entry per offending person).
func (app *App) checkLdapConsistency() (int, []map[string]interface{}, []string) {
	whitelist := app.whitelistFor("ldap")
	mirror, _ := app.db.ListLdap()

	counts := map[string]int{}
	for _, u := range mirror {
		counts[u.Office]++
	}

	list := []map[string]interface{}{}
	for _, u := range mirror {
		if u.Office == "" || contains(whitelist, u.Office) {
			continue
		}
		if counts[u.Office] > 4 {
			list = append(list, map[string]interface{}{
				"desk":  u.Office,
				"count": counts[u.Office],
				"name":  strings.TrimSpace(u.Givenname + " " + u.Surname),
			})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i]["desk"].(string) < list[j]["desk"].(string)
	})
	return len(list), list, whitelist
}

// checkDeskConsistency reports duplicate desk names within each map.
func (app *App) checkDeskConsistency() (int, []map[string]interface{}, []string) {
	whitelist := app.whitelistFor("desks")
	maps, _ := app.db.ListMaps()

	list := []map[string]interface{}{}
	deskErrors := 0
	for _, m := range maps {
		if m.Mapname == "overview" {
			continue
		}
		desks, _ := app.db.ListDesks(m.Mapname)
		counts := map[string]int{}
		for _, d := range desks {
			counts[d.Desknumber]++
		}
		for _, d := range desks {
			if contains(whitelist, d.Desknumber) {
				continue
			}
			if counts[d.Desknumber] > 1 {
				deskErrors += counts[d.Desknumber]
				list = append(list, map[string]interface{}{
					"desk":  d.Desknumber,
					"count": counts[d.Desknumber],
					"map":   m.Mapname,
				})
			}
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i]["desk"].(string) < list[j]["desk"].(string)
	})
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
