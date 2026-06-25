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

// RunADSync mirrors all configured AD sources into the ldapmirror bucket,
// recording employee/title/name changes in the changelog. It returns the number
// of mirrored desk placements (one per office value; an office field may list
// several places separated by "|").
//
// This is the Go port of tools/ldap_connector.php (CLI path). MS Teams
// notifications from the original are intentionally dropped.
func (app *App) RunADSync() (int, error) {
	sources, err := app.db.ListLdapSources()
	if err != nil {
		return 0, fmt.Errorf("loading AD sources: %w", err)
	}
	if len(sources) == 0 {
		return 0, fmt.Errorf("no AD sources configured")
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

	for _, src := range sources {
		users, err := app.syncOneSource(src)
		if err != nil {
			log.Printf("AD sync: source %q: %v", src.Description, err)
			return len(combined), fmt.Errorf("source %q: %w", src.Description, err)
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
	}

	if err := app.db.ReplaceLdap(combined); err != nil {
		return len(combined), fmt.Errorf("writing mirror: %w", err)
	}
	return len(combined), nil
}

// syncOneSource queries one AD source A-Z and returns the mirrored placements.
func (app *App) syncOneSource(src LdapSource) ([]LdapUser, error) {
	conn, err := dialLDAP(src)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := conn.Bind(src.LdapUser, src.LdapPass); err != nil {
		return nil, fmt.Errorf("bind: %w", err)
	}

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
			return nil, fmt.Errorf("search %c: %w", letter, err)
		}
		for _, e := range sr.Entries {
			givenname := e.GetAttributeValue("givenname")
			surname := e.GetAttributeValue("sn")
			mail := e.GetAttributeValue("mail")
			office := e.GetAttributeValue("physicaldeliveryofficename")
			userid := e.GetAttributeValue("samaccountname")
			title := e.GetAttributeValue("title")
			if title == "" {
				title = "-"
			}

			if office == "" || office == "-" || givenname == "" || surname == "" || mail == "" {
				continue
			}

			base := LdapUser{
				Userid:          userid,
				Givenname:       givenname,
				Surname:         surname,
				Telephonenumber: e.GetAttributeValue("telephonenumber"),
				Mail:            mail,
				Description:     title,
				Department:      e.GetAttributeValue("department"),
				Mobile:          e.GetAttributeValue("mobile"),
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
	return out, nil
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
