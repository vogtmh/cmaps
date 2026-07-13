package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// seedDemoData implements setup PATH A ("Set up a new server"): it creates two
// demo locations from the embedded sample maps, populates them with desks, and
// creates ten sample employees (with avatars) wired into the LDAP mirror so the
// maps look populated. No MySQL or AD connection is required.
func (app *App) seedDemoData() error {
	db := app.db

	// 1. Default settings (config_general equivalents).
	settings := map[string]string{
		"apptitle":     "CompanyMaps",
		"logo_regular": "/static/images/cmaps-regular.png",
		"logo_hover":   "/static/images/cmaps-hover.png",
		"domain":       "demo",
		"teamsContact": "",
		"reportURL":    "",
		"nomapText":    "",
		"nomapLink":    "",
		// New installs default to the e-mail identifier (supported by LDAP,
		// EntraID and SAML SSO). The demo data carries both a samaccountname and
		// a mail address, so the migration assistant can switch modes freely.
		"identifier": "mail",
	}
	for k, v := range settings {
		if err := db.SetSetting(k, v); err != nil {
			return fmt.Errorf("seed setting %s: %w", k, err)
		}
	}

	// 2. Default roles (mirrors config_roles defaults).
	if err := app.seedDefaultRoles(); err != nil {
		return err
	}

	// 3. Departments.
	for _, d := range []string{"- none -", "Development", "Sales", "Marketing", "Finance", "HumanResources", "IT-Administration", "Support"} {
		_ = db.AddDepartment(d)
	}

	// 4. VIP title rules (for the colored borders).
	for _, v := range []VIP{
		{Title: "CEO", Type: "Board", Description: "really important persons are orange"},
		{Title: "team manager", Type: "TeamManager", Description: "Team Managers are green"},
		{Title: "director", Type: "Director", Description: "Directors have blue"},
		{Title: "vice president", Type: "VP", Description: "VPs show up in purple"},
		{Title: "Head of", Type: "VP", Description: "equals VP level"},
	} {
		_ = db.AddVip(v)
	}

	// 5. Copy embedded sample maps onto disk and register them.
	if err := app.copySampleMap("overview.png", "overview"); err != nil {
		return err
	}
	// Both demo locations reuse the same mansion floor plan; itemscale 2
	// matches the original demo's zoom so the desks line up with the rooms.
	if err := app.copySampleMap("mansion.png", "germany"); err != nil {
		return err
	}
	if err := app.copySampleMap("mansion.png", "usa"); err != nil {
		return err
	}
	maps := demoMaps()
	for _, m := range maps {
		if err := db.PutMap(m); err != nil {
			return fmt.Errorf("seed map %s: %w", m.Mapname, err)
		}
	}

	// 6. Lay out each location from the mansion floor plan (fixtures + AD desk
	// slots). Each location gets its own desk-number namespace ("DE-NN" /
	// "US-NN") so directory matching stays per-map.
	if _, err := app.putMansionDesks("germany", "DE"); err != nil {
		return err
	}
	if _, err := app.putMansionDesks("usa", "US"); err != nil {
		return err
	}

	// 7. Create the built-in demo directory source. It generates the sample
	// employees, wires them into a self-contained source (whose sync never
	// connects to a real directory and so can never fail) and registers it in
	// the priority list so it fills the demo desks.
	if err := app.seedDemoSource(); err != nil {
		return err
	}

	// 8. A demo team built from the first few generated employees.
	var members []string
	for _, d := range app.demoDirectoryUsers() {
		if len(members) >= 4 {
			break
		}
		members = append(members, d.Givenname+" "+d.Surname)
	}
	_ = db.PutTeam(Team{Teamname: "Demo Team", Members: strings.Join(members, "|")})

	if err := db.SetMeta("setup_done", "1"); err != nil {
		return err
	}
	if err := db.SetMeta("setup_mode", "demo"); err != nil {
		return err
	}
	_ = db.AuditLog("setup", "admin", "demo data seeded (PATH A)")
	return nil
}

// deskTemplate is one desk in the mansion demo floor plan. addesk slots leave
// Desknumber empty; it is filled in per-location by putMansionDesks.
type deskTemplate struct {
	Desktype   string
	X, Y       int
	Desknumber string
	Employee   string
	Avatar     string
}

