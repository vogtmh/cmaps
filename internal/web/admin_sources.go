package web

import (
	"companymaps/internal/directory"
	"companymaps/internal/store"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// handleAdminPostSync handles the Sync tab: demo data, source priority, LDAP/EntraID/Robin/Geoapify source management.
func (app *Server) handleAdminPostSync(r *http.Request, sess Session) string {
	if app.permLevel(sess, "ldap") < 2 {
		return ""
	}
	// Create/recreate the built-in demo data source (Sync > Demo subtab,
	// offered when no real directory source is configured).
	if r.FormValue("createDemoData") != "" {
		if err := app.createDemoData(); err != nil {
			return "Error creating demo data: " + err.Error()
		}
		_ = app.db.AuditLog("LDAP", sess.Username, "Demo data created from the admin panel")
		return "Demo data created. The demo maps are now populated."
	}
	if r.FormValue("removeDemoData") != "" {
		if err := app.removeDemoData(); err != nil {
			return "Error removing demo data: " + err.Error()
		}
		_ = app.db.AuditLog("LDAP", sess.Username, "Demo data removed from the admin panel")
		return "Demo data removed."
	}
	// Unified directory-source priority list (Sync > General): reorder and
	// per-source toggles operate on the stored order, never touching the
	// sources' own sync config, and require no resync (assignment is decided
	// at render time).
	if ref := r.FormValue("moveSourceUp"); ref != "" {
		if app.moveSource(ref, -1) {
			_ = app.db.AuditLog("LDAP", sess.Username, "Source priority raised ("+ref+")")
		}
		return "Source priority updated."
	}
	if ref := r.FormValue("moveSourceDown"); ref != "" {
		if app.moveSource(ref, 1) {
			_ = app.db.AuditLog("LDAP", sess.Username, "Source priority lowered ("+ref+")")
		}
		return "Source priority updated."
	}
	if ref := r.FormValue("toggleSourceAssign"); ref != "" {
		on := r.FormValue("sourceAssign") == "1"
		app.setSourceFlags(ref, &on, nil)
		verb := "excluded from"
		if on {
			verb = "included in"
		}
		_ = app.db.AuditLog("LDAP", sess.Username, "Source "+verb+" desk assignment ("+ref+")")
		return "Source assignment updated."
	}
	if ref := r.FormValue("toggleSourceKeepDup"); ref != "" {
		keep := r.FormValue("sourceKeepDup") == "1"
		app.setSourceFlags(ref, nil, &keep)
		verb := "off"
		if keep {
			verb = "on"
		}
		_ = app.db.AuditLog("LDAP", sess.Username, "Source keep-duplicates "+verb+" ("+ref+")")
		return "Source duplicate handling updated."
	}
	// Robin / meeting-room management lives on the LDAP tab.
	if name := r.FormValue("deleteRobinSpace"); name != "" {
		_ = app.db.DeleteRobinSpace(name)
		_ = app.db.AuditLog("LDAP", sess.Username, "Robin space removed ("+name+")")
		return "Robin space removed."
	}
	if sn := r.FormValue("setRobinMapSpace"); sn != "" {
		spaces, _ := app.db.ListRobinSpaces()
		for _, s := range spaces {
			if s.Spacename == sn {
				s.Mapname = strings.ToLower(strings.TrimSpace(r.FormValue("robinMapname")))
				_ = app.db.PutRobinSpace(s)
				_ = app.db.AuditLog("LDAP", sess.Username, "Robin space map updated ("+sn+")")
				break
			}
		}
		return "Robin map updated."
	}
	if id := r.FormValue("deleteEntraID"); id != "" {
		if n, err := strconv.Atoi(id); err == nil {
			_ = app.db.DeleteEntraSource(n)
			_ = app.db.DeleteSourceMirror("entra", n)
			// Only rebuild the combined mirror once the per-source buckets
			// have been seeded; before that the combined cache holds preserved
			// pre-upgrade data that a rebuild from empty sources would wipe.
			if app.db.GetMeta("entraSeeded") == "1" {
				_, _ = app.dir.RebuildEntraMirror()
			}
			_ = app.db.AuditLog("LDAP", sess.Username, "EntraID connection removed (id "+id+")")
			return "EntraID connection removed."
		}
	}
	if id := r.FormValue("toggleEntraID"); id != "" {
		if n, err := strconv.Atoi(id); err == nil {
			srcs, _ := app.db.ListEntraSources()
			for _, s := range srcs {
				if s.ID == n {
					s.Disabled = r.FormValue("entraEnabled") != "1"
					_ = app.db.PutEntraSource(s)
					verb := "enabled"
					if s.Disabled {
						verb = "disabled"
					}
					_ = app.db.AuditLog("LDAP", sess.Username, "EntraID connection "+verb+" ("+s.Description+")")
					return "EntraID connection " + verb + "."
				}
			}
		}
		return ""
	}
	if id := r.FormValue("editEntraID"); id != "" {
		n, err := strconv.Atoi(id)
		if err != nil {
			return "Error: invalid EntraID id."
		}
		srcs, _ := app.db.ListEntraSources()
		for _, s := range srcs {
			if s.ID == n {
				if v := strings.TrimSpace(r.FormValue("newEntraDescription")); v != "" {
					s.Description = v
				}
				if v := strings.TrimSpace(r.FormValue("newEntraTenant")); v != "" {
					s.TenantID = v
				}
				if v := strings.TrimSpace(r.FormValue("newEntraClient")); v != "" {
					s.ClientID = v
				}
				if v := strings.TrimSpace(r.FormValue("newEntraAuthMethod")); v == "secret" || v == "certificate" {
					s.AuthMethod = v
				}
				// Secrets/cert material are only overwritten when supplied, so
				// re-saving without re-entering them keeps the stored values.
				if v := strings.TrimSpace(r.FormValue("newEntraSecret")); v != "" {
					s.ClientSecret = v
				}
				if v := strings.TrimSpace(r.FormValue("newEntraCert")); v != "" {
					s.CertPEM = v
				}
				if v := strings.TrimSpace(r.FormValue("newEntraKey")); v != "" {
					s.KeyPEM = v
				}
				_ = app.db.PutEntraSource(s)
				_ = app.db.AuditLog("LDAP", sess.Username, "EntraID connection updated ("+s.Description+")")
				return "EntraID connection updated."
			}
		}
		return "Error: EntraID connection not found."
	}
	if r.FormValue("newEntraTenant") != "" || r.FormValue("newEntraClient") != "" {
		desc := strings.TrimSpace(r.FormValue("newEntraDescription"))
		tenant := strings.TrimSpace(r.FormValue("newEntraTenant"))
		client := strings.TrimSpace(r.FormValue("newEntraClient"))
		method := strings.TrimSpace(r.FormValue("newEntraAuthMethod"))
		if method != "secret" && method != "certificate" {
			method = "secret"
		}
		if desc == "" {
			desc = "EntraID"
		}
		if tenant == "" || client == "" {
			return "Error: tenant id and client id are required."
		}
		_ = app.db.PutEntraSource(store.EntraSource{
			ID:           app.nextEntraID(),
			Description:  desc,
			TenantID:     tenant,
			ClientID:     client,
			AuthMethod:   method,
			ClientSecret: strings.TrimSpace(r.FormValue("newEntraSecret")),
			CertPEM:      strings.TrimSpace(r.FormValue("newEntraCert")),
			KeyPEM:       strings.TrimSpace(r.FormValue("newEntraKey")),
			LastSync:     "never",
		})
		_ = app.db.AuditLog("LDAP", sess.Username, "New EntraID connection created ("+desc+")")
		return "EntraID connection created."
	}
	if r.FormValue("saveRobin") != "" {
		if tok := strings.TrimSpace(r.FormValue("robintoken")); tok != "" {
			_ = app.db.SetRobinSetting("robintoken", tok)
		}
		_ = app.db.SetRobinSetting("robinOrganisation", strings.TrimSpace(r.FormValue("robinOrganisation")))
		_ = app.db.AuditLog("LDAP", sess.Username, "Robin credentials updated")
		return "Robin settings saved."
	}
	if r.FormValue("saveGeoapify") != "" {
		if key := strings.TrimSpace(r.FormValue("geoapifyApiKey")); key != "" {
			_ = app.db.SetGeoSetting("geoapifyApiKey", key)
			_ = app.db.AuditLog("LDAP", sess.Username, "Geoapify API key updated")
			return "Geocoding API key saved."
		}
		return "Geocoding API key unchanged."
	}
	if r.FormValue("saveRobinOptions") != "" {
		// Robin desk-occupancy is now either synced ("sync") or not ("off").
		// Whether it is shown on the map and at what priority is decided by
		// the unified source list (Sync > General), not here.
		mode := "off"
		if r.FormValue("robinDeskSync") == "1" {
			mode = "sync"
		}
		_ = app.db.SetRobinSetting("robinDeskMode", mode)
		_ = app.db.SetRobinSetting("robinStripPrefixEnabled", checkboxValue(r.FormValue("robinStripPrefixEnabled")))
		_ = app.db.SetRobinSetting("robinStripPrefixList", r.FormValue("robinStripPrefixList"))
		_ = app.db.SetRobinSetting("robinStripSuffixEnabled", checkboxValue(r.FormValue("robinStripSuffixEnabled")))
		_ = app.db.SetRobinSetting("robinStripSuffixList", r.FormValue("robinStripSuffixList"))
		_ = app.db.AuditLog("LDAP", sess.Username, "Robin options updated")
		return "Robin options saved."
	}
	if r.FormValue("discoverRobin") != "" {
		summary, err := app.robin.ReconcileLocations()
		if err != nil {
			return "Discovery failed: " + err.Error()
		}
		_ = app.db.AuditLog("LDAP", sess.Username, "Robin locations discovered")
		return summary
	}
	if r.FormValue("runRobinSync") != "" {
		res := app.robin.RunSyncStructuredNow()
		_ = app.db.AuditLog("LDAP", sess.Username, "Robin meeting sync run")
		if res.Note != "" {
			return res.Note
		}
		return fmt.Sprintf("Robin sync complete: %d of %d room(s) matched a meeting desk.", res.MatchedRooms, res.TotalRooms)
	}
	if sn := strings.TrimSpace(r.FormValue("robinSpacename")); sn != "" {
		id, err := strconv.Atoi(strings.TrimSpace(r.FormValue("robinSpaceid")))
		if err != nil {
			return "Error: Robin location id must be a number."
		}
		_ = app.db.PutRobinSpace(store.RobinSpace{
			Spacename: strings.ToLower(sn),
			Spaceid:   id,
			Mapname:   strings.ToLower(strings.TrimSpace(r.FormValue("robinMapname"))),
		})
		_ = app.db.AuditLog("LDAP", sess.Username, "Robin space created ("+sn+")")
		return "Robin space added."
	}
	if id := r.FormValue("deleteLdapID"); id != "" {
		if n, err := strconv.Atoi(id); err == nil {
			_ = app.db.DeleteLdapSource(n)
			_ = app.db.DeleteSourceMirror("ldap", n)
			_ = app.db.DeleteSourceDir(n)
			// Only rebuild the combined mirror once the per-source buckets
			// have been seeded; before that the combined cache holds preserved
			// pre-upgrade data that a rebuild from empty sources would wipe.
			if app.db.GetMeta("ldapSeeded") == "1" {
				_, _ = app.dir.RebuildLdapMirror(true)
			}
			_ = app.db.AuditLog("LDAP", sess.Username, "LDAP sync removed (id "+id+")")
			return "LDAP source removed."
		}
	}
	if id := r.FormValue("toggleLdapID"); id != "" {
		if n, err := strconv.Atoi(id); err == nil {
			srcs, _ := app.db.ListLdapSources()
			for _, s := range srcs {
				if s.ID == n {
					s.Disabled = r.FormValue("ldapEnabled") != "1"
					_ = app.db.PutLdapSource(s)
					verb := "enabled"
					if s.Disabled {
						verb = "disabled"
					}
					_ = app.db.AuditLog("LDAP", sess.Username, "LDAP source "+verb+" ("+s.Description+")")
					return "LDAP source " + verb + "."
				}
			}
		}
		return ""
	}
	if id := r.FormValue("editLdapID"); id != "" {
		n, err := strconv.Atoi(id)
		if err != nil {
			return "Error: invalid LDAP id."
		}
		srcs, _ := app.db.ListLdapSources()
		for _, s := range srcs {
			if s.ID == n {
				if v := strings.TrimSpace(r.FormValue("newLdapDescription")); v != "" {
					s.Description = v
				}
				if v := strings.TrimSpace(r.FormValue("newLdapServer")); v != "" {
					s.Server = v
				}
				if v := strings.TrimSpace(r.FormValue("newLdapType")); v != "" {
					s.Type = v
				}
				if v := strings.TrimSpace(r.FormValue("newLdapOU")); v != "" {
					s.OU = v
				}
				if v := strings.TrimSpace(r.FormValue("newLdapUser")); v != "" {
					s.LdapUser = v
				}
				// Only overwrite the bind password when a new one is supplied,
				// so re-saving without re-entering it keeps the stored secret.
				if v := r.FormValue("newLdapPass"); v != "" {
					s.LdapPass = v
				}
				_ = app.db.PutLdapSource(s)
				_ = app.db.AuditLog("LDAP", sess.Username, "LDAP source updated ("+s.Description+")")
				return "LDAP source updated."
			}
		}
		return "Error: LDAP source not found."
	}
	desc := r.FormValue("newLdapDescription")
	server := r.FormValue("newLdapServer")
	typ := r.FormValue("newLdapType")
	ou := r.FormValue("newLdapOU")
	user := r.FormValue("newLdapUser")
	pass := r.FormValue("newLdapPass")
	if desc != "" && server != "" && typ != "" && ou != "" && user != "" && pass != "" {
		_ = app.db.PutLdapSource(store.LdapSource{
			ID: app.nextLdapID(), Description: desc, Server: server, Type: typ,
			OU: ou, LdapUser: user, LdapPass: pass, LastSync: "never",
		})
		_ = app.db.AuditLog("LDAP", sess.Username, "New LDAP sync created ("+desc+")")
		return "LDAP source created."
	}
	return ""
}

func (app *Server) nextLdapID() int {
	srcs, _ := app.db.ListLdapSources()
	max := 0
	for _, s := range srcs {
		if s.Demo {
			continue // the demo source uses a reserved high ID; ignore it here
		}
		if s.ID > max {
			max = s.ID
		}
	}
	return max + 1
}

// hasRealSource reports whether at least one genuine directory source is
// configured (any non-demo LDAP source, any EntraID source, or Robin). When
// false, the admin panel offers the "create demo data" action.
func (app *Server) hasRealSource() bool {
	ldaps, _ := app.db.ListLdapSources()
	for _, s := range ldaps {
		if !s.Demo {
			return true
		}
	}
	if entras, _ := app.db.ListEntraSources(); len(entras) > 0 {
		return true
	}
	return app.robin.Configured()
}

func (app *Server) nextEntraID() int {
	srcs, _ := app.db.ListEntraSources()
	max := 0
	for _, s := range srcs {
		if s.ID > max {
			max = s.ID
		}
	}
	return max + 1
}

func (app *Server) handleRestLdap(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	idStr := r.URL.Query().Get("ldapid")
	if idStr != "" {
		if id, err := strconv.Atoi(idStr); err == nil {
			srcs, _ := app.db.ListLdapSources()
			for _, s := range srcs {
				if s.ID == id {
					// If the per-source buckets have not been seeded yet (fresh
					// upgrade), fall back to a full sync so we never publish a
					// mirror built from just one source (which would drop the
					// others until their next sync).
					if app.db.GetMeta("ldapSeeded") != "1" {
						count, err := app.dir.RunADSync()
						if err != nil {
							http.Error(w, err.Error(), http.StatusInternalServerError)
							return
						}
						_ = app.db.AuditLog("LDAP", sess.Username, "Manual sync of source "+idStr)
						writeJSON(w, map[string]interface{}{"status": "ok", "count": count, "lastSync": nowTimestamp()})
						return
					}
					dir, dbg, err := app.dir.FetchSourceDirectory(s)
					users := directory.DeriveMirrorUsers(dir)
					dbg.Mirrored = len(users)
					app.dir.SetSyncDebug(directory.ADSyncDebug{
						When:    nowTimestamp(),
						Total:   len(users),
						Sources: []directory.SourceDebug{dbg},
					})
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					_ = app.db.PutSourceDir(s.ID, dir)
					_ = app.db.PutSourceMirror("ldap", s.ID, users)
					s.LastSync = nowTimestamp()
					_ = app.db.PutLdapSource(s)
					count, err := app.dir.RebuildLdapMirror(true)
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					_ = app.db.AuditLog("LDAP", sess.Username, "Manual sync of source "+idStr)
					writeJSON(w, map[string]interface{}{"status": "ok", "count": count, "lastSync": s.LastSync})
					return
				}
			}
			http.Error(w, "ldap source not found", http.StatusNotFound)
			return
		}
	}

	count, err := app.dir.RunADSync()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "count": count})
}

