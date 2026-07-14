package directory

import (
	"companymaps/internal/integrations"
	"companymaps/internal/progress"
	"companymaps/internal/store"

	"crypto/tls"
	"fmt"
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
func (s *Syncer) RunADSync() (int, error) {
	return s.RunADSyncProg(nil)
}

// runADSync is the worker behind RunADSync. When prog is non-nil it reports
// per-source progress and a live log so the admin Sync tab can render a progress
// bar during a manual sync.
//
// Each source is fetched and derived independently and stored in its own
// per-source bucket, after which the shared combined caches are rebuilt from all
// enabled sources (combine-on-write). This keeps a single failed/partial source
// from wiping the others and lets a per-source "Sync now" refresh just that one.
func (s *Syncer) RunADSyncProg(prog *progress.Progress) (int, error) {
	sources, err := s.DB.ListLdapSources()
	if err != nil {
		return 0, fmt.Errorf("loading AD sources: %w", err)
	}
	if len(sources) == 0 {
		return 0, fmt.Errorf("no AD sources configured")
	}
	// Skip deactivated sources: their placements are dropped from the combined
	// mirror because rebuildLdapMirror only unions enabled sources below.
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
		prog.SetTotal(len(sources))
		prog.Logf("Starting sync of %d source(s)…", len(sources))
	}

	now := time.Now().In(s.DB.Location())
	debug := ADSyncDebug{When: now.Format("2006-01-02 15:04:05")}

	for _, src := range sources {
		if prog != nil {
			prog.SetStage("Syncing " + src.Description)
			prog.Logf("→ %s: connecting to %s (%s)…", src.Description, src.Server, src.Type)
		}
		dirUsers, dbg, err := s.FetchSourceDirectory(src)
		users := DeriveMirrorUsers(dirUsers)
		dbg.Mirrored = len(users)
		debug.Sources = append(debug.Sources, dbg)
		if err != nil {
			s.logger().Error("AD sync: source failed", "source", src.Description, "err", err)
			s.SetSyncDebug(debug)
			if prog != nil {
				prog.Logf("   ✗ %s: %s", src.Description, err.Error())
				prog.Step("")
			}
			return 0, fmt.Errorf("source %q: %w", src.Description, err)
		}
		if prog != nil {
			prog.Logf("   connected=%v bound=%v · directory: %d · desk placements: %d",
				dbg.Connected, dbg.Bound, dbg.EntriesFound, dbg.Mirrored)
		}

		// Store this source's data in its own bucket (combine-on-write).
		if err := s.DB.PutSourceDir(src.ID, dirUsers); err != nil {
			s.logger().Error("AD sync: write source directory", "source", src.Description, "err", err)
		}
		if err := s.DB.PutSourceMirror("ldap", src.ID, users); err != nil {
			s.logger().Error("AD sync: write source mirror", "source", src.Description, "err", err)
		}

		src.LastSync = now.Format("2006-01-02 15:04:05")
		if err := s.DB.PutLdapSource(src); err != nil {
			s.logger().Error("AD sync: update LastSync", "source", src.Description, "err", err)
		}
		_ = s.DB.AuditLog("LDAP", "System", src.Description+" has been synced.")
		if prog != nil {
			prog.Step("")
		}
	}

	// Recombine all per-source mirrors into the shared caches. The very first
	// rebuild after a fresh install/upgrade suppresses changelog announcements so
	// seeding does not flood the changelog with "new employee" entries.
	announce := s.DB.GetMeta("ldapSeeded") == "1"
	count, err := s.RebuildLdapMirror(announce)
	if err != nil {
		return count, err
	}
	_ = s.DB.SetMeta("ldapSeeded", "1")

	debug.Total = count
	s.SetSyncDebug(debug)
	if prog != nil {
		prog.Logf("Done. %d desk placement(s) from %d source(s).", count, len(sources))
	}
	return count, nil
}

