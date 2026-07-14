package web

import (
	"companymaps/internal/store"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

// This file implements the identifier migration assistant: converting all
// existing data (avatar files, map-admin records, audit log, changelog, bookings
// and desk avatar references) between the samaccountname and mail identifier
// formats. The migration is a two-phase "create the new, then delete the old"
// process. The identifier setting itself is flipped only after the data has been
// converted, so an aborted run never leaves a half-migrated, broken state.

// identifierForMode returns the identifier a user would have under the given
// mode, from their raw samaccountname and mail address.
func identifierForMode(mode, sam, mail string) string {
	if mode == "mail" {
		return mailIdentifier(mail)
	}
	return strings.TrimSpace(sam)
}

// migConflict describes a user that cannot be migrated and is therefore skipped,
// leaving their existing data untouched for manual resolution.
type migConflict struct {
	Old    string `json:"old"`
	New    string `json:"new,omitempty"`
	Reason string `json:"reason"`
}

// migPlan is the resolved old->new identifier mapping for a migration, plus the
// conflicts that were skipped.
type migPlan struct {
	current   string
	target    string
	domain    string
	bare      map[string]string // lower(old bare id) -> new bare id
	conflicts []migConflict
	already   int
	mappable  int
	totalDir  int
}

// buildMigPlan computes the old->new identifier mapping from the directory
// snapshot for the requested target mode, detecting conflicts (users with no
// target identifier, or whose target identifier collides with another user).
func (app *Server) buildMigPlan(target string) (*migPlan, error) {
	current := app.identifierMode()
	dir, err := app.db.ListDirectory()
	if err != nil {
		return nil, err
	}
	plan := &migPlan{
		current:  current,
		target:   target,
		domain:   app.db.GetSetting("domain"),
		bare:     map[string]string{},
		totalDir: len(dir),
	}

	type cand struct{ oldBare, newBare string }
	var cands []cand
	newCount := map[string]int{}

	for _, d := range dir {
		oldBare := strings.TrimSpace(d.Userid)
		if oldBare == "" {
			continue
		}
		sam := strings.TrimSpace(d.Samaccountname)
		if sam == "" && current == "samaccountname" {
			sam = oldBare
		}
		newBare := identifierForMode(target, sam, d.Mail)
		if newBare == "" {
			plan.conflicts = append(plan.conflicts, migConflict{Old: oldBare, Reason: "no target identifier (user has no mail address)"})
			continue
		}
		if strings.EqualFold(newBare, oldBare) {
			plan.already++
			continue
		}
		cands = append(cands, cand{oldBare, newBare})
		newCount[strings.ToLower(newBare)]++
	}

	for _, c := range cands {
		if newCount[strings.ToLower(c.newBare)] > 1 {
			plan.conflicts = append(plan.conflicts, migConflict{Old: c.oldBare, New: c.newBare, Reason: "target identifier collides with another user"})
			continue
		}
		plan.bare[strings.ToLower(c.oldBare)] = c.newBare
		plan.mappable++
	}
	return plan, nil
}

// mapBare returns the new bare identifier for an old bare identifier (avatar
// filename, booking user, changelog avatar, desk avatar).
func (p *migPlan) mapBare(old string) (string, bool) {
	nb, ok := p.bare[strings.ToLower(strings.TrimSpace(old))]
	return nb, ok
}

// mapUsername returns the new map-admin/audit username for an old one. In
// samaccountname mode the username keeps the DOMAIN\ prefix; in mail mode it is
// the bare mail identifier. Values that do not correspond to a directory user
// (e.g. "System", local accounts, users who have left) are returned unchanged.
func (p *migPlan) mapUsername(oldUser string) (string, bool) {
	bareOld := oldUser
	if i := strings.LastIndex(oldUser, "\\"); i >= 0 {
		bareOld = oldUser[i+1:]
	}
	nb, ok := p.bare[strings.ToLower(bareOld)]
	if !ok {
		return oldUser, false
	}
	if p.target == "mail" {
		return nb, true
	}
	if p.domain != "" {
		return p.domain + "\\" + nb, true
	}
	return nb, true
}

// bucketCount returns the number of keys in a bucket.
func (app *Server) bucketCount(bucket []byte) int {
	n := 0
	_ = app.db.Bolt().View(func(tx *bolt.Tx) error {
		if b := tx.Bucket(bucket); b != nil {
			n = b.Stats().KeyN
		}
		return nil
	})
	return n
}

// avatarCount returns the number of cached avatar files.
func (app *Server) avatarCount() int {
	entries, err := os.ReadDir(app.cfg.DataPath("avatarcache"))
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) > 4 && strings.EqualFold(e.Name()[len(e.Name())-4:], ".jpg") {
			n++
		}
	}
	return n
}

