package main

import (
	"fmt"
	"net/http"
	"strings"
)

// handleAdminPostDashboard manages the dashboard health-check whitelist.
func (app *App) handleAdminPostDashboard(r *http.Request, sess Session) string {
	if app.permLevel(sess, "health") < 1 {
		return ""
	}
	if delName := r.FormValue("deleteWhitelistName"); delName != "" {
		delType := r.FormValue("deleteWhitelistType")
		_ = app.db.DeleteWhitelist(WhitelistEntry{Type: delType, Text: delName})
		_ = app.db.AuditLog("Health", sess.Username, "Whitelist entry removed ("+delType+": "+delName+")")
		return "Whitelist entry removed."
	}
	name := r.FormValue("ignoreHealthName")
	typ := r.FormValue("ignoreHealthType")
	if name != "" && typ != "" {
		_ = app.db.AddWhitelist(WhitelistEntry{Type: typ, Text: name})
		_ = app.db.AuditLog("Health", sess.Username, "Whitelist entry added ("+typ+": "+name+")")
		return "Whitelist updated."
	}
	return ""
}

func (app *App) handleAuditReimport(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	if app.permLevel(sess, "adminpanel") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c := MySQLConfig{
		Host:     orDefault(r.FormValue("host"), "localhost"),
		Port:     orDefault(r.FormValue("port"), "3306"),
		Database: strings.TrimSpace(r.FormValue("database")),
		User:     strings.TrimSpace(r.FormValue("user")),
		Password: r.FormValue("password"),
	}
	if c.Database == "" || c.User == "" {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "Database and user are required."})
		return
	}
	count, err := app.ImportAuditOnly(c)
	if err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "message": err.Error()})
		return
	}
	_ = app.db.AuditLog("auditlog", sess.Username, fmt.Sprintf("Audit log re-imported from MySQL (%d historical entries)", count))
	writeJSON(w, map[string]interface{}{"ok": true, "count": count,
		"message": fmt.Sprintf("Imported %d historical audit entries.", count)})
}
