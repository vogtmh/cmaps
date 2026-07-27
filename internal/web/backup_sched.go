package web

import (
	"archive/zip"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Scheduled-backup settings keys (stored in the general settings bucket). They
// are excluded from the admin "Base variables" table in admin_data.go via the
// backupSchedKeyPrefix check.
const (
	backupSchedKeyPrefix = "backupSched"

	settingBackupSchedEnabled  = "backupSchedEnabled"  // "1" / ""
	settingBackupSchedInterval = "backupSchedInterval" // daily | weekly | monthly
	settingBackupSchedTime     = "backupSchedTime"     // "HH:MM" (24h, server local time)
	settingBackupSchedDest     = "backupSchedDest"     // relative (under data dir) or absolute path
	settingBackupSchedKeep     = "backupSchedKeep"     // integer, oldest pruned beyond this
	settingBackupSchedNextRun  = "backupSchedNextRun"  // RFC3339, survives restarts
	settingBackupSchedLastRun  = "backupSchedLastRun"  // RFC3339
)

const (
	// defaultBackupDest is the destination used when none is configured; it is
	// resolved relative to the data directory.
	defaultBackupDest = "backups"
	// defaultBackupKeep is the retention default (number of archives kept).
	defaultBackupKeep = 5
	// backupFilePrefix / backupFileSuffix bracket every scheduled archive name;
	// download/delete/prune only ever touch files matching this pattern.
	backupFilePrefix = "cmaps-backup-"
	backupFileSuffix = ".zip"
	// backupMissGrace is how long after a scheduled time a run is still treated
	// as "on time". A larger gap means the server was down, so the run is
	// skipped rather than fired late.
	backupMissGrace = 10 * time.Minute
)

// resolveBackupDest turns a configured destination (possibly empty or relative)
// into an absolute filesystem path. Relative paths are resolved under the data
// directory so a bare "backups" lands in <data>/backups.
func (app *Server) resolveBackupDest(dest string) string {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		dest = defaultBackupDest
	}
	if !filepath.IsAbs(dest) {
		dest = app.cfg.DataPath(dest)
	}
	if abs, err := filepath.Abs(dest); err == nil {
		return abs
	}
	return filepath.Clean(dest)
}

// backupDestDir returns the resolved absolute destination directory for
// scheduled backups.
func (app *Server) backupDestDir() string {
	return app.resolveBackupDest(app.db.GetSetting(settingBackupSchedDest))
}

// backupKeep returns the configured retention count (minimum 1).
func (app *Server) backupKeep() int {
	n, err := strconv.Atoi(strings.TrimSpace(app.db.GetSetting(settingBackupSchedKeep)))
	if err != nil || n < 1 {
		return defaultBackupKeep
	}
	return n
}

// nextTimeOfDay returns the next occurrence of the given HH:MM after now: today
// if the time is still ahead, otherwise tomorrow. This is the "first run" the
// user sees after enabling a schedule.
func nextTimeOfDay(now time.Time, hhmm string) time.Time {
	t, err := time.Parse("15:04", strings.TrimSpace(hhmm))
	if err != nil {
		t, _ = time.Parse("15:04", "03:00")
	}
	c := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	if !c.After(now) {
		c = c.AddDate(0, 0, 1)
	}
	return c
}

// advanceInterval moves t forward by one scheduling interval.
func advanceInterval(t time.Time, interval string) time.Time {
	switch interval {
	case "weekly":
		return t.AddDate(0, 0, 7)
	case "monthly":
		return t.AddDate(0, 1, 0)
	default: // daily
		return t.AddDate(0, 0, 1)
	}
}

// advanceAfter advances from by whole intervals until it is strictly after now,
// so missed runs (server was down) are skipped to the next future occurrence.
func advanceAfter(from, now time.Time, interval string) time.Time {
	next := from
	for i := 0; i < 4000 && !next.After(now); i++ {
		next = advanceInterval(next, interval)
	}
	return next
}

// setBackupNextRun persists the next scheduled run time.
func (app *Server) setBackupNextRun(t time.Time) {
	_ = app.db.SetSetting(settingBackupSchedNextRun, t.Format(time.RFC3339))
}