// deskAvatarCount returns the number of desks that carry an avatar reference.
func (app *Server) deskAvatarCount() int {
	desks, _ := app.db.ListAllDesks()
	n := 0
	for _, d := range desks {
		if strings.TrimSpace(d.Avatar) != "" {
			n++
		}
	}
	return n
}

// copyFile is defined in wizard.go.

// ── Shadow-bucket staging ───────────────────────────────────
//
// The migration never touches live data — neither database records nor avatar
// files — until the admin presses Apply. The "create" step converts every record
// into a set of temporary staging buckets and records exactly how many rows
// changed per area. Apply then copies the staged rows into the live buckets and
// performs the avatar file renames in one go. Cancelling, or one hour of
// inactivity, simply drops the staging buckets, leaving the live data pristine.

var (
	bktMigMeta      = []byte("migstage_meta")
	bktMigAudit     = []byte("migstage_audit")
	bktMigChangelog = []byte("migstage_changelog")
	bktMigBookings  = []byte("migstage_bookings")
	bktMigDesks     = []byte("migstage_desks")
	bktMigAdmins    = []byte("migstage_admins")
)

var migStageBuckets = [][]byte{bktMigMeta, bktMigAudit, bktMigChangelog, bktMigBookings, bktMigDesks, bktMigAdmins}

const migStageTTL = time.Hour

// migStageArea is one row of the "changes staged" breakdown shown in step 3.
type migStageArea struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Total   int    `json:"total"`
	Changed int    `json:"changed"`
}

// migStageInfo is the staging metadata persisted in bktMigMeta.
type migStageInfo struct {
	Target    string         `json:"target"`
	Current   string         `json:"current"`
	CreatedAt int64          `json:"createdAt"`
	Conflicts int            `json:"conflicts"`
	Areas     []migStageArea `json:"areas"`
}

// stageExpired reports whether staging metadata is older than the TTL.
func (info migStageInfo) stageExpired() bool {
	return time.Since(time.Unix(info.CreatedAt, 0)) > migStageTTL
}

// purgeStaging drops all staging buckets. No avatar files are ever created during
// staging, so nothing on disk needs cleaning up.
func (app *Server) purgeStaging() {
	_ = app.db.Bolt().Update(func(tx *bolt.Tx) error {
		for _, b := range migStageBuckets {
			_ = tx.DeleteBucket(b)
		}
		return nil
	})
}

// readStageInfo returns the current staging metadata, if any.
func (app *Server) readStageInfo() (migStageInfo, bool) {
	var info migStageInfo
	found := false
	_ = app.db.Bolt().View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bktMigMeta)
		if b == nil {
			return nil
		}
		v := b.Get([]byte("info"))
		if v == nil {
			return nil
		}
		if json.Unmarshal(v, &info) == nil {
			found = true
		}
		return nil
	})
	return info, found
}

func (app *Server) writeStageInfo(info migStageInfo) error {
	return app.db.Bolt().Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bktMigMeta)
		if err != nil {
			return err
		}
		data, err := json.Marshal(info)
		if err != nil {
			return err
		}
		return b.Put([]byte("info"), data)
	})
}

