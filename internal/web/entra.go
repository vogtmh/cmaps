package web

import (
	"companymaps/internal/directory"

	"fmt"
	"net/http"
	"strconv"
	"time"
)

// ──────────────────────────────────────────────
// REST handlers
// ──────────────────────────────────────────────

// handleRestEntraSync starts an EntraID sync in the background so the admin Sync
// tab can poll for live progress.
func (app *Server) handleRestEntraSync(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !app.dir.EntraHasEnabledSource() {
		writeJSON(w, map[string]interface{}{"started": false, "error": "No enabled EntraID connection."})
		return
	}
	if !app.dir.EntraProg.Start(0, "Starting…") {
		writeJSON(w, map[string]interface{}{"started": false, "running": true})
		return
	}
	_ = app.db.AuditLog("LDAP", sess.Username, "Manual EntraID sync")
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				app.dir.EntraProg.Finish("", fmt.Sprintf("sync crashed: %v", rec))
			}
		}()
		count, err := app.dir.RunEntraSyncProg(&app.dir.EntraProg)
		if err != nil {
			app.dir.EntraProg.Finish("", err.Error())
			return
		}
		app.dir.EntraProg.Finish(fmt.Sprintf("Mirrored %d placement(s).", count), "")
	}()
	writeJSON(w, map[string]interface{}{"started": true})
}

// handleRestEntraProgress returns the current EntraID sync progress snapshot.
func (app *Server) handleRestEntraProgress(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, app.dir.EntraProg.Snapshot())
}

// handleRestEntraTest validates one EntraID source's credentials by acquiring a
// token and making a minimal Microsoft Graph call, without running a full sync.
func (app *Server) handleRestEntraTest(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	id, err := strconv.Atoi(r.URL.Query().Get("entraid"))
	if err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "checks": []testCheck{{Name: "Connection", Status: "fail", Detail: "invalid EntraID id"}}})
		return
	}
	writeJSON(w, app.entraValidate(id))
}

// entraValidate runs a structured, read-only connectivity test for a single
// EntraID connection and returns the {ok, checks} payload rendered by the admin
// test modal. It acquires an access token and confirms Microsoft Graph is
// reachable — without performing a full sync.
func (app *Server) entraValidate(id int) map[string]interface{} {
	var checks []testCheck
	add := func(name, status, detail string) {
		checks = append(checks, testCheck{Name: name, Status: status, Detail: detail})
	}

	srcs, _ := app.db.ListEntraSources()
	var src *EntraSource
	for i := range srcs {
		if srcs[i].ID == id {
			src = &srcs[i]
			break
		}
	}
	if src == nil {
		return testResult([]testCheck{{Name: "Connection", Status: "fail", Detail: "EntraID connection not found."}})
	}

	if src.Disabled {
		add("Connection status", "warn", "This connection is disabled and is skipped during sync.")
	} else {
		add("Connection status", "ok", "This connection is enabled.")
	}

	client, err := directory.NewEntraClient(*src)
	if err != nil {
		add("Access token", "fail", "Could not build the Graph client: "+err.Error())
		return testResult(checks)
	}
	if _, err := client.AccessToken(); err != nil {
		add("Access token", "fail", "Could not acquire an access token: "+err.Error())
		return testResult(checks)
	}
	add("Access token", "ok", "Access token acquired for tenant "+src.TenantID+".")

	var page struct {
		Count int                        `json:"@odata.count"`
		Value []directory.EntraGraphUser `json:"value"`
	}
	if err := client.Get("/users?$top=1&$count=true&$select=id,displayName", &page); err != nil {
		add("Microsoft Graph", "fail", "Graph request failed: "+err.Error())
		return testResult(checks)
	}
	add("Microsoft Graph", "ok", "Microsoft Graph is reachable.")

	if page.Count > 0 {
		add("Users visible", "ok", fmt.Sprintf("%d user(s) visible in the tenant.", page.Count))
	} else {
		add("Users visible", "warn", "No users were returned — check the app registration's permissions and scope.")
	}

	return testResult(checks)
}

// handleRestEntraSyncOne synchronously syncs a single EntraID connection (the
// per-connection "Sync now" button), mirroring the LDAP per-source sync: the
// EntraID mirror is replaced with just this source's users.
func (app *Server) handleRestEntraSyncOne(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	id, err := strconv.Atoi(r.URL.Query().Get("entraid"))
	if err != nil {
		http.Error(w, "invalid EntraID id", http.StatusBadRequest)
		return
	}
	srcs, _ := app.db.ListEntraSources()
	var src *EntraSource
	for i := range srcs {
		if srcs[i].ID == id {
			src = &srcs[i]
			break
		}
	}
	if src == nil {
		http.Error(w, "EntraID source not found", http.StatusNotFound)
		return
	}
	if src.Disabled {
		http.Error(w, "connection is disabled", http.StatusConflict)
		return
	}
	// If the per-source buckets have not been seeded yet (fresh upgrade), fall
	// back to a full sync so we never publish a mirror built from just one
	// source (which would drop the others until their next sync).
	if app.db.GetMeta("entraSeeded") != "1" {
		count, err := app.dir.RunEntraSync()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = app.db.AuditLog("LDAP", sess.Username, "Manual EntraID sync of source "+strconv.Itoa(id))
		writeJSON(w, map[string]interface{}{"status": "ok", "count": count, "lastSync": nowTimestamp()})
		return
	}
	client, err := directory.NewEntraClient(*src)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	users, err := client.ListUsers()
	if err != nil {
		http.Error(w, "graph list users: "+err.Error(), http.StatusInternalServerError)
		return
	}
	dir := make([]DirectoryUser, 0, len(users))
	for _, u := range users {
		if !u.AccountEnabled {
			continue
		}
		dir = append(dir, app.dir.GraphUserToDirectory(u))
	}
	mirror := directory.DeriveMirrorUsers(dir)
	if err := app.db.PutSourceMirror("entra", src.ID, mirror); err != nil {
		http.Error(w, "writing EntraID mirror: "+err.Error(), http.StatusInternalServerError)
		return
	}
	count, err := app.dir.RebuildEntraMirror()
	if err != nil {
		http.Error(w, "writing EntraID mirror: "+err.Error(), http.StatusInternalServerError)
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	src.LastSync = now
	_ = app.db.PutEntraSource(*src)
	_ = app.db.SetEntraSetting("entraLastSync", now)
	_ = app.db.AuditLog("LDAP", sess.Username, "Manual EntraID sync of source "+strconv.Itoa(id))
	writeJSON(w, map[string]interface{}{"status": "ok", "count": count, "lastSync": now})
}

// handleRestEntraGenCert generates a fresh self-signed RSA certificate and
// private key (both PEM-encoded) for use with EntraID certificate auth. The
// admin uploads the certificate to the app registration and keeps the private
// key here. Nothing is persisted server-side — the caller fills the form.
func (app *Server) handleRestEntraGenCert(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	certPEM, keyPEM, err := directory.GenerateEntraCert()
	if err != nil {
		writeJSON(w, map[string]interface{}{"status": "error", "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "cert": certPEM, "key": keyPEM})
}