// rebuildLdapMirror recombines every enabled LDAP source's per-source mirror
// (and directory snapshot) into the shared ldapmirror + directory caches
// (combine-on-write). When announce is true it diffs the freshly combined mirror
// against the current one and records employee/title/name changes in the
// changelog; the first rebuild after a fresh install/upgrade passes
// announce=false so seeding existing data produces no announcements.
func (s *Syncer) RebuildLdapMirror(announce bool) (int, error) {
	sources, err := s.DB.ListLdapSources()
	if err != nil {
		return 0, fmt.Errorf("loading AD sources: %w", err)
	}
	now := time.Now().In(s.DB.Location())

	// Snapshot the existing mirror (keyed by userid) for change detection.
	var oldByID map[string]store.LdapUser
	if announce {
		oldUsers, _ := s.DB.ListLdap()
		oldByID = make(map[string]store.LdapUser, len(oldUsers))
		for _, u := range oldUsers {
			if _, ok := oldByID[u.Userid]; !ok {
				oldByID[u.Userid] = u
			}
		}
	}

	var combined []store.LdapUser
	var allDir []store.DirectoryUser
	dirSeen := make(map[string]bool)
	seen := make(map[string]bool)

	for _, src := range sources {
		if src.Disabled {
			continue
		}
		users, _ := s.DB.GetSourceMirror("ldap", src.ID)
		dirUsers, _ := s.DB.GetSourceDir(src.ID)

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
			if !announce {
				continue
			}
			// Change detection is per unique user (first placement only).
			if seen[u.Userid] {
				continue
			}
			seen[u.Userid] = true

			fullname := strings.TrimSpace(u.Givenname + " " + u.Surname)
			old, known := oldByID[u.Userid]
			if !known {
				s.logChange(now, fullname, u.Userid, "Employee", "none", u.Description)
				continue
			}
			if old.Description != "" && old.Description != u.Description {
				s.logChange(now, fullname, u.Userid, "Title", old.Description, u.Description)
			}
			if old.Description != "" && old.Surname != u.Surname {
				s.logChange(now, fullname, u.Userid, "Name", old.Surname, u.Surname)
			}
		}
	}

	// Flag which mirrored users have a cached avatar on disk so the client can
	// point everyone without one at a single shared placeholder instead of
	// requesting a unique (missing) image per person.
	avatars := s.AvatarFileSet()
	for i := range combined {
		combined[i].HasAvatar = avatars[strings.ToLower(combined[i].Userid)]
	}

	if err := s.DB.ReplaceDirectory(allDir); err != nil {
		s.logger().Error("AD sync: write directory", "err", err)
	}
	if err := s.DB.ReplaceLdap(combined); err != nil {
		return len(combined), fmt.Errorf("writing mirror: %w", err)
	}
	// Refresh stored full names for existing admins from the fresh directory.
	s.RefreshAdminNames(allDir)
	return len(combined), nil
}

// refreshAdminNames updates the stored Fullname of each admin user from the
// freshly fetched directory, so the Users tab shows live names even if the
// directory is later cleared. Usernames are matched on samaccountname after
// stripping any DOMAIN\ prefix.
func (s *Syncer) RefreshAdminNames(dir []store.DirectoryUser) (matched, updated int) {
	if len(dir) == 0 {
		return 0, 0
	}
	byID := make(map[string]store.DirectoryUser, len(dir))
	for _, d := range dir {
		byID[strings.ToLower(d.Userid)] = d
	}
	users, err := s.DB.ListUsers()
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
			_ = s.DB.PutUser(u)
			updated++
		}
	}
	return matched, updated
}

// The sync diagnostics accessors (SetSyncDebug / SyncDebug) live in syncer.go.

