package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// appVersion is the CompanyMaps version, surfaced on the dashboard. Bump this
// manually after meaningful changes.
const appVersion = "9.2"

// buildDate returns the date the running binary was built, derived from the
// executable's modification time (go build sets it on compile; update.sh's mv
// preserves it). Returns "unknown" when it can't be determined.
func buildDate() string {
	if exe, err := os.Executable(); err == nil {
		if fi, err := os.Stat(exe); err == nil {
			return fi.ModTime().Format("2006-01-02")
		}
	}
	return "unknown"
}

// intgHealthResult is one integration's most recent connectivity-test outcome.
type intgHealthResult struct {
	OK      bool
	Message string
	Checked time.Time
}

// startIntegrationHealthScheduler runs the integration connectivity tests once
// shortly after boot and then on the given interval (hourly), caching the
// results for the dashboard. The tests are read-only pre-flight checks and never
// trigger a sync.
func (app *App) startIntegrationHealthScheduler(first, interval time.Duration) {
	go func() {
		timer := time.NewTimer(first)
		defer timer.Stop()
		for range timer.C {
			app.runIntegrationTests()
			timer.Reset(interval)
		}
	}()
}

// runIntegrationTests probes every configured sync integration and replaces the
// cached result map.
func (app *App) runIntegrationTests() {
	results := map[string]intgHealthResult{
		"ldap":  app.testLdapIntegration(),
		"entra": app.testEntraIntegration(),
		"robin": app.testRobinIntegration(),
		"saml":  app.testSamlIntegration(),
	}
	app.intgHealthMu.Lock()
	app.intgHealth = results
	app.intgHealthMu.Unlock()
}

// firstFailDetail returns the detail of the first failing check in a testResult
// payload ({ok, checks:[]testCheck}), or "" when none failed.
func firstFailDetail(res map[string]interface{}) string {
	checks, _ := res["checks"].([]testCheck)
	for _, c := range checks {
		if c.Status == "fail" {
			return c.Detail
		}
	}
	return ""
}

func (app *App) testLdapIntegration() intgHealthResult {
	srcs, _ := app.db.ListLdapSources()
	enabled, okCount := 0, 0
	var fails []string
	for _, s := range srcs {
		if s.Disabled {
			continue
		}
		enabled++
		res := app.ldapValidate(s.ID)
		if ok, _ := res["ok"].(bool); ok {
			okCount++
		} else {
			label := strings.TrimSpace(s.Description)
			if label == "" {
				label = s.Server
			}
			fails = append(fails, label+": "+firstFailDetail(res))
		}
	}
	if enabled == 0 {
		return intgHealthResult{OK: false, Message: "No enabled connections.", Checked: time.Now()}
	}
	if len(fails) == 0 {
		return intgHealthResult{OK: true, Message: fmt.Sprintf("%d/%d connection(s) reachable.", okCount, enabled), Checked: time.Now()}
	}
	return intgHealthResult{OK: false, Message: strings.Join(fails, " · "), Checked: time.Now()}
}

func (app *App) testEntraIntegration() intgHealthResult {
	srcs, _ := app.db.ListEntraSources()
	enabled, okCount := 0, 0
	var fails []string
	for _, s := range srcs {
		if s.Disabled {
			continue
		}
		enabled++
		res := app.entraValidate(s.ID)
		if ok, _ := res["ok"].(bool); ok {
			okCount++
		} else {
			label := strings.TrimSpace(s.Description)
			if label == "" {
				label = s.TenantID
			}
			fails = append(fails, label+": "+firstFailDetail(res))
		}
	}
	if enabled == 0 {
		return intgHealthResult{OK: false, Message: "No enabled connections.", Checked: time.Now()}
	}
	if len(fails) == 0 {
		return intgHealthResult{OK: true, Message: fmt.Sprintf("%d/%d connection(s) reachable.", okCount, enabled), Checked: time.Now()}
	}
	return intgHealthResult{OK: false, Message: strings.Join(fails, " · "), Checked: time.Now()}
}

func (app *App) testRobinIntegration() intgHealthResult {
	if !app.robinEnabled() {
		return intgHealthResult{OK: false, Message: "Robin is disabled.", Checked: time.Now()}
	}
	token := strings.TrimSpace(app.db.GetRobinSetting("robintoken"))
	org := strings.TrimSpace(app.db.GetRobinSetting("robinOrganisation"))
	if token == "" || org == "" {
		return intgHealthResult{OK: false, Message: "Access token or organisation id not configured.", Checked: time.Now()}
	}
	locs, err := app.robinListLocations()
	if err != nil {
		return intgHealthResult{OK: false, Message: "Robin API rejected the request: " + err.Error(), Checked: time.Now()}
	}
	return intgHealthResult{OK: true, Message: fmt.Sprintf("Robin API reachable (%d location(s)).", len(locs)), Checked: time.Now()}
}