// mansionLayout mirrors the original "mansion" demo map: room fixtures plus AD
// desk slots, in display coordinates for a 1600px-wide target at itemscale 2.
var mansionLayout = []deskTemplate{
	// Left wing offices.
	{Desktype: "addesk", X: 345, Y: 80},
	{Desktype: "addesk", X: 344, Y: 160},
	{Desktype: "addesk", X: 435, Y: 80},
	{Desktype: "addesk", X: 435, Y: 160},
	{Desktype: "addesk", X: 347, Y: 303},
	{Desktype: "addesk", X: 347, Y: 392},
	{Desktype: "addesk", X: 337, Y: 597},
	{Desktype: "addesk", X: 426, Y: 597},
	// Central open-plan desks.
	{Desktype: "addesk", X: 885, Y: 240},
	{Desktype: "addesk", X: 966, Y: 240},
	{Desktype: "addesk", X: 1051, Y: 240},
	{Desktype: "addesk", X: 885, Y: 320},
	{Desktype: "addesk", X: 966, Y: 320},
	{Desktype: "addesk", X: 1051, Y: 320},
	// Right wing offices.
	{Desktype: "addesk", X: 1440, Y: 250},
	{Desktype: "addesk", X: 1520, Y: 250},
	{Desktype: "addesk", X: 1440, Y: 340},
	{Desktype: "addesk", X: 1520, Y: 340},
	// Fixtures.
	{Desktype: "food", X: 693, Y: 327, Desknumber: "Food", Employee: "Lunchroom"},
	{Desktype: "meeting", X: 1253, Y: 318, Desknumber: "Meeting", Employee: "Presentation Room"},
	{Desktype: "restroom", X: 1515, Y: 510, Desknumber: "Restroom", Employee: "Men"},
	{Desktype: "printer", X: 431, Y: 289, Desknumber: "Printer 1", Employee: "PRT001"},
	{Desktype: "printer", X: 915, Y: 418, Desknumber: "Printer 2", Employee: "PRT002"},
	{Desktype: "printer", X: 1018, Y: 418, Desknumber: "Printer 3", Employee: "PRT003"},
	{Desktype: "keycardlock", X: 778, Y: 608, Desknumber: "Main Entry", Employee: "MainEntry"},
	{Desktype: "keycardlock", X: 1219, Y: 650, Desknumber: "Side Entry", Employee: "SideEntry"},
	{Desktype: "firstaid", X: 1028, Y: 577, Desknumber: "FirstAid", Employee: "Click to find helpers"},
	{Desktype: "service", X: 1384, Y: 847, Desknumber: "Service", Employee: "Garage"},
	{Desktype: "hotseat", X: 568, Y: 614, Desknumber: "HotSeat", Employee: "HotSeat", Avatar: "hotseat"},
	{Desktype: "blocked", X: 907, Y: 543, Desknumber: "Blocked", Employee: "Blocked", Avatar: "blocked"},
}

// putMansionDesks creates the mansion floor plan on mapName. addesk slots get a
// per-map desk number ("<prefix>-NN"); the returned slice lists those numbers in
// order so callers can seat employees in them. Fixtures are created as-is.
func (app *App) putMansionDesks(mapName, prefix string) ([]string, error) {
	var slots []string
	for i, t := range mansionLayout {
		d := Desk{
			ID:         i + 1,
			Map:        mapName,
			Desktype:   t.Desktype,
			X:          t.X,
			Y:          t.Y,
			Desknumber: t.Desknumber,
			Employee:   t.Employee,
			Avatar:     t.Avatar,
			Department: "- none -",
		}
		if t.Desktype == "addesk" {
			d.Desknumber = fmt.Sprintf("%s-%02d", prefix, len(slots)+1)
			d.Employee = "ldap-mirror"
			slots = append(slots, d.Desknumber)
		}
		if err := app.db.PutDesk(d); err != nil {
			return nil, err
		}
	}
	return slots, nil
}

// personSeed is one generated demo employee.
type personSeed struct {
	Given, Sur, Title, Dept, Phone, Mobile string
}

