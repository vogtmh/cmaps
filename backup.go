package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

// backupGroup describes one selectable unit of data for the import dialog. A
// group is backed either by a set of bolt buckets (Dir == "") or by an asset
// directory under the data dir (Buckets == nil). Export always writes every
// group; import restores only the groups the admin ticks.
type backupGroup struct {
	Key     string
	Label   string
	Buckets [][]byte
	Dir     string
}

// backupGroups is the authoritative list of import-selectable data sets. Order
// is the order shown in the import dialog.
var backupGroups = []backupGroup{
	{Key: "maps", Label: "Maps & desks", Buckets: [][]byte{bucketMaps, bucketDesks}},
	{Key: "users", Label: "Users, roles, teams & departments", Buckets: [][]byte{bucketUsers, bucketRoles, bucketTeams, bucketDepts, bucketVips}},
	{Key: "ldap", Label: "LDAP directory & sources", Buckets: [][]byte{bucketLdap, bucketDirectory, bucketLdapSrc, bucketChangelog}},
	{Key: "bookings", Label: "Bookings", Buckets: [][]byte{bucketBookings}},
	{Key: "settings", Label: "Settings & integrations", Buckets: [][]byte{bucketSettings, bucketMeta, bucketGeoCfg, bucketRobinCfg, bucketRobin, bucketRobinDesk, bucketMeeting, bucketWhitelist}},
	{Key: "stats", Label: "Statistics & audit log", Buckets: [][]byte{bucketStats, bucketTracking, bucketAudit}},
	{Key: "mapimages", Label: "Map images", Dir: "maps"},
	{Key: "logos", Label: "Logos", Dir: "logos"},
	{Key: "avatars", Label: "Avatar cache", Dir: "avatarcache"},
}

// backupAssetDirs lists the data-dir subfolders that are bundled into an export
// (everything that lives under the data dir besides the bolt database). The
// config.json file lives outside the data dir and is deliberately never exported.
var backupAssetDirs = []string{"maps", "logos", "avatarcache"}

// handleRestExportStart kicks off building the export zip in the background so
// the admin Backup dialog can show a determinate progress bar while everything
// is zipped, then download the finished file.
func (app *App) handleRestExportStart(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "config") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !app.exportProg.start(0, "Preparing…") {
		writeJSON(w, map[string]interface{}{"started": false, "running": true})
		return
	}
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				app.exportProg.finish("", fmt.Sprintf("export crashed: %v", rec))
			}
		}()
		if err := app.buildExport(); err != nil {
			app.exportProg.finish("", err.Error())
			return
		}
		app.exportProg.finish("Export ready.", "")
	}()
	_ = app.db.AuditLog("Settings", sess.Username, "Data export started")
	writeJSON(w, map[string]interface{}{"started": true})
}

// handleRestExportProgress returns the current export build progress snapshot.
func (app *App) handleRestExportProgress(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "config") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, app.exportProg.snapshot())
}

