package main

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLConfig holds connection parameters for a CompanyMaps 8 source database.
type MySQLConfig struct {
	Host     string
	Port     string
	Database string
	User     string
	Password string
}

// ImportResult summarizes what was imported.
type ImportResult struct {
	Maps       int `json:"maps"`
	Desks      int `json:"desks"`
	LdapUsers  int `json:"ldap_users"`
	Bookings   int `json:"bookings"`
	Teams      int `json:"teams"`
	Roles      int `json:"roles"`
	Users      int `json:"users"`
	Changelog  int `json:"changelog"`
	Stats      int `json:"stats"`
	Settings   int `json:"settings"`
	Vips       int `json:"vips"`
	LdapSrc    int `json:"ldap_sources"`
	RobinSpace int `json:"robin_spaces"`
	Whitelist  int `json:"whitelist"`
}

// ImportProgress tracks the state of an in-flight (or finished) MySQL import so
// the setup UI can poll it while the import runs in the background. All access
// goes through its methods, which are safe for concurrent use.
type ImportProgress struct {
	mu      sync.Mutex
	running bool
	done    bool
	stage   string
	result  ImportResult
	summary string
	errMsg  string
}

// start marks the import as running and clears any previous state. It returns
// false if an import is already in flight.
func (p *ImportProgress) start() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return false
	}
	p.running = true
	p.done = false
	p.stage = "Starting"
	p.result = ImportResult{}
	p.summary = ""
	p.errMsg = ""
	return true
}

// update records the current stage and counts.
func (p *ImportProgress) update(stage string, res ImportResult) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stage = stage
	p.result = res
}

// finish records the terminal state (success summary or error message).
func (p *ImportProgress) finish(summary, errMsg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.running = false
	p.done = true
	p.summary = summary
	p.errMsg = errMsg
}

// snapshot returns a JSON-serializable copy of the current progress.
func (p *ImportProgress) snapshot() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return map[string]interface{}{
		"running": p.running,
		"done":    p.done,
		"stage":   p.stage,
		"result":  p.result,
		"summary": p.summary,
		"error":   p.errMsg,
	}
}

// progressBatch controls how often row-level progress is reported while reading
// large tables (changelog, stats, bookings).
const progressBatch = 500