// purgeStaleStaging drops the staging buckets if they have expired.
func (app *Server) purgeStaleStaging() {
	if info, ok := app.readStageInfo(); ok && info.stageExpired() {
		app.purgeStaging()
	}
}

// startMigStageJanitor periodically discards abandoned migration staging.
func (app *Server) startMigStageJanitor(interval time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for range t.C {
			app.purgeStaleStaging()
		}
	}()
}

// stageBucket converts every record in src through transform and writes the
// changed rows (same key) into a freshly recreated dst staging bucket. It
// returns the total rows scanned and the number of changed rows staged.
func (app *Server) stageBucket(dst, src []byte, transform func(v []byte) ([]byte, bool)) (total, changed int, err error) {
	err = app.db.Bolt().Update(func(tx *bolt.Tx) error {
		_ = tx.DeleteBucket(dst)
		d, e := tx.CreateBucket(dst)
		if e != nil {
			return e
		}
		s := tx.Bucket(src)
		if s == nil {
			return nil
		}
		c := s.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			total++
			nv, ch := transform(v)
			if !ch {
				continue
			}
			if e := d.Put(append([]byte(nil), k...), nv); e != nil {
				return e
			}
			changed++
		}
		return nil
	})
	return
}

// stageAdmins stages the map-admin (directory-derived) user records that need
// re-keying. Each staged row is keyed by the OLD username and holds the JSON of
// the new user (carrying the new username), so apply can delete the old key and
// insert the new one.
func (app *Server) stageAdmins(plan *migPlan) (total, changed int, err error) {
	err = app.db.Bolt().Update(func(tx *bolt.Tx) error {
		_ = tx.DeleteBucket(bktMigAdmins)
		d, e := tx.CreateBucket(bktMigAdmins)
		if e != nil {
			return e
		}
		s := tx.Bucket(store.BucketUsers)
		if s == nil {
			return nil
		}
		c := s.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var u store.User
			if json.Unmarshal(v, &u) != nil {
				continue
			}
			if u.IsLocal {
				continue
			}
			total++
			newName, ok := plan.mapUsername(u.Username)
			if !ok || newName == u.Username {
				continue
			}
			nu := u
			nu.Username = newName
			nb, err := json.Marshal(nu)
			if err != nil {
				return err
			}
			if e := d.Put(append([]byte(nil), k...), nb); e != nil {
				return e
			}
			changed++
		}
		return nil
	})
	return
}

// avatarChangeCount counts avatar files whose base id maps to a different target
// id — i.e. the number of files that Apply will rename. No files are touched.
func (app *Server) avatarChangeCount(plan *migPlan) (total, changed int) {
	entries, err := os.ReadDir(app.cfg.DataPath("avatarcache"))
	if err != nil {
		return 0, 0
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || len(name) <= 4 || !strings.EqualFold(name[len(name)-4:], ".jpg") {
			continue
		}
		total++
		base := name[:len(name)-4]
		if nb, ok := plan.mapBare(base); ok && !strings.EqualFold(nb, base) {
			changed++
		}
	}
	return
}