// nextRunLabel formats the next scheduled run for the admin panel.
func (app *Server) nextRunLabel() string {
	if app.db.GetSetting(settingBackupSchedEnabled) != "1" {
		return "Scheduled backups are disabled."
	}
	raw := app.db.GetSetting(settingBackupSchedNextRun)
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return "Not scheduled yet."
	}
	return t.Local().Format("Mon 2 Jan 2006, 15:04")
}

// lastRunLabel formats the last completed scheduled backup for the admin panel.
func (app *Server) lastRunLabel() string {
	raw := app.db.GetSetting(settingBackupSchedLastRun)
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return ""
	}
	return t.Local().Format("Mon 2 Jan 2006, 15:04")
}

// StartBackupScheduler runs a one-minute ticker that fires a scheduled backup
// when the configured time is reached. State lives in the settings bucket so the
// schedule survives restarts; missed runs are skipped, not caught up.
func (app *Server) StartBackupScheduler() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		app.backupSchedTick() // evaluate once shortly after boot
		for range ticker.C {
			app.backupSchedTick()
		}
	}()
}

// backupSchedTick evaluates the schedule once: it runs a backup when the next
// run time has just passed, or recomputes the next time when disabled/missed.
func (app *Server) backupSchedTick() {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Default().Error("scheduled backup tick crashed", "err", rec)
		}
	}()
	if app.db.GetSetting(settingBackupSchedEnabled) != "1" {
		return
	}
	interval := app.db.GetSetting(settingBackupSchedInterval)
	hhmm := app.db.GetSetting(settingBackupSchedTime)
	now := time.Now()

	next, err := time.Parse(time.RFC3339, app.db.GetSetting(settingBackupSchedNextRun))
	if err != nil {
		// No next time recorded yet (freshly enabled): arm the first run.
		app.setBackupNextRun(nextTimeOfDay(now, hhmm))
		return
	}
	if now.Before(next) {
		return // not time yet
	}
	if now.Sub(next) <= backupMissGrace {
		if err := app.runScheduledBackup(); err != nil {
			slog.Default().Error("scheduled backup failed", "err", err)
		} else {
			slog.Default().Info("scheduled backup complete")
		}
	} else {
		slog.Default().Warn("scheduled backup missed while server was down, skipping", "was", next.Local().Format(time.RFC3339))
	}
	app.setBackupNextRun(advanceAfter(next, now, interval))
}

// runScheduledBackup writes a full backup archive into the destination
// directory and prunes older archives beyond the retention count. The archive
// is written to a temp file first and atomically renamed so a partially written
// zip is never listed or restored.
func (app *Server) runScheduledBackup() error {
	dir := app.backupDestDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating backup dir: %w", err)
	}
	name := backupFilePrefix + time.Now().Format("20060102-150405") + backupFileSuffix
	full := filepath.Join(dir, name)
	tmpPath := full + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating backup file: %w", err)
	}
	zw := zip.NewWriter(f)
	if err := app.writeBackupZip(zw, nil); err != nil {
		_ = zw.Close()
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := zw.Close(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("finalizing backup zip: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("closing backup zip: %w", err)
	}
	if err := os.Rename(tmpPath, full); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("finalizing backup: %w", err)
	}

	_ = app.db.SetSetting(settingBackupSchedLastRun, time.Now().Format(time.RFC3339))
	_ = app.db.AuditLog("Settings", "system", "Scheduled backup created: "+name)
	app.pruneBackups(dir)
	return nil
}

// backupFile describes one stored backup archive.
type backupFile struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Modtime string `json:"modtime"`
	unix    int64
}

// listBackupFiles returns the stored backup archives in dir, newest first.
func listBackupFiles(dir string) []backupFile {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]backupFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.HasPrefix(n, backupFilePrefix) || !strings.HasSuffix(n, backupFileSuffix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, backupFile{
			Name:    n,
			Size:    info.Size(),
			Modtime: info.ModTime().Local().Format("2 Jan 2006, 15:04"),
			unix:    info.ModTime().Unix(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].unix > out[j].unix })
	return out
}

// pruneBackups deletes the oldest archives beyond the retention count.
func (app *Server) pruneBackups(dir string) {
	keep := app.backupKeep()
	files := listBackupFiles(dir)
	for i := keep; i < len(files); i++ {
		_ = os.Remove(filepath.Join(dir, files[i].Name))
	}
}