var demoGiven = []string{
	"Alice", "Bob", "Carol", "David", "Eve", "Frank", "Grace", "Heidi",
	"Ivan", "Judy", "Karl", "Linda", "Martin", "Nora", "Oscar", "Petra",
	"Quentin", "Rosa", "Steffen", "Tina", "Ulrich", "Vera", "Werner", "Xenia",
}

var demoSur = []string{
	"Anderson", "Brown", "Clark", "Davis", "Evans", "Foster", "Green", "Hughes",
	"Ingram", "Jones", "Keller", "Lehmann", "Meyer", "Neumann", "Olsen", "Peters",
	"Quandt", "Richter", "Schmidt", "Thomas", "Weber", "Vogel", "Wagner", "Zimmer",
}

// demoTitles pairs a job title with a department. Several titles contain the VIP
// keywords (director / team manager / vice president / head of) so the colored
// borders show up on the demo maps.
var demoTitles = []struct{ Title, Dept string }{
	{"Director of Engineering", "Development"},
	{"Software Engineer", "Development"},
	{"Team Manager Support", "Support"},
	{"Support Engineer", "Support"},
	{"Marketing Lead", "Marketing"},
	{"Product Designer", "Marketing"},
	{"Vice President Sales", "Sales"},
	{"Account Executive", "Sales"},
	{"Finance Analyst", "Finance"},
	{"Controller", "Finance"},
	{"Head of People", "HumanResources"},
	{"Recruiter", "HumanResources"},
	{"Systems Administrator", "IT-Administration"},
	{"DevOps Engineer", "IT-Administration"},
	{"QA Engineer", "Development"},
	{"Director of Sales", "Sales"},
	{"Sales Manager", "Sales"},
	{"Team Manager Marketing", "Marketing"},
}

// demoPerson deterministically builds the seq-th demo employee. region selects
// the phone-number style ("DE" or "US"); US numbers use the 555-01xx exchange
// reserved for fiction.
func demoPerson(seq int, region string) personSeed {
	g := demoGiven[(seq-1)%len(demoGiven)]
	s := demoSur[(seq*7)%len(demoSur)]
	t := demoTitles[(seq-1)%len(demoTitles)]
	var phone, mobile string
	if region == "DE" {
		phone = fmt.Sprintf("+49 30 5550%03d", seq)
		mobile = fmt.Sprintf("+49 151 5550%04d", seq)
	} else {
		phone = fmt.Sprintf("+1 555 01%02d", seq%100)
		mobile = fmt.Sprintf("+1 555 02%02d", seq%100)
	}
	return personSeed{Given: g, Sur: s, Title: t.Title, Dept: t.Dept, Phone: phone, Mobile: mobile}
}

// seatCounts returns how many people to seat at each of n desk slots: every slot
// gets one person except the last two (left free), and two interior slots are
// shared by three people each to demonstrate the shared-desk view.
func seatCounts(n int) []int {
	counts := make([]int, n)
	for i := range counts {
		counts[i] = 1
	}
	if n >= 2 {
		counts[n-1] = 0
		counts[n-2] = 0
	}
	for _, idx := range []int{8, 11} {
		if idx >= 0 && idx < n && counts[idx] == 1 {
			counts[idx] = 3
		}
	}
	return counts
}

// demoMaps returns the map records for the two demo locations plus the overview.
func demoMaps() []MapInfo {
	return []MapInfo{
		{Mapname: "overview", Itemscale: "1", Published: "yes", Country: "none", Timezone: "Europe/Berlin", Address: "none", MapX: 0, MapY: 0},
		{Mapname: "germany", Itemscale: "2", Published: "yes", Country: "de", Timezone: "Europe/Berlin", Address: "CompanyMaps Demo GmbH<br/>Musterstraße 1<br/>12345 Musterstadt<br/>Germany", MapX: 760, MapY: 158},
		{Mapname: "usa", Itemscale: "2", Published: "yes", Country: "us", Timezone: "America/New_York", Address: "CompanyMaps Demo Inc.<br/>123 Sample Street<br/>Anytown, CA 90210<br/>USA", MapX: 280, MapY: 250},
	}
}

// demoSourceID is the reserved LdapSource ID for the built-in demo source. It is
// deliberately high so it never collides with user-created sources (nextLdapID
// also skips demo sources).
const demoSourceID = 9001