// runIdentifierStage runs the full conversion into the temporary staging buckets
// without altering any live data. It records a per-area change breakdown that the
// review step displays before the admin commits with Apply.
func (app *Server) runIdentifierStage(target string) {
	prog := &app.migrateProg
	app.purgeStaging()
	plan, err := app.buildMigPlan(target)
	if err != nil {
		prog.Finish("", "building migration plan: "+err.Error())
		return
	}
	if plan.totalDir == 0 {
		prog.Finish("", "the directory is empty — run a directory sync before migrating")
		return
	}
	prog.Logf("Staging conversion: %s → %s", plan.current, target)
	prog.Logf("%d directory user(s): %d to convert, %d already in target form, %d conflict(s) skipped.",
		plan.totalDir, plan.mappable, plan.already, len(plan.conflicts))

	var areas []migStageArea
	add := func(key, label string, total, changed int) {
		areas = append(areas, migStageArea{Key: key, Label: label, Total: total, Changed: changed})
	}

	prog.SetStage("Staging audit log")
	t, c, err := app.stageBucket(bktMigAudit, store.BucketAudit, func(v []byte) ([]byte, bool) {
		var e store.AuditEntry
		if json.Unmarshal(v, &e) != nil {
			return nil, false
		}
		nn, ok := plan.mapUsername(e.User)
		if !ok || nn == e.User {
			return nil, false
		}
		e.User = nn
		nv, err := json.Marshal(e)
		if err != nil {
			return nil, false
		}
		return nv, true
	})
	if err != nil {
		prog.Finish("", "staging audit log: "+err.Error())
		return
	}
	add("audit", "Audit log", t, c)
	prog.Logf("Audit log: %d of %d record(s) staged.", c, t)

	prog.SetStage("Staging changelog")
	t, c, err = app.stageBucket(bktMigChangelog, store.BucketChangelog, func(v []byte) ([]byte, bool) {
		var e store.ChangelogEntry
		if json.Unmarshal(v, &e) != nil {
			return nil, false
		}
		nb, ok := plan.mapBare(e.Avatar)
		if !ok || nb == e.Avatar {
			return nil, false
		}
		e.Avatar = nb
		nv, err := json.Marshal(e)
		if err != nil {
			return nil, false
		}
		return nv, true
	})
	if err != nil {
		prog.Finish("", "staging changelog: "+err.Error())
		return
	}
	add("changelog", "Changelog", t, c)
	prog.Logf("Changelog: %d of %d record(s) staged.", c, t)

	prog.SetStage("Staging bookings")
	t, c, err = app.stageBucket(bktMigBookings, store.BucketBookings, func(v []byte) ([]byte, bool) {
		var b store.Booking
		if json.Unmarshal(v, &b) != nil {
			return nil, false
		}
		nb, ok := plan.mapBare(b.User)
		if !ok || nb == b.User {
			return nil, false
		}
		b.User = nb
		nv, err := json.Marshal(b)
		if err != nil {
			return nil, false
		}
		return nv, true
	})
	if err != nil {
		prog.Finish("", "staging bookings: "+err.Error())
		return
	}
	add("bookings", "Bookings", t, c)
	prog.Logf("Bookings: %d of %d record(s) staged.", c, t)

	prog.SetStage("Staging desks")
	t, c, err = app.stageBucket(bktMigDesks, store.BucketDesks, func(v []byte) ([]byte, bool) {
		var d store.Desk
		if json.Unmarshal(v, &d) != nil {
			return nil, false
		}
		if strings.TrimSpace(d.Avatar) == "" {
			return nil, false
		}
		nb, ok := plan.mapBare(d.Avatar)
		if !ok || nb == d.Avatar {
			return nil, false
		}
		d.Avatar = nb
		nv, err := json.Marshal(d)
		if err != nil {
			return nil, false
		}
		return nv, true
	})
	if err != nil {
		prog.Finish("", "staging desks: "+err.Error())
		return
	}
	add("desks", "Desks", t, c)
	prog.Logf("Desks: %d of %d record(s) staged.", c, t)

	prog.SetStage("Staging map admins")
	t, c, err = app.stageAdmins(plan)
	if err != nil {
		prog.Finish("", "staging map admins: "+err.Error())
		return
	}
	add("admins", "Map admins", t, c)
	prog.Logf("Map admins: %d of %d record(s) staged.", c, t)

	prog.SetStage("Scanning avatars")
	at, ac := app.avatarChangeCount(plan)
	add("avatars", "Avatar images", at, ac)
	prog.Logf("Avatars: %d of %d file(s) will be renamed.", ac, at)

	// Local accounts: count those still missing a synthetic mail.
	users, _ := app.db.ListUsers()
	localTotal, localChanged := 0, 0
	for _, u := range users {
		if u.IsLocal {
			localTotal++
			if strings.TrimSpace(u.Mail) == "" {
				localChanged++
			}
		}
	}
	add("localmail", "Local account mail", localTotal, localChanged)

	info := migStageInfo{
		Target:    target,
		Current:   plan.current,
		CreatedAt: time.Now().Unix(),
		Conflicts: len(plan.conflicts),
		Areas:     areas,
	}
	if err := app.writeStageInfo(info); err != nil {
		prog.Finish("", "writing staging metadata: "+err.Error())
		return
	}

	totalChanges := 0
	for _, a := range areas {
		totalChanges += a.Changed
	}
	prog.Finish(fmt.Sprintf("Staged %d change(s). Review, then apply to switch.", totalChanges), "")
}