// handleRestExportDownload streams the most recently built export zip and then
// deletes the temporary file so it does not linger on disk.
func (app *App) handleRestExportDownload(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "config") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	app.exportMu.Lock()
	tmpPath := app.exportPath
	name := app.exportName
	app.exportMu.Unlock()
	if tmpPath == "" {
		http.Error(w, "no export available", http.StatusNotFound)
		return
	}
	f, err := os.Open(tmpPath)
	if err != nil {
		http.Error(w, "export not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, "export not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	_, _ = io.Copy(w, f)

	// One-shot download: drop the temp file and clear the pointer afterwards.
	f.Close()
	_ = os.Remove(tmpPath)
	app.exportMu.Lock()
	if app.exportPath == tmpPath {
		app.exportPath = ""
		app.exportName = ""
	}
	app.exportMu.Unlock()
	_ = app.db.AuditLog("Settings", sess.Username, "Data export downloaded")
}

// buildExport writes a consistent snapshot of the bolt database plus every asset
// directory into a fresh temp zip, updating exportProg as it goes. The resulting
// path is stored on the App for handleRestExportDownload.
func (app *App) buildExport() error {
	// Drop any previous export still waiting to be downloaded.
	app.exportMu.Lock()
	if app.exportPath != "" {
		_ = os.Remove(app.exportPath)
		app.exportPath = ""
		app.exportName = ""
	}
	app.exportMu.Unlock()

	// Count asset files up front so the progress bar is determinate (+1 for db).
	total := 1
	for _, dir := range backupAssetDirs {
		total += countFiles(app.cfg.dataPath(dir))
	}
	app.exportProg.beginPhase(total, "Snapshotting database…")

	tmp, err := os.CreateTemp("", "cmaps-export-*.zip")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	zw := zip.NewWriter(tmp)

	// manifest.json — metadata describing what the archive contains.
	manifest := map[string]interface{}{
		"app":       "CompanyMaps",
		"kind":      "cmaps-backup",
		"version":   1,
		"generated": time.Now().Format(time.RFC3339),
	}
	if mb, err := json.MarshalIndent(manifest, "", "  "); err == nil {
		if fw, err := zw.Create("manifest.json"); err == nil {
			_, _ = fw.Write(mb)
		}
	}

	// cmaps.db — consistent bolt snapshot written straight into the archive.
	dbWriter, err := zw.Create("cmaps.db")
	if err != nil {
		_ = closeAndRemove(tmp, zw, tmpPath)
		return fmt.Errorf("zip db entry: %w", err)
	}
	if err := app.db.bolt.View(func(tx *bolt.Tx) error {
		_, werr := tx.WriteTo(dbWriter)
		return werr
	}); err != nil {
		_ = closeAndRemove(tmp, zw, tmpPath)
		return fmt.Errorf("db snapshot: %w", err)
	}
	app.exportProg.step("Adding files…")

	// Asset directories — copied file by file, preserving the dir/<name> layout.
	for _, dir := range backupAssetDirs {
		root := app.cfg.dataPath(dir)
		err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable / missing entries
			}
			if d.IsDir() {
				return nil
			}
			rel, rerr := filepath.Rel(app.cfg.DataDir, p)
			if rerr != nil {
				return nil
			}
			fw, cerr := zw.Create(filepath.ToSlash(rel))
			if cerr != nil {
				return nil
			}
			src, oerr := os.Open(p)
			if oerr != nil {
				return nil
			}
			_, _ = io.Copy(fw, src)
			_ = src.Close()
			app.exportProg.step("")
			return nil
		})
		if err != nil {
			_ = closeAndRemove(tmp, zw, tmpPath)
			return fmt.Errorf("archiving %s: %w", dir, err)
		}
	}

	if err := zw.Close(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("finalizing zip: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("closing zip: %w", err)
	}

	app.exportMu.Lock()
	app.exportPath = tmpPath
	app.exportName = "cmaps-backup-" + time.Now().Format("20060102-150405") + ".zip"
	app.exportMu.Unlock()
	return nil
}

// closeAndRemove tears down a half-built export after an error.
func closeAndRemove(f *os.File, zw *zip.Writer, p string) error {
	_ = zw.Close()
	_ = f.Close()
	return os.Remove(p)
}

// countFiles returns the number of regular files under root (0 if it is missing).
func countFiles(root string) int {
	n := 0
	_ = filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			n++
		}
		return nil
	})
	return n
}