// demoSlots returns the AD-desk numbers of one demo location in layout order,
// matching exactly what putMansionDesks assigns ("<prefix>-NN").
func demoSlots(prefix string) []string {
	var slots []string
	for _, t := range mansionLayout {
		if t.Desktype == "addesk" {
			slots = append(slots, fmt.Sprintf("%s-%02d", prefix, len(slots)+1))
		}
	}
	return slots
}

// demoDirectoryUsers regenerates the demo directory deterministically. Every
// person keeps BOTH a samaccountname (DEMOnnn) and a mail address, and their
// active Userid is computed for the CURRENT identifier mode, so the demo works in
// either mode and the identifier migration assistant can round-trip it.
func (app *App) demoDirectoryUsers() []DirectoryUser {
	var out []DirectoryUser
	seq := 0
	for _, loc := range []struct{ prefix, region string }{
		{"DE", "DE"},
		{"US", "US"},
	} {
		slots := demoSlots(loc.prefix)
		counts := seatCounts(len(slots))
		for slotIdx, n := range counts {
			for k := 0; k < n; k++ {
				seq++
				sam := fmt.Sprintf("DEMO%03d", seq)
				p := demoPerson(seq, loc.region)
				mail := fmt.Sprintf("%s.%s%d@demo.companymaps.local", lower(p.Given), lower(p.Sur), seq)
				out = append(out, DirectoryUser{
					Userid:         app.userIdentifier(sam, mail),
					Samaccountname: sam,
					Givenname:      p.Given,
					Surname:        p.Sur,
					Mail:           mail,
					Office:         slots[slotIdx],
					Department:     p.Dept,
					Title:          p.Title,
					Phone:          p.Phone,
					Mobile:         p.Mobile,
				})
			}
		}
	}
	return out
}

// demoSeq parses the running number from a demo samaccountname ("DEMO007" -> 7).
func demoSeq(sam string) int {
	n := 0
	fmt.Sscanf(sam, "DEMO%d", &n)
	if n <= 0 {
		n = 1
	}
	return n
}

// ensureDemoAvatars copies a bundled sample photo for every demo user that does
// not already have a cached avatar under its active identifier. Best effort: any
// copy error is ignored so a demo (re)sync can never fail on avatars.
func (app *App) ensureDemoAvatars(dir []DirectoryUser) {
	for _, d := range dir {
		id := strings.TrimSpace(d.Userid)
		if id == "" {
			continue
		}
		if _, err := os.Stat(app.cfg.DataPath("avatarcache", id+".jpg")); err == nil {
			continue
		}
		seq := demoSeq(d.Samaccountname)
		_ = app.copySampleAvatar(fmt.Sprintf("%02d.jpg", (seq-1)%10+1), id)
	}
}

// seedDemoSource (re)creates the built-in demo directory source: it regenerates
// the demo people and avatars, stores the per-source directory + desk-placement
// mirror the render engine reads, registers the source (idempotent, fixed ID)
// and rebuilds the combined caches. Used by the initial demo setup and by the
// admin "create demo data" button.
func (app *App) seedDemoSource() error {
	dir := app.demoDirectoryUsers()
	app.ensureDemoAvatars(dir)
	if err := app.db.PutLdapSource(LdapSource{
		ID: demoSourceID, Description: "Demo data", Type: "DEMO", Demo: true, LastSync: "never",
	}); err != nil {
		return err
	}
	if err := app.db.PutSourceDir(demoSourceID, dir); err != nil {
		return err
	}
	if err := app.db.PutSourceMirror("ldap", demoSourceID, deriveMirrorUsers(dir)); err != nil {
		return err
	}
	if _, err := app.rebuildLdapMirror(false); err != nil {
		return err
	}
	_ = app.db.AuditLog("LDAP", "System", "Demo data source created/refreshed")
	return nil
}

