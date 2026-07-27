package web

import (
	"companymaps/internal/store"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// handleAdminPostUsers handles map-admin create/delete and role changes.
func (app *Server) handleAdminPostUsers(r *http.Request, sess Session) string {
	if app.permLevel(sess, "users") < 2 {
		return ""
	}
	if del := r.FormValue("deleteUser"); del != "" {
		_ = app.db.DeleteUser(del)
		_ = app.db.AuditLog("Users", sess.Username, "Admin removed ("+del+")")
		return "User removed."
	}
	if chg := r.FormValue("changeRoleUser"); chg != "" {
		roleInt, _ := strconv.Atoi(r.FormValue("changeRole"))
		if existing, found, _ := app.db.GetUser(chg); found {
			existing.Role = roleInt
			_ = app.db.PutUser(existing)
			_ = app.db.AuditLog("Users", sess.Username, "Role changed ("+chg+", role "+r.FormValue("changeRole")+")")
			return "Role updated."
		}
		return ""
	}
	user := r.FormValue("newadminuser")
	role := r.FormValue("newadminrole")
	if user != "" && role != "" {
		roleInt, _ := strconv.Atoi(role)

		// Resolve the entered value against the directory so the admin can
		// type a display name (or samaccountname) and we save the proper
		// DOMAIN\samaccountname plus the full name automatically.
		username := strings.TrimSpace(user)
		fullname := strings.TrimSpace(r.FormValue("newadminname"))
		mail := ""
		if d, ok := app.resolveDirectoryEntry(user); ok {
			domain := app.db.GetSetting("domain")
			if domain != "" {
				username = domain + "\\" + d.Userid
			} else {
				username = d.Userid
			}
			fullname = d.DisplayName()
			mail = d.Mail
		}

		existing, found, _ := app.db.GetUser(username)
		if found {
			existing.Role = roleInt
			if fullname != "" {
				existing.Fullname = fullname
			}
			if mail != "" {
				existing.Mail = mail
			}
			_ = app.db.PutUser(existing)
		} else {
			_ = app.db.PutUser(store.User{Username: username, Role: roleInt, Fullname: fullname, Mail: mail})
		}
		_ = app.db.AuditLog("Users", sess.Username, "New admin created ("+username+", role "+role+")")
		return "User created."
	}
	return ""
}

func (app *Server) handleRestDirectorySearch(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "users") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	domain := app.db.GetSetting("domain")

	const maxResults = 25
	out := make([]map[string]string, 0, maxResults)
	seen := make(map[string]bool) // dedup by lowercased username

	add := func(name, sam, username, mail, office, source string) {
		key := strings.ToLower(strings.TrimSpace(username))
		if key == "" || seen[key] || len(out) >= maxResults {
			return
		}
		seen[key] = true
		out = append(out, map[string]string{
			"name":     name,
			"sam":      sam,
			"username": username,
			"mail":     mail,
			"office":   office,
			"source":   source,
		})
	}
	matches := func(hay string) bool {
		return q == "" || strings.Contains(strings.ToLower(hay), q)
	}
	resolveUsername := func(userid string) string {
		if domain != "" {
			return domain + "\\" + userid
		}
		return userid
	}

	// 1. SAML users first: role 0 means a SAML-provisioned account that has not
	// been granted a role yet and is invisible to every other source.
	if users, err := app.db.ListUsers(); err == nil {
		for _, u := range users {
			if u.Role != 0 {
				continue
			}
			name := strings.TrimSpace(u.Fullname)
			if name == "" {
				name = u.Username
			}
			if !matches(name + " " + u.Username + " " + u.Mail) {
				continue
			}
			// The stored username is authoritative for SAML users so granting a
			// role updates the existing record instead of creating a duplicate.
			add(name, u.Username, u.Username, u.Mail, "", "SAML")
		}
	}

	// 2. LDAP directory.
	if results, err := app.db.SearchDirectory(q, maxResults); err == nil {
		for _, d := range results {
			add(d.DisplayName(), d.Userid, resolveUsername(d.Userid), d.Mail, d.Office, "LDAP")
		}
	}

	// 3. Entra source mirrors (enabled sources only).
	if srcs, err := app.db.ListEntraSources(); err == nil {
		for _, src := range srcs {
			if src.Disabled {
				continue
			}
			mirror, _ := app.db.GetSourceMirror("entra", src.ID)
			for _, u := range mirror {
				name := strings.TrimSpace(u.Givenname + " " + u.Surname)
				if name == "" {
					name = u.Userid
				}
				if !matches(name + " " + u.Userid + " " + u.Mail) {
					continue
				}
				add(name, u.Userid, resolveUsername(u.Userid), u.Mail, u.Office, "Entra")
			}
		}
	}

	writeJSON(w, out)
}

