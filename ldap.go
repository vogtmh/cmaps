package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// ldapSyncAttrs are the AD attributes the mirror needs.
var ldapSyncAttrs = []string{
	"givenname", "sn", "telephonenumber", "mail",
	"physicaldeliveryofficename", "samaccountname", "title", "department", "mobile",
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
	seen := make(map[string]bool)
	debug := ADSyncDebug{When: now.Format("2006-01-02 15:04:05")}

	for _, src := range sources {
		if prog != nil {
			prog.setStage("Syncing " + src.Description)
			prog.logf("→ %s: connecting to %s (%s)…", src.Description, src.Server, src.Type)
		}
		users, dbg, err := app.syncOneSource(src)
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
			prog.logf("   connected=%v bound=%v · entries found: %d · mirrored: %d · skipped: %d",
				dbg.Connected, dbg.Bound, dbg.EntriesFound, dbg.Mirrored, dbg.Skipped)
			if len(dbg.SkipReasons) > 0 {
				for reason, n := range dbg.SkipReasons {
					prog.logf("      skipped %d × %s", n, reason)
				}
			}
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

	if err := app.db.ReplaceLdap(combined); err != nil {
		return len(combined), fmt.Errorf("writing mirror: %w", err)
	}
	debug.Total = len(combined)
	app.setSyncDebug(debug)
	if prog != nil {
		prog.logf("Done. Mirrored %d placement(s) from %d source(s).", len(combined), len(sources))
	}
	return len(combined), nil
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

// syncOneSource queries one AD source A-Z and returns the mirrored placements.
// The returned SourceDebug captures diagnostics (entries found, skip reasons,
// the attribute names AD actually returned) to help explain empty results.
func (app *App) syncOneSource(src LdapSource) ([]LdapUser, SourceDebug, error) {
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

	var out []LdapUser
	for letter := 'A'; letter <= 'Z'; letter++ {
		filter := fmt.Sprintf("(&(physicaldeliveryofficename=*)(givenname=%c*))", letter)
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

			givenname := e.GetEqualFoldAttributeValue("givenname")
			surname := e.GetEqualFoldAttributeValue("sn")
			mail := e.GetEqualFoldAttributeValue("mail")
			office := e.GetEqualFoldAttributeValue("physicaldeliveryofficename")
			userid := e.GetEqualFoldAttributeValue("samaccountname")
			title := e.GetEqualFoldAttributeValue("title")
			if title == "" {
				title = "-"
			}

			switch {
			case office == "" || office == "-":
				dbg.Skipped++
				dbg.SkipReasons["no office"]++
				continue
			case givenname == "":
				dbg.Skipped++
				dbg.SkipReasons["no givenname"]++
				continue
			case surname == "":
				dbg.Skipped++
				dbg.SkipReasons["no surname"]++
				continue
			case mail == "":
				dbg.Skipped++
				dbg.SkipReasons["no mail"]++
				continue
			}

			base := LdapUser{
				Userid:          userid,
				Givenname:       givenname,
				Surname:         surname,
				Telephonenumber: e.GetEqualFoldAttributeValue("telephonenumber"),
				Mail:            mail,
				Description:     title,
				Department:      e.GetEqualFoldAttributeValue("department"),
				Mobile:          e.GetEqualFoldAttributeValue("mobile"),
			}

			// An office field may encode several desk places separated by "|".
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
	}
	dbg.Mirrored = len(out)
	return out, dbg, nil
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
			if sources, _ := app.db.ListLdapSources(); len(sources) == 0 {
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