// importResult summarizes what an import applied, per group.
type importResult struct {
	Group   string `json:"group"`
	Label   string `json:"label"`
	Records int    `json:"records"`
	Files   int    `json:"files"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// handleRestImport accepts an uploaded export zip plus a set of selected groups
// and restores exactly those groups, overwriting existing data. Database groups
// replace their buckets wholesale; asset groups replace the directory contents.
func (app *App) handleRestImport(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "config") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "Could not read upload: " + err.Error()})
		return
	}
	file, _, err := r.FormFile("archive")
	if err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "No file uploaded."})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "Could not read upload: " + err.Error()})
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "Not a valid zip archive."})
		return
	}

	// Which groups did the admin tick?
	selected := map[string]bool{}
	for _, g := range backupGroups {
		if r.FormValue("group_"+g.Key) != "" {
			selected[g.Key] = true
		}
	}
	if len(selected) == 0 {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "Select at least one data set to import."})
		return
	}

	// Index zip entries by their archive path for quick lookup.
	entries := map[string]*zip.File{}
	for _, f := range zr.File {
		entries[f.Name] = f
	}

	results := make([]importResult, 0, len(selected))
	needsDB := false
	for _, g := range backupGroups {
		if selected[g.Key] && g.Dir == "" {
			needsDB = true
			break
		}
	}

	// Open the archived bolt db once (read-only) if any db group is selected.
	var srcDB *bolt.DB
	if needsDB {
		dbFile := entries["cmaps.db"]
		if dbFile == nil {
			writeJSON(w, map[string]interface{}{"ok": false, "message": "Archive does not contain a database (cmaps.db)."})
			return
		}
		tmp, err := os.CreateTemp("", "cmaps-import-*.db")
		if err != nil {
			writeJSON(w, map[string]interface{}{"ok": false, "message": "Server error: " + err.Error()})
			return
		}
		tmpPath := tmp.Name()
		defer os.Remove(tmpPath)
		rc, err := dbFile.Open()
		if err == nil {
			_, _ = io.Copy(tmp, rc)
			_ = rc.Close()
		}
		_ = tmp.Close()
		srcDB, err = bolt.Open(tmpPath, 0600, &bolt.Options{ReadOnly: true, Timeout: 5 * time.Second})
		if err != nil {
			writeJSON(w, map[string]interface{}{"ok": false, "message": "Could not read archived database: " + err.Error()})
			return
		}
		defer srcDB.Close()
	}

	for _, g := range backupGroups {
		if !selected[g.Key] {
			continue
		}
		if g.Dir != "" {
			n, err := app.importAssetDir(zr, g.Dir)
			res := importResult{Group: g.Key, Label: g.Label, Files: n, Status: "ok"}
			if err != nil {
				res.Status = "failed"
				res.Message = err.Error()
			}
			results = append(results, res)
			continue
		}
		n, err := app.importBuckets(srcDB, g.Buckets)
		res := importResult{Group: g.Key, Label: g.Label, Records: n, Status: "ok"}
		if err != nil {
			res.Status = "failed"
			res.Message = err.Error()
		}
		results = append(results, res)
	}

	labels := make([]string, 0, len(results))
	for _, res := range results {
		labels = append(labels, res.Label)
	}
	_ = app.db.AuditLog("Settings", sess.Username, "Data import: "+strings.Join(labels, ", "))
	writeJSON(w, map[string]interface{}{"ok": true, "results": results})
}

// importBuckets replaces each named bucket in the live database with the
// contents of the same bucket in the archived database. A bucket missing from
// the source is cleared (so the import is an exact overwrite, never a merge).
func (app *App) importBuckets(srcDB *bolt.DB, buckets [][]byte) (int, error) {
	// Read all source entries into memory first so we never hold a read tx on the
	// source while writing the destination.
	type kv struct{ k, v []byte }
	staged := make(map[string][]kv, len(buckets))
	count := 0
	if err := srcDB.View(func(tx *bolt.Tx) error {
		for _, name := range buckets {
			b := tx.Bucket(name)
			if b == nil {
				staged[string(name)] = nil
				continue
			}
			var rows []kv
			_ = b.ForEach(func(k, v []byte) error {
				if v == nil {
					return nil // nested buckets are not used in this schema
				}
				rows = append(rows, kv{append([]byte(nil), k...), append([]byte(nil), v...)})
				return nil
			})
			staged[string(name)] = rows
		}
		return nil
	}); err != nil {
		return 0, err
	}

	if err := app.db.bolt.Update(func(tx *bolt.Tx) error {
		for _, name := range buckets {
			if err := tx.DeleteBucket(name); err != nil && err != bolt.ErrBucketNotFound {
				return err
			}
			nb, err := tx.CreateBucket(name)
			if err != nil {
				return err
			}
			for _, row := range staged[string(name)] {
				if err := nb.Put(row.k, row.v); err != nil {
					return err
				}
				count++
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}
	return count, nil
}

// importAssetDir clears the named data-dir subfolder and restores its files from
// the archive (an overwrite, not a merge). Only files stored under "<dir>/" in
// the archive are considered, and each is flattened to its base name to defend
// against zip path traversal.
func (app *App) importAssetDir(zr *zip.Reader, dir string) (int, error) {
	dst := app.cfg.dataPath(dir)
	if err := os.MkdirAll(dst, 0755); err != nil {
		return 0, err
	}
	// Clear existing files (keep the directory itself).
	if existing, err := os.ReadDir(dst); err == nil {
		for _, e := range existing {
			if !e.IsDir() {
				_ = os.Remove(filepath.Join(dst, e.Name()))
			}
		}
	}
	prefix := dir + "/"
	n := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := path.Clean(f.Name)
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		base := filepath.Base(name)
		if base == "." || base == ".." || base == "" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		out, err := os.Create(filepath.Join(dst, base))
		if err != nil {
			_ = rc.Close()
			continue
		}
		_, _ = io.Copy(out, rc)
		_ = out.Close()
		_ = rc.Close()
		n++
	}
	return n, nil
}
