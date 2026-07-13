package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// requireAdmin ensures the request is from the local break-glass admin (used to
// gate the setup wizard before RBAC data exists).
func (app *App) requireAdmin(w http.ResponseWriter, r *http.Request) (Session, bool) {
	sess, ok := app.currentSession(r)
	if !ok || !(sess.AdminPassword || app.permLevel(sess, "adminpanel") >= 2) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return Session{}, false
	}
	return sess, true
}

// handleSetup renders the setup wizard.
func (app *App) handleSetup(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.requireAdmin(w, r); !ok {
		return
	}
	app.render(w, "wizard.html", map[string]interface{}{
		"AppTitle":   app.appTitle(),
		"Configured": app.db.IsConfigured(),
		"Status":     r.URL.Query().Get("status"),
		"Error":      r.URL.Query().Get("error"),
		"Step":       app.db.GetMeta("migrate_step"),
		"DataPath":   orDefault(app.db.GetMeta("migrate_datapath"), "/var/www/html"),
	})
}

// handleSetupDemo runs PATH A (seed demo data).
func (app *App) handleSetupDemo(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.requireAdmin(w, r); !ok {
		return
	}
	if err := app.seedDemoData(); err != nil {
		http.Redirect(w, r, "/setup?error="+urlMsg("Seeding failed: "+err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleSetupImport runs PATH B step 1 (MySQL import). The import runs in the
// background so large databases don't hit the reverse-proxy gateway timeout; the
// browser polls /setup/import/progress for live status.
func (app *App) handleSetupImport(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.requireAdmin(w, r); !ok {
		return
	}
	c := MySQLConfig{
		Host:     orDefault(r.FormValue("host"), "localhost"),
		Port:     orDefault(r.FormValue("port"), "3306"),
		Database: r.FormValue("database"),
		User:     r.FormValue("user"),
		Password: r.FormValue("password"),
	}
	if !app.importProg.start() {
		writeJSON(w, map[string]interface{}{"started": false, "message": "An import is already running."})
		return
	}
	go func() {
		res, err := app.ImportFromMySQL(c, func(stage string, r ImportResult) {
			app.importProg.update(stage, r)
		})
		if err != nil {
			app.importProg.finish("", "Import failed: "+err.Error())
			return
		}
		_ = app.db.SetMeta("migrate_step", "datacopy")
		_ = app.db.SetMeta("setup_mode", "migrate")
		summary := fmt.Sprintf("Imported %d maps, %d desks, %d LDAP users, %d bookings, %d teams.",
			res.Maps, res.Desks, res.LdapUsers, res.Bookings, res.Teams)
		app.importProg.finish(summary, "")
	}()
	writeJSON(w, map[string]interface{}{"started": true})
}

// handleSetupImportProgress returns the current MySQL import progress as JSON.
func (app *App) handleSetupImportProgress(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.requireAdmin(w, r); !ok {
		return
	}
	writeJSON(w, app.importProg.snapshot())
}

// handleSetupDataCopy runs PATH B step 2 (copy maps + avatarcache from legacy disk path).
func (app *App) handleSetupDataCopy(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.requireAdmin(w, r); !ok {
		return
	}
	src := strings.TrimRight(orDefault(r.FormValue("datapath"), "/var/www/html"), "/")
	_ = app.db.SetMeta("migrate_datapath", src)

	maps, err1 := copyDir(filepath.Join(src, "maps"), app.cfg.DataPath("maps"))
	avs, err2 := copyDir(filepath.Join(src, "avatarcache"), app.cfg.DataPath("avatarcache"))
	if err1 != nil && err2 != nil {
		http.Redirect(w, r, "/setup?error="+urlMsg(fmt.Sprintf("Copy failed (maps: %v, avatars: %v)", err1, err2)), http.StatusSeeOther)
		return
	}
	_ = app.db.SetMeta("migrate_step", "ldap")
	_ = app.db.AuditLog("setup", "admin", fmt.Sprintf("data copy: %d maps, %d avatars from %s", maps, avs, src))
	http.Redirect(w, r, "/setup?status="+urlMsg(fmt.Sprintf("Copied %d map files and %d avatars.", maps, avs)), http.StatusSeeOther)
}

// handleSetupLdap runs PATH B step 3 (store AD sync credentials + run initial sync).
func (app *App) handleSetupLdap(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.requireAdmin(w, r); !ok {
		return
	}
	if r.FormValue("skip") == "1" {
		_ = app.db.SetMeta("migrate_step", "robin")
		http.Redirect(w, r, "/setup?status="+urlMsg("Skipped AD sync setup."), http.StatusSeeOther)
		return
	}
	src := LdapSource{
		ID:          1,
		Description: orDefault(r.FormValue("description"), "Primary AD"),
		Server:      r.FormValue("server"),
		Type:        orDefault(r.FormValue("type"), "LDAPS"),
		OU:          r.FormValue("ou"),
		LdapUser:    r.FormValue("ldapuser"),
		LdapPass:    r.FormValue("ldappass"),
	}
	// If sources already imported, append rather than overwrite ID 1.
	if existing, _ := app.db.ListLdapSources(); len(existing) > 0 && r.FormValue("server") == "" {
		_ = app.db.SetMeta("migrate_step", "robin")
		http.Redirect(w, r, "/setup?status="+urlMsg("Using imported AD source(s)."), http.StatusSeeOther)
		return
	}
	if err := app.db.PutLdapSource(src); err != nil {
		http.Redirect(w, r, "/setup?error="+urlMsg("Saving AD source failed: "+err.Error()), http.StatusSeeOther)
		return
	}
	// Run an initial sync (best effort).
	if n, err := app.dir.RunADSync(); err != nil {
		http.Redirect(w, r, "/setup?error="+urlMsg("AD source saved but initial sync failed: "+err.Error()), http.StatusSeeOther)
		return
	} else {
		_ = app.db.SetMeta("migrate_step", "robin")
		http.Redirect(w, r, "/setup?status="+urlMsg(fmt.Sprintf("AD sync complete: %d users mirrored.", n)), http.StatusSeeOther)
	}
}

// handleSetupRobin runs PATH B step 4 (store Robin token; skippable).
func (app *App) handleSetupRobin(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.requireAdmin(w, r); !ok {
		return
	}
	if r.FormValue("skip") != "1" {
		_ = app.db.SetRobinSetting("robintoken", r.FormValue("robintoken"))
		_ = app.db.SetRobinSetting("robinOrganisation", r.FormValue("robinorg"))
	}
	_ = app.db.SetMeta("migrate_step", "done")
	http.Redirect(w, r, "/setup?status="+urlMsg("Robin configuration saved. Setup is almost complete."), http.StatusSeeOther)
}

// handleSetupFinish marks the wizard complete.
func (app *App) handleSetupFinish(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.requireAdmin(w, r); !ok {
		return
	}
	_ = app.db.SetMeta("setup_done", "1")
	_ = app.db.AuditLog("setup", "admin", "setup wizard finished")
	http.Redirect(w, r, "/setup?status="+urlMsg("Setup complete!"), http.StatusSeeOther)
}

// --- helpers ---

func urlMsg(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "+"), "&", "%26")
}

// copyDir copies regular files from src into dst (non-recursive for maps/avatars,
// which are flat directories). Returns the number of files copied.
func copyDir(src, dst string) (int, error) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := copyFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err == nil {
			count++
		}
	}
	return count, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