// handleRestAllUsers powers the "All users" subtab: it lists every account in
// the users bucket (including role-0 accounts, e.g. freshly provisioned SAML
// users) with a resolved display name, mail and role. The list is paged and
// searchable server-side so large directories load lazily.
func (app *Server) handleRestAllUsers(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "users") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	// Role names for the role column.
	roleName := map[int]string{}
	if roles, err := app.db.ListRoles(); err == nil {
		for _, ro := range roles {
			roleName[ro.ID] = ro.Rolename
		}
	}
	// Cached directory for name/mail resolution, keyed by lowercased sam.
	ldapNames := map[string]string{}
	ldapMails := map[string]string{}
	if dir, err := app.db.ListDirectory(); err == nil {
		for _, d := range dir {
			ldapNames[strings.ToLower(d.Userid)] = d.DisplayName()
			ldapMails[strings.ToLower(d.Userid)] = d.Mail
		}
	}

	type row struct {
		Avatar    string `json:"avatar"`
		Name      string `json:"name"`
		Username  string `json:"username"`
		Mail      string `json:"mail"`
		Role      int    `json:"role"`
		RoleName  string `json:"rolename"`
		LastLogin string `json:"lastlogin"`
	}

	users, _ := app.db.ListUsers()
	rows := make([]row, 0, len(users))
	for _, u := range users {
		sam := u.Username
		if idx := strings.LastIndex(sam, "\\"); idx >= 0 {
			sam = sam[idx+1:]
		}
		display := strings.TrimSpace(u.Fullname)
		if display == "" {
			if full, ok := ldapNames[strings.ToLower(sam)]; ok {
				display = full
			}
		}
		if display == "" {
			display = u.Username
		}
		mail := strings.TrimSpace(u.Mail)
		if mail == "" {
			if m, ok := ldapMails[strings.ToLower(sam)]; ok {
				mail = strings.TrimSpace(m)
			}
		}
		rn := roleName[u.Role]
		if rn == "" {
			if u.Role == 0 {
				rn = "No role"
			} else {
				rn = strconv.Itoa(u.Role)
			}
		}
		if q != "" && !strings.Contains(strings.ToLower(display+" "+u.Username+" "+mail), q) {
			continue
		}
		rows = append(rows, row{
			Avatar:    sam,
			Name:      display,
			Username:  u.Username,
			Mail:      mail,
			Role:      u.Role,
			RoleName:  rn,
			LastLogin: u.LastLogin,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
	})

	total := len(rows)
	end := offset + limit
	if offset > total {
		offset = total
	}
	if end > total {
		end = total
	}
	page := rows[offset:end]
	writeJSON(w, map[string]interface{}{
		"users":   page,
		"hasMore": end < total,
		"total":   total,
	})
}

// handleRestDirectoryMatch re-resolves the full name (and mail) of every
// existing admin against the cached AD directory. This is useful for accounts
// that were created before the directory cache existed, so their names get
func (app *Server) handleRestDirectoryMatch(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "users") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	dir, _ := app.db.ListDirectory()
	if len(dir) == 0 {
		writeJSON(w, map[string]interface{}{
			"matched": 0, "updated": 0, "directory": 0,
			"message": "The directory cache is empty. Run an LDAP sync first.",
		})
		return
	}
	matched, updated := app.dir.RefreshAdminNames(dir)
	_ = app.db.AuditLog("Users", sess.Username, fmt.Sprintf("Matched names from directory (%d matched, %d updated)", matched, updated))
	writeJSON(w, map[string]interface{}{
		"matched":   matched,
		"updated":   updated,
		"directory": len(dir),
		"message":   fmt.Sprintf("%d user(s) matched, %d name(s) updated.", matched, updated),
	})
}

// handleRestSetting saves a single base variable and returns the stored value
func (app *Server) resolveDirectoryEntry(input string) (store.DirectoryUser, bool) {
	v := strings.TrimSpace(input)
	if v == "" {
		return store.DirectoryUser{}, false
	}
	// Strip any DOMAIN\ prefix and try a direct samaccountname lookup.
	sam := v
	if idx := strings.LastIndex(sam, "\\"); idx >= 0 {
		sam = sam[idx+1:]
	}
	if d, found, _ := app.db.GetDirectoryUser(sam); found {
		return d, true
	}
	// Fall back to an exact (case-insensitive) display-name match.
	dir, err := app.db.ListDirectory()
	if err != nil {
		return store.DirectoryUser{}, false
	}
	lv := strings.ToLower(v)
	var match store.DirectoryUser
	count := 0
	for _, d := range dir {
		if strings.ToLower(d.DisplayName()) == lv {
			match = d
			count++
		}
	}
	if count == 1 {
		return match, true
	}
	return store.DirectoryUser{}, false
}

// handleRestRobinTest runs a full Robin meeting sync and returns the step-by-step