// testCheck is one row in a structured connection-test summary, matching the
// SAML/Robin test UI. Alias of the shared integrations.Check (transitional
func (app *Server) handleRestLdapTest(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	id, err := strconv.Atoi(r.URL.Query().Get("ldapid"))
	if err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "checks": []testCheck{{Name: "Connection", Status: "fail", Detail: "invalid LDAP id"}}})
		return
	}
	writeJSON(w, app.dir.LdapValidate(id))
}

// handleRestLdapDebug returns diagnostics from the most recent AD sync so the
// admin panel can show why a sync mirrored few/no users.
func (app *Server) handleRestLdapDebug(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, app.dir.SyncDebug())
}

// handleRestDirectorySearch returns users matching a query, for the admin
// add-user autocomplete. It merges three sources so any admin candidate can be
// found: SAML users (listed first, as they cannot otherwise be discovered),
// then the LDAP directory, then Entra source mirrors (Entra may eventually
// replace LDAP, so both are considered). Robin is intentionally excluded. Each
// result includes the resolved username so the caller never has to know the
func (app *Server) handleRestLdapSync(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !app.dir.LdapProg.Start(0, "Starting…") {
		writeJSON(w, map[string]interface{}{"started": false, "running": true})
		return
	}
	_ = app.db.AuditLog("LDAP", sess.Username, "Manual AD sync (all sources)")
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				app.dir.LdapProg.Finish("", fmt.Sprintf("sync crashed: %v", rec))
			}
		}()
		count, err := app.dir.RunADSyncProg(&app.dir.LdapProg)
		if err != nil {
			app.dir.LdapProg.Finish("", err.Error())
			return
		}
		app.dir.LdapProg.Finish(fmt.Sprintf("Mirrored %d placement(s).", count), "")
	}()
	writeJSON(w, map[string]interface{}{"started": true})
}

// handleRestLdapProgress returns the current AD sync progress snapshot.
func (app *Server) handleRestLdapProgress(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, app.dir.LdapProg.Snapshot())
}

// handleAuditReimport is the superadmin-only one-time action that re-imports the
// historical audit log from the legacy CompanyMaps 8 MySQL database. The original
// migration omitted the `auditlog` table, so production instances are missing
// their pre-migration history. This clears the local audit bucket, imports the
// old log (oldest-first) and lets live events continue to append on top. MySQL
