package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// ldapSyncAttrs are the AD attributes the mirror needs.
var ldapSyncAttrs = []string{
	"givenname", "sn", "telephonenumber", "mail",
	"physicaldeliveryofficename", "samaccountname", "title", "department", "mobile",
	"proxyAddresses",
}

// SourceDebug records diagnostics for one AD source during a sync run so the
// admin panel can explain results like "0 users found".
type SourceDebug struct {
	Description    string         `json:"description"`
	Server         string         `json:"server"`
	Type           string         `json:"type"`
	OU             string         `json:"ou"`
	BindUser       string         `json:"bind_user"`
	Connected      bool           `json:"connected"`
	Bound          bool           `json:"bound"`
	EntriesFound   int            `json:"entries_found"`
	Mirrored       int            `json:"mirrored"`
	Skipped        int            `json:"skipped"`
	SkipReasons    map[string]int `json:"skip_reasons"`
	AttributeNames []string       `json:"attribute_names"`
	Error          string         `json:"error"`
}

// ADSyncDebug is the full diagnostic snapshot of the most recent sync.
type ADSyncDebug struct {
	When    string        `json:"when"`
	Total   int           `json:"total"`
	Sources []SourceDebug `json:"sources"`
}

// RunADSync mirrors all configured AD sources into the ldapmirror bucket,
// recording employee/title/name changes in the changelog. It returns the number
// of mirrored desk placements (one per office value; an office field may list
// several places separated by "|").
//
// This is the Go port of tools/ldap_connector.php (CLI path). MS Teams
// notifications from the original are intentionally dropped.
func (app *App) RunADSync() (int, error) {
	return app.runADSync(nil)
}