// fetchSourceDirectory queries one AD source A-Z and returns EVERY enabled user
// account it finds (regardless of the office attribute) as store.DirectoryUser records.
// This full snapshot powers admin autocomplete and name resolution, and is the
// local data the office-filtered mirror is derived from. Disabled accounts are
// excluded via the userAccountControl bit filter.
func (s *Syncer) FetchSourceDirectory(src store.LdapSource) ([]store.DirectoryUser, SourceDebug, error) {
	dbg := SourceDebug{
		Description: src.Description,
		Server:      src.Server,
		Type:        src.Type,
		OU:          src.OU,
		BindUser:    src.LdapUser,
		SkipReasons: map[string]int{},
	}

	// The built-in demo source never talks to a real directory: it regenerates
	// the bundled demo employees, so its sync can never fail. Avatars are ensured
	// for the current identifier mode so a mode switch keeps the photos.
	if src.Demo {
		dir := s.DemoDirectory()
		s.EnsureDemoAvatars(dir)
		dbg.Connected = true
		dbg.Bound = true
		dbg.EntriesFound = len(dir)
		return dir, dbg, nil
	}

	conn, err := DialLDAP(src)
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
	var out []store.DirectoryUser
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

			sam := e.GetEqualFoldAttributeValue("samaccountname")
			if sam == "" {
				dbg.Skipped++
				dbg.SkipReasons["no samaccountname"]++
				continue
			}
			if key := strings.ToLower(sam); seen[key] {
				continue // same account can match several letters via aliases
			} else {
				seen[key] = true
			}

			title := e.GetEqualFoldAttributeValue("title")
			if title == "" {
				title = "-"
			}
			mail := store.NormalizeMail(e.GetEqualFoldAttributeValue("mail"))
			out = append(out, store.DirectoryUser{
				Userid:         UserIdentifier(s.DB, sam, mail),
				Samaccountname: sam,
				Givenname:      e.GetEqualFoldAttributeValue("givenname"),
				Surname:        e.GetEqualFoldAttributeValue("sn"),
				Mail:           mail,
				Office:         e.GetEqualFoldAttributeValue("physicaldeliveryofficename"),
				Department:     e.GetEqualFoldAttributeValue("department"),
				Title:          title,
				Phone:          e.GetEqualFoldAttributeValue("telephonenumber"),
				Mobile:         e.GetEqualFoldAttributeValue("mobile"),
				Aliases:        ExtractProxyAliases(e.GetEqualFoldAttributeValues("proxyAddresses"), mail),
			})
		}
	}
	return out, dbg, nil
}

// extractProxyAliases parses AD proxyAddresses ("SMTP:primary@x", "smtp:alias@x")
// into a lowercased list of SMTP addresses other than the primary mail. Non-SMTP
// schemes (sip:, x500:, etc.) and the primary address itself are dropped.
func ExtractProxyAliases(proxies []string, primaryMail string) []string {
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
func (s *Syncer) AvatarFileSet() map[string]bool {
	set := map[string]bool{}
	entries, err := os.ReadDir(s.AvatarDir)
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
func DeriveMirrorUsers(dir []store.DirectoryUser) []store.LdapUser {
	var out []store.LdapUser
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

		base := store.LdapUser{
			Userid:          d.Userid,
			Samaccountname:  d.Samaccountname,
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
func DialLDAP(src store.LdapSource) (*ldap.Conn, error) {
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
func (s *Syncer) LdapValidate(id int) map[string]interface{} {
	var checks []integrations.Check
	add := func(name, status, detail string) {
		checks = append(checks, integrations.Check{Name: name, Status: status, Detail: detail})
	}

	srcs, _ := s.DB.ListLdapSources()
	var src *store.LdapSource
	for i := range srcs {
		if srcs[i].ID == id {
			src = &srcs[i]
			break
		}
	}
	if src == nil {
		return integrations.Result([]integrations.Check{{Name: "Connection", Status: "fail", Detail: "LDAP connection not found."}})
	}

	if src.Disabled {
		add("Connection status", "warn", "This connection is disabled and is skipped during sync.")
	} else {
		add("Connection status", "ok", "This connection is enabled.")
	}

	conn, err := DialLDAP(*src)
	if err != nil {
		add("Server connection", "fail", "Could not connect to "+src.Server+": "+err.Error())
		return integrations.Result(checks)
	}
	defer conn.Close()
	add("Server connection", "ok", "Connected to "+src.Server+" ("+strings.ToUpper(src.Type)+").")

	if err := conn.Bind(src.LdapUser, src.LdapPass); err != nil {
		add("Bind", "fail", "Authentication failed for "+src.LdapUser+": "+err.Error())
		return integrations.Result(checks)
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

	return integrations.Result(checks)
}

// logChange appends a row to the ldap changelog.
func (s *Syncer) logChange(now time.Time, name, avatar, changeType, oldVal, newVal string) {
	_ = s.DB.AddChangelog(store.ChangelogEntry{
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

// anyLdapSourceEnabled reports whether at least one non-disabled LDAP source is
// configured (used to decide whether the scheduled AD sync should run).
func (s *Syncer) AnyLdapSourceEnabled() bool {
	sources, _ := s.DB.ListLdapSources()
	for _, s := range sources {
		if !s.Disabled {
			return true
		}
	}
	return false
}