func (app *App) testSamlIntegration() intgHealthResult {
	cfg := app.cfg.SAML
	if !cfg.Enabled {
		return intgHealthResult{OK: false, Message: "SAML is disabled.", Checked: time.Now()}
	}
	if cfg.EntraLoginURL == "" {
		return intgHealthResult{OK: false, Message: "No IdP login URL configured.", Checked: time.Now()}
	}
	if cfg.EntraX509Certificate == "" {
		return intgHealthResult{OK: false, Message: "No IdP signing certificate configured.", Checked: time.Now()}
	}
	// Reachability check: prefer the federation metadata URL (returns IdP
	// metadata), else confirm the login endpoint responds.
	url := cfg.FederationMetadataURL
	if url == "" {
		url = cfg.EntraLoginURL
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return intgHealthResult{OK: false, Message: "IdP endpoint unreachable: " + err.Error(), Checked: time.Now()}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return intgHealthResult{OK: false, Message: fmt.Sprintf("IdP endpoint returned HTTP %d.", resp.StatusCode), Checked: time.Now()}
	}
	return intgHealthResult{OK: true, Message: "IdP endpoint reachable; configuration present.", Checked: time.Now()}
}

// handleRestDashboard returns the consolidated dashboard payload: system info,
// sync-integration status (with the cached hourly connectivity test), overview
// counts, data-consistency totals and the last 7 days of visitors.
func (app *App) handleRestDashboard(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || (app.permLevel(sess, "dashboard") < 1 && app.permLevel(sess, "adminpanel") < 1 && app.permLevel(sess, "health") < 1) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	app.intgHealthMu.Lock()
	health := app.intgHealth
	app.intgHealthMu.Unlock()

	out := map[string]interface{}{
		"system":       app.dashboardSystem(),
		"integrations": app.dashboardIntegrations(health),
		"overview":     app.dashboardOverview(),
		"visitors":     app.dashboardVisitors(),
	}

	ldapErrs, _, _ := app.checkLdapConsistency()
	deskErrs, _, _ := app.checkDeskConsistency()
	out["health"] = map[string]interface{}{"ldap": ldapErrs, "desks": deskErrs}

	writeJSON(w, out)
}

// dashboardPermitted enforces the shared read permission for every dashboard
// section endpoint (dashboard, adminpanel or health).
func (app *App) dashboardPermitted(r *http.Request) bool {
	sess, ok := app.currentSession(r)
	return ok && (app.permLevel(sess, "dashboard") >= 1 || app.permLevel(sess, "adminpanel") >= 1 || app.permLevel(sess, "health") >= 1)
}

// The dashboard sections are served as independent endpoints so each card can be
// fetched in parallel and faded in the moment its own data is ready, instead of
// blocking the whole dashboard on the slowest query.

func (app *App) handleRestDashboardOverview(w http.ResponseWriter, r *http.Request) {
	if !app.dashboardPermitted(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, map[string]interface{}{"overview": app.dashboardOverview()})
}

func (app *App) handleRestDashboardSystem(w http.ResponseWriter, r *http.Request) {
	if !app.dashboardPermitted(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, map[string]interface{}{"system": app.dashboardSystem()})
}