// applyStagedBucket copies every staged row into the live bucket under the same
// key (overwrite). Returns the number of rows applied.
func (app *Server) applyStagedBucket(live, stage []byte) (int, error) {
	n := 0
	err := app.db.Bolt().Update(func(tx *bolt.Tx) error {
		s := tx.Bucket(stage)
		if s == nil {
			return nil
		}
		l := tx.Bucket(live)
		if l == nil {
			return nil
		}
		c := s.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if e := l.Put(append([]byte(nil), k...), append([]byte(nil), v...)); e != nil {
				return e
			}
			n++
		}
		return nil
	})
	return n, err
}

// applyStagedAdmins re-keys the staged map-admin records: the staged key is the
// old username and the value holds the new user (with the new username).
func (app *Server) applyStagedAdmins() (int, error) {
	n := 0
	err := app.db.Bolt().Update(func(tx *bolt.Tx) error {
		s := tx.Bucket(bktMigAdmins)
		if s == nil {
			return nil
		}
		l := tx.Bucket(store.BucketUsers)
		if l == nil {
			return nil
		}
		c := s.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var u store.User
			if json.Unmarshal(v, &u) != nil {
				continue
			}
			if e := l.Put([]byte(u.Username), append([]byte(nil), v...)); e != nil {
				return e
			}
			if string(k) != u.Username {
				if e := l.Delete(append([]byte(nil), k...)); e != nil {
					return e
				}
			}
			n++
		}
		return nil
	})
	return n, err
}