// handleRestBackupSchedSave validates and persists the schedule settings, then
// re-arms the next run time. Returns the resolved destination and next-run label.
func (app *Server) handleRestBackupSchedSave(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "config") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	_ = r.ParseForm()

	enabled := r.FormValue("enabled") == "1"
	interval := r.FormValue("interval")
	switch interval {
	case "daily", "weekly", "monthly":
	default:
		writeJSON(w, map[string]interface{}{"ok": false, "message": "Choose a valid interval."})
		return
	}
	hhmm := strings.TrimSpace(r.FormValue("time"))
	if _, err := time.Parse("15:04", hhmm); err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "Enter a valid time (HH:MM)."})
		return
	}
	keep, err := strconv.Atoi(strings.TrimSpace(r.FormValue("keep")))
	if err != nil || keep < 1 {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "Keep count must be at least 1."})
		return
	}
	dest := strings.TrimSpace(r.FormValue("dest"))

	resolved := app.resolveBackupDest(dest)
	if err := os.MkdirAll(resolved, 0o755); err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "Destination is not writable: " + err.Error()})
		return
	}

	_ = app.db.SetSetting(settingBackupSchedInterval, interval)
	_ = app.db.SetSetting(settingBackupSchedTime, hhmm)
	_ = app.db.SetSetting(settingBackupSchedKeep, strconv.Itoa(keep))
	_ = app.db.SetSetting(settingBackupSchedDest, dest)
	if enabled {
		_ = app.db.SetSetting(settingBackupSchedEnabled, "1")
		app.setBackupNextRun(nextTimeOfDay(time.Now(), hhmm))
	} else {
		_ = app.db.SetSetting(settingBackupSchedEnabled, "")
	}
	_ = app.db.AuditLog("Settings", sess.Username, "Backup schedule updated")

	writeJSON(w, map[string]interface{}{
		"ok":       true,
		"resolved": resolved,
		"nextRun":  app.nextRunLabel(),
	})
}

// handleRestBackupList returns the stored backup archives (newest first).
func (app *Server) handleRestBackupList(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "config") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	dir := app.backupDestDir()
	writeJSON(w, map[string]interface{}{
		"ok":       true,
		"resolved": dir,
		"nextRun":  app.nextRunLabel(),
		"lastRun":  app.lastRunLabel(),
		"files":    listBackupFiles(dir),
	})
}

// handleRestBackupDownload streams one stored backup archive. The name is
// reduced to its base and validated against the archive pattern to defend
// against path traversal.
func (app *Server) handleRestBackupDownload(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "config") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	name := filepath.Base(r.URL.Query().Get("name"))
	if !strings.HasPrefix(name, backupFilePrefix) || !strings.HasSuffix(name, backupFileSuffix) {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}
	full := filepath.Join(app.backupDestDir(), name)
	f, err := os.Open(full)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	http.ServeContent(w, r, name, info.ModTime(), f)
	_ = app.db.AuditLog("Settings", sess.Username, "Backup downloaded: "+name)
}

// handleRestBackupDelete removes one stored backup archive.
func (app *Server) handleRestBackupDelete(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "config") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	name := filepath.Base(r.FormValue("name"))
	if !strings.HasPrefix(name, backupFilePrefix) || !strings.HasSuffix(name, backupFileSuffix) {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "Invalid file name."})
		return
	}
	full := filepath.Join(app.backupDestDir(), name)
	if err := os.Remove(full); err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "Could not delete: " + err.Error()})
		return
	}
	_ = app.db.AuditLog("Settings", sess.Username, "Backup deleted: "+name)
	writeJSON(w, map[string]interface{}{"ok": true})
}

// handleRestBackupRunNow writes a scheduled-style backup immediately (does not
// change the schedule). Used by the "Run now" button.
func (app *Server) handleRestBackupRunNow(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "config") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := app.runScheduledBackup(); err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "message": err.Error()})
		return
	}
	_ = app.db.AuditLog("Settings", sess.Username, "Backup run manually")
	writeJSON(w, map[string]interface{}{"ok": true})
}