func (app *App) handleRestDashboardIntegrations(w http.ResponseWriter, r *http.Request) {
	if !app.dashboardPermitted(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	app.intgHealthMu.Lock()
	health := app.intgHealth
	app.intgHealthMu.Unlock()
	writeJSON(w, map[string]interface{}{"integrations": app.dashboardIntegrations(health)})
}

func (app *App) handleRestDashboardVisitors(w http.ResponseWriter, r *http.Request) {
	if !app.dashboardPermitted(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, map[string]interface{}{"visitors": app.dashboardVisitors()})
}

func (app *App) dashboardSystem() map[string]interface{} {
	host, _ := os.Hostname()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	sys := map[string]interface{}{
		"hostname":   host,
		"os":         runtime.GOOS + " / " + runtime.GOARCH,
		"goVersion":  runtime.Version(),
		"appVersion": appVersion,
		"buildDate":  buildDate(),
		"uptime":     humanDuration(time.Since(app.startTime)),
		"numCPU":     runtime.NumCPU(),
		"goroutines": runtime.NumGoroutine(),
		"heapAlloc":  humanBytes(ms.Alloc),
		"dataDir":    app.cfg.DataDir,
		"serverTime": app.nowLocal(),
		"cpuPct":     cpuLoad(),
	}

	if vm, err := mem.VirtualMemory(); err == nil {
		sys["memPct"] = fmt.Sprintf("%.1f", vm.UsedPercent)
		sys["memUsed"] = humanBytes(vm.Used)
		sys["memTotal"] = humanBytes(vm.Total)
	} else {
		sys["memPct"] = "0"
	}

	path := app.cfg.DataDir
	if path == "" {
		path = "/"
	}
	if du, err := disk.Usage(path); err == nil {
		sys["diskPct"] = fmt.Sprintf("%.1f", du.UsedPercent)
		sys["diskUsed"] = humanBytes(du.Used)
		sys["diskFree"] = humanBytes(du.Free)
		sys["diskTotal"] = humanBytes(du.Total)
	} else {
		sys["diskPct"] = "0"
	}

	return sys
}

func (app *App) dashboardIntegrations(health map[string]intgHealthResult) []map[string]interface{} {
	fill := func(o map[string]interface{}, key string) {
		if r, ok := health[key]; ok {
			o["testDone"] = true
			o["testOk"] = r.OK
			o["testMessage"] = r.Message
			if !r.Checked.IsZero() {
				o["checked"] = r.Checked.In(app.db.Location()).Format("2006-01-02 15:04:05")
			}
		} else {
			o["testDone"] = false
		}
	}

	// LDAP
	ldapSrcs, _ := app.db.ListLdapSources()
	ldapLast := ""
	for _, s := range ldapSrcs {
		if s.LastSync > ldapLast {
			ldapLast = s.LastSync
		}
	}
	ldapEnabled := app.anyLdapSourceEnabled()
	ldap := map[string]interface{}{
		"key":        "ldap",
		"name":       "LDAP / Active Directory",
		"configured": len(ldapSrcs) > 0,
		"enabled":    ldapEnabled,
		"count":      len(ldapSrcs),
		"lastSync":   ldapLast,
		"nextSync":   app.nextSyncLabel(app.getNextSync(&app.nextLdapSync), ldapEnabled),
	}
	fill(ldap, "ldap")

	// EntraID
	entraSrcs, _ := app.db.ListEntraSources()
	entraEnabled := app.entraHasEnabledSource()
	entra := map[string]interface{}{
		"key":        "entra",
		"name":       "Microsoft EntraID",
		"configured": len(entraSrcs) > 0,
		"enabled":    entraEnabled,
		"count":      len(entraSrcs),
		"lastSync":   app.db.GetEntraSetting("entraLastSync"),
		"nextSync":   app.nextSyncLabel(app.getNextSync(&app.nextEntraSync), entraEnabled),
	}
	fill(entra, "entra")

	// SAML (authentication only — no scheduled sync)
	samlCfg := app.cfg.SAML
	saml := map[string]interface{}{
		"key":        "saml",
		"name":       "SAML single sign-on",
		"configured": samlCfg.EntraLoginURL != "" || samlCfg.FederationMetadataURL != "",
		"enabled":    samlCfg.Enabled,
		"lastSync":   "",
		"nextSync":   "",
	}
	fill(saml, "saml")

	// Robin
	robinToken := strings.TrimSpace(app.db.GetRobinSetting("robintoken"))
	robinEnabled := app.robinEnabled() && robinToken != ""
	robinLast := ""
	if res, ok := app.LastRobinSyncResult(); ok {
		robinLast = res.Time
	}
	robin := map[string]interface{}{
		"key":        "robin",
		"name":       "Robin meeting rooms & desks",
		"configured": robinToken != "",
		"enabled":    robinEnabled,
		"lastSync":   robinLast,
		"nextSync":   app.nextSyncLabel(app.getNextSync(&app.nextRobinSync), robinEnabled),
	}
	fill(robin, "robin")

	return []map[string]interface{}{ldap, entra, saml, robin}
}

func (app *App) dashboardOverview() map[string]interface{} {
	maps, _ := app.db.ListMaps()
	desks, _ := app.db.ListAllDesks()
	dir, _ := app.db.ListDirectory()
	entraUsers, _ := app.db.ListEntraLdap()
	rooms, _ := app.db.ListRobinSpaces()
	teams, _ := app.db.ListTeams()
	users, _ := app.db.ListUsers()
	itemTypes, _ := app.db.ListItemTypes()

	return map[string]interface{}{
		"maps":           len(maps),
		"desks":          len(desks),
		"directoryUsers": len(dir),
		"entraUsers":     len(entraUsers),
		"rooms":          len(rooms),
		"teams":          len(teams),
		"admins":         len(users),
		"itemTypes":      len(itemTypes),
	}
}

// dashboardVisitors returns the last 7 calendar days of visitor counts (oldest
// first), filling missing days with zero.
func (app *App) dashboardVisitors() []map[string]interface{} {
	stats, _ := app.db.ListStats()
	byDate := map[string]int64{}
	for _, s := range stats {
		byDate[s.Date] = s.Count
	}
	now := time.Now().In(app.db.Location())
	out := make([]map[string]interface{}, 0, 7)
	for i := 6; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		key := d.Format("2006-01-02")
		out = append(out, map[string]interface{}{
			"date":  key,
			"label": d.Format("Mon"),
			"count": byDate[key],
		})
	}
	return out
}

func (app *App) nowLocal() string {
	return time.Now().In(app.db.Location()).Format("2006-01-02 15:04:05")
}

// humanBytes formats a byte count as a human-readable string (KB/MB/GB/TB).
func humanBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// humanDuration formats a duration as a compact "Nd Nh Nm" string.
func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	parts := []string{}
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if mins > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%dm", mins))
	}
	return strings.Join(parts, " ")
}