// runIdentifierApply commits a previously staged migration: it swaps the staged
// rows into the live buckets, performs the avatar file renames, backfills local
// mail, flips the identifier setting, rebuilds the derived caches and finally
// drops the staging buckets. Nothing on disk or in the live buckets is touched
// until this point.
func (app *Server) runIdentifierApply(target string) {
	prog := &app.migrateProg
	info, ok := app.readStageInfo()
	if !ok || info.Target != target {
		prog.Finish("", "no staged migration found — please run the create step again")
		return
	}
	if info.stageExpired() {
		app.purgeStaging()
		prog.Finish("", "the staged migration expired (older than 1 hour) — please run the create step again")
		return
	}
	plan, err := app.buildMigPlan(target)
	if err != nil {
		prog.Finish("", "building migration plan: "+err.Error())
		return
	}
	prog.Logf("Applying staged migration: %s → %s", info.Current, target)

	prog.SetStage("Applying audit log")
	if n, err := app.applyStagedBucket(store.BucketAudit, bktMigAudit); err != nil {
		prog.Finish("", "applying audit log: "+err.Error())
		return
	} else {
		prog.Logf("Audit log: %d record(s) updated.", n)
	}

	prog.SetStage("Applying changelog")
	if n, err := app.applyStagedBucket(store.BucketChangelog, bktMigChangelog); err != nil {
		prog.Finish("", "applying changelog: "+err.Error())
		return
	} else {
		prog.Logf("Changelog: %d record(s) updated.", n)
	}

	prog.SetStage("Applying bookings")
	if n, err := app.applyStagedBucket(store.BucketBookings, bktMigBookings); err != nil {
		prog.Finish("", "applying bookings: "+err.Error())
		return
	} else {
		prog.Logf("Bookings: %d record(s) updated.", n)
	}

	prog.SetStage("Applying desks")
	if n, err := app.applyStagedBucket(store.BucketDesks, bktMigDesks); err != nil {
		prog.Finish("", "applying desks: "+err.Error())
		return
	} else {
		prog.Logf("Desks: %d record(s) updated.", n)
	}

	prog.SetStage("Applying map admins")
	if n, err := app.applyStagedAdmins(); err != nil {
		prog.Finish("", "applying map admins: "+err.Error())
		return
	} else {
		prog.Logf("Map admins: %d re-keyed.", n)
	}

	// Keep already-authenticated admins signed in: their map-admin record was
	// just re-keyed, so any active session (notably SAML users, who cannot fall
	// back to the local admin password) must follow the rename — otherwise
	// permLevel() can no longer find their user and they are 403'd out, which
	// stalls the progress poll even though the migration finishes fine.
	app.sessions.Remap(plan.mapUsername)

	// Avatar files: rename <old>.jpg -> <new>.jpg (disk is only touched now).
	entries, _ := os.ReadDir(app.cfg.DataPath("avatarcache"))
	prog.BeginPhase(len(entries), "Renaming avatars")
	avatars := 0
	for _, e := range entries {
		prog.Step("")
		name := e.Name()
		if e.IsDir() || len(name) <= 4 || !strings.EqualFold(name[len(name)-4:], ".jpg") {
			continue
		}
		base := name[:len(name)-4]
		nb, ok := plan.mapBare(base)
		if !ok || strings.EqualFold(nb, base) {
			continue
		}
		src := app.cfg.DataPath("avatarcache", name)
		dst := app.cfg.DataPath("avatarcache", nb+".jpg")
		if err := copyFile(src, dst); err != nil {
			prog.Logf("   ✗ avatar %s: %v", base, err)
			continue
		}
		_ = os.Remove(src)
		avatars++
	}
	prog.Logf("Avatars: %d renamed.", avatars)

	// Local accounts: backfill synthetic mail so they are never missing one.
	prog.SetStage("Backfilling local accounts")
	users, _ := app.db.ListUsers()
	localMail := 0
	for _, u := range users {
		if u.IsLocal && strings.TrimSpace(u.Mail) == "" {
			u.Mail = u.Username + "@cmaps.local"
			_ = app.db.PutUser(u)
			localMail++
		}
	}
	if localMail > 0 {
		prog.Logf("Local accounts: %d synthetic mail address(es) added.", localMail)
	}

	prog.SetStage("Applying setting")
	if err := app.db.SetSetting("identifier", target); err != nil {
		prog.Finish("", "converted data but failed to save the identifier setting: "+err.Error())
		return
	}
	_ = app.db.AuditLog("config", "System", "Employee identifier switched to "+target)
	prog.Logf("Identifier setting is now: %s", target)

	prog.SetStage("Re-syncing directory")
	if app.dir.AnyLdapSourceEnabled() {
		if _, err := app.dir.RunADSync(); err != nil {
			prog.Logf("   ⚠ LDAP re-sync: %v", err)
		} else {
			prog.Logf("LDAP directory re-synced.")
		}
	}
	if app.dir.EntraHasEnabledSource() {
		if _, err := app.dir.RunEntraSync(); err != nil {
			prog.Logf("   ⚠ EntraID re-sync: %v", err)
		} else {
			prog.Logf("EntraID directory re-synced.")
		}
	}

	app.purgeStaging()
	prog.Finish(fmt.Sprintf("Migrated to %s. %d avatar(s) renamed, %d conflict(s) skipped.", target, avatars, info.Conflicts), "")
}

// ── REST endpoints ──────────────────────────────────────────

// validMigTarget reports whether target is a valid identifier mode different
// from the current one.
func (app *Server) validMigTarget(target string) bool {
	if target != "mail" && target != "samaccountname" {
		return false
	}
	return target != app.identifierMode()
}