// createDemoData (re)creates the two demo locations (maps + desks) and the demo
// directory source. Used by the admin "create demo data" button when no real
// directory source is configured; safe to run repeatedly (it overwrites the demo
// maps/desks/people in place).
func (app *App) createDemoData() error {
	for _, d := range []string{"- none -", "Development", "Sales", "Marketing", "Finance", "HumanResources", "IT-Administration", "Support"} {
		_ = app.db.AddDepartment(d)
	}
	if err := app.copySampleMap("overview.png", "overview"); err != nil {
		return err
	}
	if err := app.copySampleMap("mansion.png", "germany"); err != nil {
		return err
	}
	if err := app.copySampleMap("mansion.png", "usa"); err != nil {
		return err
	}
	for _, m := range demoMaps() {
		if err := app.db.PutMap(m); err != nil {
			return err
		}
	}
	if _, err := app.putMansionDesks("germany", "DE"); err != nil {
		return err
	}
	if _, err := app.putMansionDesks("usa", "US"); err != nil {
		return err
	}
	return app.seedDemoSource()
}

// removeDemoData deletes the built-in demo data: the demo directory source and
// its caches, the two demo locations (maps + desks, the shared overview map is
// kept) and the demo avatars. Used by the admin "remove demo data" button once a
// real source is configured.
func (app *App) removeDemoData() error {
	_ = app.db.DeleteLdapSource(demoSourceID)
	_ = app.db.DeleteSourceMirror("ldap", demoSourceID)
	_ = app.db.DeleteSourceDir(demoSourceID)
	for _, d := range app.demoDirectoryUsers() {
		if id := strings.TrimSpace(d.Userid); id != "" {
			_ = removeFileIfExists(app.cfg.DataPath("avatarcache", id+".jpg"))
		}
	}
	for _, m := range []string{"germany", "usa"} {
		if desks, _ := app.db.ListDesks(m); desks != nil {
			for _, dk := range desks {
				_ = app.db.DeleteDesk(m, dk.ID)
			}
		}
		_ = app.db.DeleteMap(m)
		_ = removeFileIfExists(app.cfg.DataPath("maps", m+".png"))
	}
	_, err := app.rebuildLdapMirror(false)
	return err
}

// seedDefaultRoles installs the standard role set if no roles exist yet.
func (app *App) seedDefaultRoles() error {
	roles := []Role{
		{ID: 1, Rolename: "superadmin", Perms: fullPerms(2)},
		{ID: 2, Rolename: "admin", Perms: map[string]int{"desks": 2, "dashboard": 1, "config": 0, "ldap": 1, "maps": 0, "users": 1, "teams": 2, "stats": 1, "auditlog": 0, "health": 1, "adminpanel": 2}},
		{ID: 3, Rolename: "groupmanager", Perms: map[string]int{"dashboard": 1, "teams": 1}},
		{ID: 4, Rolename: "deskmaintainer", Perms: map[string]int{"desks": 2, "dashboard": 1, "health": 1, "adminpanel": 2}},
		{ID: 5, Rolename: "user", Perms: fullPerms(0)},
	}
	for _, r := range roles {
		if err := app.db.PutRole(r); err != nil {
			return err
		}
	}
	return nil
}

// permFeatures is the canonical list of permission features (config_roles columns).
var permFeatures = []string{"desks", "dashboard", "config", "ldap", "maps", "users", "teams", "stats", "auditlog", "health", "adminpanel"}

func fullPerms(level int) map[string]int {
	m := make(map[string]int, len(permFeatures))
	for _, f := range permFeatures {
		m[f] = level
	}
	return m
}

func (app *App) copySampleMap(sampleName, mapName string) error {
	data, err := fs.ReadFile(sampleFS, filepath.ToSlash(filepath.Join("sample/maps", sampleName)))
	if err != nil {
		return fmt.Errorf("reading sample map %s: %w", sampleName, err)
	}
	dst := app.cfg.DataPath("maps", mapName+".png")
	return os.WriteFile(dst, data, 0644)
}

func (app *App) copySampleAvatar(sampleName, userid string) error {
	data, err := fs.ReadFile(sampleFS, filepath.ToSlash(filepath.Join("sample/avatars", sampleName)))
	if err != nil {
		return fmt.Errorf("reading sample avatar %s: %w", sampleName, err)
	}
	dst := app.cfg.DataPath("avatarcache", userid+".jpg")
	return os.WriteFile(dst, data, 0644)
}

// --- small string/util helpers ---

func lower(s string) string { return toCase(s, false) }

func toCase(s string, upper bool) string {
	b := []byte(s)
	for i := range b {
		c := b[i]
		if upper && c >= 'a' && c <= 'z' {
			b[i] = c - 32
		} else if !upper && c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