// runADSync is the worker behind RunADSync. When prog is non-nil it reports
// per-source progress and a live log so the admin Sync tab can render a progress
// bar during a manual sync.
func (app *App) runADSync(prog *syncProgress) (int, error) {
	sources, err := app.db.ListLdapSources()
	if err != nil {
		return 0, fmt.Errorf("loading AD sources: %w", err)
	}
	if len(sources) == 0 {
		return 0, fmt.Errorf("no AD sources configured")
	}
	// Skip deactivated sources: their placements are dropped from the mirror on
	// the next sync because ReplaceLdap rewrites the whole mirror below.
	enabled := sources[:0]
	for _, s := range sources {
		if !s.Disabled {
			enabled = append(enabled, s)
		}
	}
	sources = enabled
	if len(sources) == 0 {
		return 0, fmt.Errorf("no enabled AD sources configured")
	}
	if prog != nil {
		prog.setTotal(len(sources))
		prog.logf("Starting sync of %d source(s)…", len(sources))
	}

	// Snapshot the existing mirror, keyed by userid, for change detection.
	oldUsers, _ := app.db.ListLdap()
	oldByID := make(map[string]LdapUser, len(oldUsers))
	for _, u := range oldUsers {
		if _, ok := oldByID[u.Userid]; !ok {
			oldByID[u.Userid] = u
		}
	}

	now := time.Now().In(app.db.loc)

	var combined []LdapUser
	var allDir []DirectoryUser
	dirSeen := make(map[string]bool)
	seen := make(map[string]bool)
	debug := ADSyncDebug{When: now.Format("2006-01-02 15:04:05")}

	for _, src := range sources {
		if prog != nil {
			prog.setStage("Syncing " + src.Description)
			prog.logf("→ %s: connecting to %s (%s)…", src.Description, src.Server, src.Type)
		}
		dirUsers, dbg, err := app.fetchSourceDirectory(src)
		users := deriveMirrorUsers(dirUsers)
		dbg.Mirrored = len(users)
		debug.Sources = append(debug.Sources, dbg)
		if err != nil {
			log.Printf("AD sync: source %q: %v", src.Description, err)
			debug.Total = len(combined)
			app.setSyncDebug(debug)
			if prog != nil {
				prog.logf("   ✗ %s: %s", src.Description, err.Error())
				prog.step("")
			}
			return len(combined), fmt.Errorf("source %q: %w", src.Description, err)
		}
		if prog != nil {
			prog.logf("   connected=%v bound=%v · directory: %d · desk placements: %d",
				dbg.Connected, dbg.Bound, dbg.EntriesFound, dbg.Mirrored)
		}

		// Accumulate the full directory snapshot (deduplicated across sources).
		for _, d := range dirUsers {
			key := strings.ToLower(d.Userid)
			if dirSeen[key] {
				continue
			}
			dirSeen[key] = true
			allDir = append(allDir, d)
		}

		for _, u := range users {
			combined = append(combined, u)

			// Change detection is per unique user (first placement only).
			if seen[u.Userid] {
				continue
			}
			seen[u.Userid] = true

			fullname := strings.TrimSpace(u.Givenname + " " + u.Surname)
			old, known := oldByID[u.Userid]
			if !known {
				app.logChange(now, fullname, u.Userid, "Employee", "none", u.Description)
				continue
			}
			if old.Description != "" && old.Description != u.Description {
				app.logChange(now, fullname, u.Userid, "Title", old.Description, u.Description)
			}
			if old.Description != "" && old.Surname != u.Surname {
				app.logChange(now, fullname, u.Userid, "Name", old.Surname, u.Surname)
			}
		}

		src.LastSync = now.Format("2006-01-02 15:04:05")
		if err := app.db.PutLdapSource(src); err != nil {
			log.Printf("AD sync: updating LastSync for %q: %v", src.Description, err)
		}
		_ = app.db.AuditLog("LDAP", "System", src.Description+" has been synced.")
		if prog != nil {
			prog.step("")
		}
	}

	if err := app.db.ReplaceDirectory(allDir); err != nil {
		log.Printf("AD sync: writing directory: %v", err)
	}
	// Flag which mirrored users have a cached avatar on disk so the client can
	// point everyone without one at a single shared placeholder instead of
	// requesting a unique (missing) image per person.
	avatars := app.avatarFileSet()
	for i := range combined {
		combined[i].HasAvatar = avatars[strings.ToLower(combined[i].Userid)]
	}
	if err := app.db.ReplaceLdap(combined); err != nil {
		return len(combined), fmt.Errorf("writing mirror: %w", err)
	}
	// Refresh stored full names for existing admins from the fresh directory.
	app.refreshAdminNames(allDir)
	debug.Total = len(combined)
	app.setSyncDebug(debug)
	if prog != nil {
		prog.logf("Done. %d directory user(s), %d desk placement(s) from %d source(s).", len(allDir), len(combined), len(sources))
	}
	return len(combined), nil
}

// refreshAdminNames updates the stored Fullname of each admin user from the
// freshly fetched directory, so the Users tab shows live names even if the
// directory is later cleared. Usernames are matched on samaccountname after
// stripping any DOMAIN\ prefix.
func (app *App) refreshAdminNames(dir []DirectoryUser) (matched, updated int) {
	if len(dir) == 0 {
		return 0, 0
	}
	byID := make(map[string]DirectoryUser, len(dir))
	for _, d := range dir {
		byID[strings.ToLower(d.Userid)] = d
	}
	users, err := app.db.ListUsers()
	if err != nil {
		return 0, 0
	}
	for _, u := range users {
		sam := u.Username
		if idx := strings.LastIndex(sam, "\\"); idx >= 0 {
			sam = sam[idx+1:]
		}
		d, ok := byID[strings.ToLower(sam)]
		if !ok {
			continue
		}
		matched++
		name := d.DisplayName()
		if name != "" && name != u.Fullname {
			u.Fullname = name
			if d.Mail != "" {
				u.Mail = d.Mail
			}
			_ = app.db.PutUser(u)
			updated++
		}
	}
	return matched, updated
}