// ImportFromMySQL connects to a CompanyMaps 8 MySQL database and copies every
// relevant table into the BoltDB buckets. It is idempotent: re-running overwrites
// keyed records. The optional report callback is invoked as work progresses so a
// caller can surface live progress to the UI; it may be nil.
func (app *App) ImportFromMySQL(c MySQLConfig, report func(stage string, res ImportResult)) (*ImportResult, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=false",
		c.User, c.Password, c.Host, c.Port, c.Database)
	sqldb, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening mysql: %w", err)
	}
	defer sqldb.Close()
	if err := sqldb.Ping(); err != nil {
		return nil, fmt.Errorf("connecting to mysql: %w", err)
	}

	res := &ImportResult{}
	db := app.db

	// progress reports the current stage and counts to the caller (if any).
	progress := func(stage string) {
		if report != nil {
			report(stage, *res)
		}
	}
	progress("Connecting")

	// config_general -> settings
	if rows, err := sqldb.Query("SELECT `variable`,`value` FROM `config_general`"); err == nil {
		for rows.Next() {
			var k, v string
			if rows.Scan(&k, &v) == nil {
				if db.SetSetting(k, v) == nil {
					res.Settings++
				}
			}
		}
		rows.Close()
	}
	progress("Settings")

	// config_maplist -> maps
	mapNames := []string{}
	if rows, err := sqldb.Query("SELECT `mapname`,`itemscale`,`published`,`country`,`flagsize`,`timezone`,`address`,`mapX`,`mapY` FROM `config_maplist`"); err == nil {
		for rows.Next() {
			var m MapInfo
			if rows.Scan(&m.Mapname, &m.Itemscale, &m.Published, &m.Country, &m.Flagsize, &m.Timezone, &m.Address, &m.MapX, &m.MapY) == nil {
				if db.PutMap(m) == nil {
					res.Maps++
					mapNames = append(mapNames, m.Mapname)
				}
			}
		}
		rows.Close()
	}
	progress("Maps")

	// desks_<map> -> desks
	for _, mn := range mapNames {
		if mn == "overview" {
			continue
		}
		table := "desks_" + mn
		q := fmt.Sprintf("SELECT `ID`,`desktype`,`x`,`y`,`desknumber`,`employee`,`avatar`,`department` FROM `%s`", table)
		rows, err := sqldb.Query(q)
		if err != nil {
			continue // table may not exist (e.g. nomap locations)
		}
		for rows.Next() {
			var d Desk
			if rows.Scan(&d.ID, &d.Desktype, &d.X, &d.Y, &d.Desknumber, &d.Employee, &d.Avatar, &d.Department) == nil {
				d.Map = mn
				if db.PutDesk(d) == nil {
					res.Desks++
					if res.Desks%progressBatch == 0 {
						progress("Desks")
					}
				}
			}
		}
		rows.Close()
	}
	progress("Desks")

	// ldap-mirror -> ldapmirror
	if rows, err := sqldb.Query("SELECT `givenname`,`surname`,`telephonenumber`,`mail`,`physicaldeliveryofficename`,`ipphone`,`description`,`department`,`mobile` FROM `ldap-mirror`"); err == nil {
		var users []LdapUser
		for rows.Next() {
			var u LdapUser
			if rows.Scan(&u.Givenname, &u.Surname, &u.Telephonenumber, &u.Mail, &u.Office, &u.Userid, &u.Description, &u.Department, &u.Mobile) == nil {
				users = append(users, u)
			}
		}
		rows.Close()
		if db.ReplaceLdap(users) == nil {
			res.LdapUsers = len(users)
		}
	}
	progress("Address book")

	// bookings -> bookings
	if rows, err := sqldb.Query("SELECT `date`,`map`,`desk`,`user`,`fullname`,`phone`,`mail` FROM `bookings`"); err == nil {
		for rows.Next() {
			var b Booking
			if rows.Scan(&b.Date, &b.Map, &b.Desk, &b.User, &b.Fullname, &b.Phone, &b.Mail) == nil {
				if db.AddBooking(b) == nil {
					res.Bookings++
					if res.Bookings%progressBatch == 0 {
						progress("Bookings")
					}
				}
			}
		}
		rows.Close()
	}
	progress("Bookings")

	// config_teams -> teams
	if rows, err := sqldb.Query("SELECT `teamname`,`teammembers` FROM `config_teams`"); err == nil {
		for rows.Next() {
			var t Team
			if rows.Scan(&t.Teamname, &t.Members) == nil {
				if db.PutTeam(t) == nil {
					res.Teams++
				}
			}
		}
		rows.Close()
	}

	// config_roles -> roles
	if rows, err := sqldb.Query("SELECT `ID`,`rolename`,`perm_desks`,`perm_dashboard`,`perm_config`,`perm_ldap`,`perm_maps`,`perm_users`,`perm_teams`,`perm_stats`,`perm_auditlog`,`perm_health`,`perm_adminpanel` FROM `config_roles`"); err == nil {
		for rows.Next() {
			var r Role
			p := make([]int, len(permFeatures))
			dst := []interface{}{&r.ID, &r.Rolename}
			for i := range p {
				dst = append(dst, &p[i])
			}
			if rows.Scan(dst...) == nil {
				r.Perms = map[string]int{}
				for i, f := range permFeatures {
					r.Perms[f] = p[i]
				}
				if db.PutRole(r) == nil {
					res.Roles++
				}
			}
		}
		rows.Close()
	}

	// config_mapadmins -> users
	if rows, err := sqldb.Query("SELECT `user`,`role` FROM `config_mapadmins`"); err == nil {
		for rows.Next() {
			var username, role string
			if rows.Scan(&username, &role) == nil {
				rid, _ := strconv.Atoi(strings.TrimSpace(role))
				if db.PutUser(User{Username: username, Role: rid}) == nil {
					res.Users++
				}
			}
		}
		rows.Close()
	}

	// config_vips -> vips
	if rows, err := sqldb.Query("SELECT `Parsed Text in Job Title`,`Type`,`Description` FROM `config_vips`"); err == nil {
		for rows.Next() {
			var v VIP
			if rows.Scan(&v.Title, &v.Type, &v.Description) == nil {
				if db.AddVip(v) == nil {
					res.Vips++
				}
			}
		}
		rows.Close()
	}

	// config_department_list -> departments
	if rows, err := sqldb.Query("SELECT `department-name` FROM `config_department_list`"); err == nil {
		for rows.Next() {
			var d string
			if rows.Scan(&d) == nil {
				_ = db.AddDepartment(d)
			}
		}
		rows.Close()
	}

	// config_ldap -> ldapsources
	if rows, err := sqldb.Query("SELECT `ID`,`description`,`server`,`type`,`OU`,`LdapUser`,`LdapPass`,`LastSync` FROM `config_ldap`"); err == nil {
		for rows.Next() {
			var s LdapSource
			if rows.Scan(&s.ID, &s.Description, &s.Server, &s.Type, &s.OU, &s.LdapUser, &s.LdapPass, &s.LastSync) == nil {
				s.LastSync = strings.TrimSpace(s.LastSync)
				if db.PutLdapSource(s) == nil {
					res.LdapSrc++
				}
			}
		}
		rows.Close()
	}

	// config_robinspaces -> robinspaces
	if rows, err := sqldb.Query("SELECT `spacename`,`spaceid` FROM `config_robinspaces`"); err == nil {
		for rows.Next() {
			var s RobinSpace
			if rows.Scan(&s.Spacename, &s.Spaceid) == nil {
				if db.PutRobinSpace(s) == nil {
					res.RobinSpace++
				}
			}
		}
		rows.Close()
	}

	// ldap_changelog -> changelog
	if rows, err := sqldb.Query("SELECT `year`,`month`,`day`,`hour`,`minute`,`name`,`avatar`,`type`,`oldvalue`,`newvalue` FROM `ldap_changelog` ORDER BY `ID` ASC"); err == nil {
		for rows.Next() {
			var e ChangelogEntry
			if rows.Scan(&e.Year, &e.Month, &e.Day, &e.Hour, &e.Minute, &e.Name, &e.Avatar, &e.Type, &e.Oldvalue, &e.Newvalue) == nil {
				if db.AddChangelog(e) == nil {
					res.Changelog++
					if res.Changelog%progressBatch == 0 {
						progress("Change history")
					}
				}
			}
		}
		rows.Close()
	}
	progress("Change history")

	// stats -> stats
	if rows, err := sqldb.Query("SELECT `date`,`year`,`month`,`day`,`count` FROM `stats`"); err == nil {
		for rows.Next() {
			var s StatEntry
			if rows.Scan(&s.Date, &s.Year, &s.Month, &s.Day, &s.Count) == nil {
				if db.PutStat(s) == nil {
					res.Stats++
					if res.Stats%progressBatch == 0 {
						progress("Statistics")
					}
				}
			}
		}
		rows.Close()
	}
	progress("Statistics")

	// health_whitelist -> whitelist
	if rows, err := sqldb.Query("SELECT `type`,`text` FROM `health_whitelist`"); err == nil {
		for rows.Next() {
			var e WhitelistEntry
			if rows.Scan(&e.Type, &e.Text) == nil {
				if db.AddWhitelist(e) == nil {
					res.Whitelist++
				}
			}
		}
		rows.Close()
	}

	_ = db.AuditLog("setup", "admin", fmt.Sprintf("MySQL import complete: %d maps, %d desks, %d ldap users", res.Maps, res.Desks, res.LdapUsers))
	progress("Done")
	return res, nil
}