// handleRestIdentifierAnalyze previews a migration: the old->new mapping stats,
// the conflicts that would be skipped, and per-area record counts so the admin
// knows what to expect before running. It makes no changes.
func (app *Server) handleRestIdentifierAnalyze(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "adminpanel") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	target := r.FormValue("target")
	if !app.validMigTarget(target) {
		http.Error(w, "invalid target identifier", http.StatusBadRequest)
		return
	}
	plan, err := app.buildMigPlan(target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{
		"current":   plan.current,
		"target":    plan.target,
		"totalDir":  plan.totalDir,
		"mappable":  plan.mappable,
		"already":   plan.already,
		"conflicts": plan.conflicts,
		"counts": map[string]int{
			"avatars":   app.avatarCount(),
			"mapAdmins": app.bucketCount(store.BucketUsers),
			"bookings":  app.bucketCount(store.BucketBookings),
			"changelog": app.bucketCount(store.BucketChangelog),
			"audit":     app.bucketCount(store.BucketAudit),
			"desks":     app.deskAvatarCount(),
		},
	})
}

// handleRestIdentifierCreate starts the "create new data" phase: it converts
// every record into temporary staging buckets and records a per-area change
// breakdown, without touching any live data or avatar files.
func (app *Server) handleRestIdentifierCreate(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "adminpanel") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	target := r.FormValue("target")
	if !app.validMigTarget(target) {
		http.Error(w, "invalid target identifier", http.StatusBadRequest)
		return
	}
	if !app.migrateProg.Start(0, "Staging…") {
		writeJSON(w, map[string]interface{}{"started": false, "running": true})
		return
	}
	_ = app.db.AuditLog("config", sess.Username, "Staged identifier migration data for "+target)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				app.migrateProg.Finish("", fmt.Sprintf("staging crashed: %v", rec))
			}
		}()
		app.runIdentifierStage(target)
	}()
	writeJSON(w, map[string]interface{}{"started": true})
}

// handleRestIdentifierStageResult returns the staged per-area change breakdown
// for the review step. Expired staging is purged and reported as such.
func (app *Server) handleRestIdentifierStageResult(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "adminpanel") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	info, found := app.readStageInfo()
	if !found {
		writeJSON(w, map[string]interface{}{"status": "none"})
		return
	}
	if info.stageExpired() {
		app.purgeStaging()
		writeJSON(w, map[string]interface{}{"status": "expired"})
		return
	}
	remaining := int(migStageTTL.Seconds()) - int(time.Since(time.Unix(info.CreatedAt, 0)).Seconds())
	if remaining < 0 {
		remaining = 0
	}
	writeJSON(w, map[string]interface{}{
		"status":       "ok",
		"target":       info.Target,
		"current":      info.Current,
		"conflicts":    info.Conflicts,
		"areas":        info.Areas,
		"expiresInSec": remaining,
	})
}

// handleRestIdentifierCancel discards a staged migration, leaving the live data
// untouched.
func (app *Server) handleRestIdentifierCancel(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "adminpanel") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	app.purgeStaging()
	writeJSON(w, map[string]interface{}{"status": "ok"})
}

// handleRestIdentifierApply starts the final "apply" phase: it converts the live
// records, removes the superseded old avatars and activates the new identifier.
func (app *Server) handleRestIdentifierApply(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "adminpanel") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	target := r.FormValue("target")
	if !app.validMigTarget(target) {
		http.Error(w, "invalid target identifier", http.StatusBadRequest)
		return
	}
	if !app.migrateProg.Start(0, "Applying…") {
		writeJSON(w, map[string]interface{}{"started": false, "running": true})
		return
	}
	_ = app.db.AuditLog("config", sess.Username, "Applied identifier migration to "+target)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				app.migrateProg.Finish("", fmt.Sprintf("migration crashed: %v", rec))
			}
		}()
		app.runIdentifierApply(target)
	}()
	writeJSON(w, map[string]interface{}{"started": true})
}

// handleRestIdentifierProgress returns the current migration progress snapshot.
func (app *Server) handleRestIdentifierProgress(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "adminpanel") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, app.migrateProg.Snapshot())
}