// setSyncDebug stores the most recent sync diagnostics (concurrency-safe).
func (app *App) setSyncDebug(d ADSyncDebug) {
	app.syncDebugMu.Lock()
	app.syncDebug = d
	app.syncDebugMu.Unlock()
}

// SyncDebug returns the most recent sync diagnostics (concurrency-safe).
func (app *App) SyncDebug() ADSyncDebug {
	app.syncDebugMu.Lock()
	defer app.syncDebugMu.Unlock()
	return app.syncDebug
}

// syncOneSource queries one AD source A-Z and returns the office-filtered mirror
// placements for that source. It is a thin wrapper that fetches the full
// directory for the source and then derives the mirror locally, so callers that
// only need the mirror (e.g. the per-source "Sync now" button) keep working.
func (app *App) syncOneSource(src LdapSource) ([]LdapUser, SourceDebug, error) {
	dir, dbg, err := app.fetchSourceDirectory(src)
	if err != nil {
		return nil, dbg, err
	}
	mirror := deriveMirrorUsers(dir)
	dbg.Mirrored = len(mirror)
	return mirror, dbg, nil
}

// fetchSourceDirectory queries one AD source A-Z and returns EVERY enabled user
// account it finds (regardless of the office attribute) as DirectoryUser records.
// This full snapshot powers admin autocomplete and name resolution, and is the
// local data the office-filtered mirror is derived from. Disabled accounts are
// excluded via the userAccountControl bit filter.
func (app *App) fetchSourceDirectory(src LdapSource) ([]DirectoryUser, SourceDebug, error) {
	dbg := SourceDebug{
		Description: src.Description,
		Server:      src.Server,
		Type:        src.Type,
		OU:          src.OU,
		BindUser:    src.LdapUser,
		SkipReasons: map[string]int{},
	}

	conn, err := dialLDAP(src)
	if err != nil {
		dbg.Error = err.Error()
		return nil, dbg, err
	}
	defer conn.Close()
	dbg.Connected = true

	if err := conn.Bind(src.LdapUser, src.LdapPass); err != nil {
		dbg.Error = "bind: " + err.Error()
		return nil, dbg, fmt.Errorf("bind: %w", err)
	}
	dbg.Bound = true

	seen := map[string]bool{}
	var out []DirectoryUser
	for letter := 'A'; letter <= 'Z'; letter++ {
		// Every enabled person with a given name starting with this letter, no
		// office requirement (the office filter now happens locally).
		filter := fmt.Sprintf("(&(givenname=%c*)(!(userAccountControl:1.2.840.113556.1.4.803:=2)))", letter)
		req := ldap.NewSearchRequest(
			src.OU,
			ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
			0, 0, false,
			filter, ldapSyncAttrs, nil,
		)
		sr, err := conn.SearchWithPaging(req, 1000)
		if err != nil {
			dbg.Error = fmt.Sprintf("search %c: %v", letter, err)
			return nil, dbg, fmt.Errorf("search %c: %w", letter, err)
		}
		dbg.EntriesFound += len(sr.Entries)
		for _, e := range sr.Entries {
			// Record the attribute names AD returned for the first entry seen;
			// a case mismatch here is the usual cause of empty mirrors.
			if dbg.AttributeNames == nil && len(e.Attributes) > 0 {
				for _, a := range e.Attributes {
					dbg.AttributeNames = append(dbg.AttributeNames, a.Name)
				}
			}

			userid := e.GetEqualFoldAttributeValue("samaccountname")
			if userid == "" {
				dbg.Skipped++
				dbg.SkipReasons["no samaccountname"]++
				continue
			}
			if key := strings.ToLower(userid); seen[key] {
				continue // same account can match several letters via aliases
			} else {
				seen[key] = true
			}

			title := e.GetEqualFoldAttributeValue("title")
			if title == "" {
				title = "-"
			}
			out = append(out, DirectoryUser{
				Userid:     userid,
				Givenname:  e.GetEqualFoldAttributeValue("givenname"),
				Surname:    e.GetEqualFoldAttributeValue("sn"),
				Mail:       e.GetEqualFoldAttributeValue("mail"),
				Office:     e.GetEqualFoldAttributeValue("physicaldeliveryofficename"),
				Department: e.GetEqualFoldAttributeValue("department"),
				Title:      title,
				Phone:      e.GetEqualFoldAttributeValue("telephonenumber"),
				Mobile:     e.GetEqualFoldAttributeValue("mobile"),
				Aliases:    extractProxyAliases(e.GetEqualFoldAttributeValues("proxyAddresses"), e.GetEqualFoldAttributeValue("mail")),
			})
		}
	}
	return out, dbg, nil
}

// extractProxyAliases parses AD proxyAddresses ("SMTP:primary@x", "smtp:alias@x")
// into a lowercased list of SMTP addresses other than the primary mail. Non-SMTP
// schemes (sip:, x500:, etc.) and the primary address itself are dropped.
func extractProxyAliases(proxies []string, primaryMail string) []string {
	primary := strings.ToLower(strings.TrimSpace(primaryMail))
	seen := map[string]bool{}
	var out []string
	for _, p := range proxies {
		p = strings.TrimSpace(p)
		if len(p) < 5 || !strings.EqualFold(p[:5], "smtp:") {
			continue
		}
		addr := strings.ToLower(strings.TrimSpace(p[5:]))
		if addr == "" || addr == primary || seen[addr] {
			continue
		}
		seen[addr] = true
		out = append(out, addr)
	}
	return out
}

// avatarFileSet returns the set of userids (lowercased) that currently have a
// cached avatar image on disk. Used during sync to persist a per-user
// "has avatar" flag.
func (app *App) avatarFileSet() map[string]bool {
	set := map[string]bool{}
	entries, err := os.ReadDir(app.cfg.dataPath("avatarcache"))
	if err != nil {
		return set
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) > 4 && strings.EqualFold(name[len(name)-4:], ".jpg") {
			set[strings.ToLower(name[:len(name)-4])] = true
		}
	}
	return set
}

// deriveMirrorUsers applies the office/name/mail rules locally to a directory
// snapshot, producing the office-filtered desk-placement mirror that the maps
// and desks features consume. An office field may encode several desk places
// separated by "|"; each becomes its own placement.
func deriveMirrorUsers(dir []DirectoryUser) []LdapUser {
	var out []LdapUser
	for _, d := range dir {
		office := d.Office
		switch {
		case office == "" || office == "-":
			continue
		case d.Givenname == "":
			continue
		case d.Surname == "":
			continue
		case d.Mail == "":
			continue
		}

		base := LdapUser{
			Userid:          d.Userid,
			Givenname:       d.Givenname,
			Surname:         d.Surname,
			Telephonenumber: d.Phone,
			Mail:            d.Mail,
			Description:     d.Title,
			Department:      d.Department,
			Mobile:          d.Mobile,
			Aliases:         d.Aliases,
		}

		office = strings.ReplaceAll(office, " ", "")
		if strings.Contains(office, "|") {
			for _, place := range strings.Split(office, "|") {
				if place == "" {
					continue
				}
				u := base
				u.Office = place
				out = append(out, u)
			}
		} else {
			u := base
			u.Office = office
			out = append(out, u)
		}
	}
	return out
}

// dialLDAP opens a connection to an AD source honouring its LDAP/LDAPS type.
func dialLDAP(src LdapSource) (*ldap.Conn, error) {
	switch strings.ToUpper(src.Type) {
	case "LDAPS":
		host := src.Server
		if !strings.Contains(host, ":") {
			host += ":636"
		}
		conn, err := ldap.DialTLS("tcp", host, &tls.Config{ServerName: strings.Split(host, ":")[0]})
		if err != nil {
			return nil, fmt.Errorf("ldaps dial: %w", err)
		}
		return conn, nil
	case "LDAP", "":
		url := src.Server
		if !strings.HasPrefix(url, "ldap://") {
			url = "ldap://" + url
		}
		conn, err := ldap.DialURL(url)
		if err != nil {
			return nil, fmt.Errorf("ldap dial: %w", err)
		}
		return conn, nil
	default:
		return nil, fmt.Errorf("unknown LDAP type %q", src.Type)
	}
}

// ldapValidate runs a structured, read-only connectivity test for a single LDAP
// source and returns the {ok, checks} payload rendered by the admin test modal.
// It dials the server, binds with the service account and confirms the search
// base is reachable — without performing a full sync.
func (app *App) ldapValidate(id int) map[string]interface{} {
	var checks []testCheck
	add := func(name, status, detail string) {
		checks = append(checks, testCheck{Name: name, Status: status, Detail: detail})
	}

	srcs, _ := app.db.ListLdapSources()
	var src *LdapSource
	for i := range srcs {
		if srcs[i].ID == id {
			src = &srcs[i]
			break
		}
	}
	if src == nil {
		return testResult([]testCheck{{Name: "Connection", Status: "fail", Detail: "LDAP connection not found."}})
	}

	if src.Disabled {
		add("Connection status", "warn", "This connection is disabled and is skipped during sync.")
	} else {
		add("Connection status", "ok", "This connection is enabled.")
	}

	conn, err := dialLDAP(*src)
	if err != nil {
		add("Server connection", "fail", "Could not connect to "+src.Server+": "+err.Error())
		return testResult(checks)
	}
	defer conn.Close()
	add("Server connection", "ok", "Connected to "+src.Server+" ("+strings.ToUpper(src.Type)+").")

	if err := conn.Bind(src.LdapUser, src.LdapPass); err != nil {
		add("Bind", "fail", "Authentication failed for "+src.LdapUser+": "+err.Error())
		return testResult(checks)
	}
	add("Bind", "ok", "Authenticated as "+src.LdapUser+".")

	req := ldap.NewSearchRequest(
		src.OU,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		1, 10, false,
		"(objectClass=*)", []string{"dn"}, nil,
	)
	sr, err := conn.Search(req)
	switch {
	case err != nil && ldap.IsErrorWithCode(err, ldap.LDAPResultSizeLimitExceeded):
		add("Search base", "ok", "Search base \""+src.OU+"\" is reachable.")
	case err != nil:
		add("Search base", "fail", "Search base \""+src.OU+"\" could not be searched: "+err.Error())
	case len(sr.Entries) == 0:
		add("Search base", "warn", "Search base \""+src.OU+"\" is reachable but returned no entries.")
	default:
		add("Search base", "ok", "Search base \""+src.OU+"\" is reachable.")
	}

	return testResult(checks)
}

// logChange appends a row to the ldap changelog.
func (app *App) logChange(now time.Time, name, avatar, changeType, oldVal, newVal string) {
	_ = app.db.AddChangelog(ChangelogEntry{
		Year:     now.Year(),
		Month:    int(now.Month()),
		Day:      now.Day(),
		Hour:     now.Hour(),
		Minute:   now.Minute(),
		Name:     name,
		Avatar:   avatar,
		Type:     changeType,
		Oldvalue: oldVal,
		Newvalue: newVal,
	})
}

// StartADSyncScheduler runs RunADSync in the background on a fixed interval.
// Sync is skipped when no AD sources are configured.
func (app *App) StartADSyncScheduler(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			// Skip when there is nothing enabled to sync (no sources, or every
			// source deactivated) so the scheduler stays quiet instead of logging
			// a "no enabled AD sources" error every interval.
			sources, _ := app.db.ListLdapSources()
			anyEnabled := false
			for _, s := range sources {
				if !s.Disabled {
					anyEnabled = true
					break
				}
			}
			if !anyEnabled {
				continue
			}
			if n, err := app.RunADSync(); err != nil {
				log.Printf("scheduled AD sync failed: %v", err)
			} else {
				log.Printf("scheduled AD sync: %d placements mirrored", n)
			}
		}
	}()
}
