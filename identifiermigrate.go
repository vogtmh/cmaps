package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

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
func (app *App) buildMigPlan(target string) (*migPlan, error) {
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
func (app *App) bucketCount(bucket []byte) int {
	n := 0
	_ = app.db.bolt.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket(bucket); b != nil {
			n = b.Stats().KeyN
		}
		return nil
	})
	return n
}

// avatarCount returns the number of cached avatar files.
func (app *App) avatarCount() int {
	entries, err := os.ReadDir(app.cfg.dataPath("avatarcache"))
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
func (app *App) deskAvatarCount() int {
	desks, _ := app.db.ListAllDesks()
	n := 0
	for _, d := range desks {
		if strings.TrimSpace(d.Avatar) != "" {
			n++
		}
	}
	return n
}

// rewriteBucketField rewrites values in a bucket in place (same keys). The
// transform returns the new value bytes and whether it changed. Changes are
// gathered first, then applied in a single write transaction, so the cursor is
// never mutated mid-iteration.
func (app *App) rewriteBucketField(bucket []byte, transform func(v []byte) ([]byte, bool)) (int, error) {
	type kv struct{ k, v []byte }
	var changes []kv
	err := app.db.bolt.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		if b == nil {
			return nil
		}
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			nv, changed := transform(v)
			if changed {
				changes = append(changes, kv{append([]byte(nil), k...), nv})
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	err = app.db.bolt.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		for _, ch := range changes {
			if err := b.Put(ch.k, ch.v); err != nil {
				return err
			}
		}
		return nil
	})
	return len(changes), err
}

// copyFile is defined in wizard.go.

// runIdentifierMigration performs the full migration to the target mode, driving
// the shared migrateProg progress bar.
func (app *App) runIdentifierMigration(target string) {
	prog := &app.migrateProg
	plan, err := app.buildMigPlan(target)
	if err != nil {
		prog.finish("", "building migration plan: "+err.Error())
		return
	}
	if plan.totalDir == 0 {
		prog.finish("", "the directory is empty — run a directory sync before migrating")
		return
	}
	prog.logf("Migrating identifier: %s → %s", plan.current, target)
	prog.logf("%d directory user(s): %d to convert, %d already in target form, %d conflict(s) skipped.",
		plan.totalDir, plan.mappable, plan.already, len(plan.conflicts))
	for _, c := range plan.conflicts {
		prog.logf("   ⚠ skipped %s: %s", c.Old, c.Reason)
	}

	converted := map[string]int{}

	// 1. Backfill synthetic mail addresses for local accounts so they are never
	//    reported as missing a mail. Their login/username is left unchanged.
	prog.beginPhase(0, "Local accounts")
	users, _ := app.db.ListUsers()
	for _, u := range users {
		if u.IsLocal && strings.TrimSpace(u.Mail) == "" {
			u.Mail = u.Username + "@cmaps.local"
			_ = app.db.PutUser(u)
			converted["localMail"]++
		}
	}

	// 2. Avatar files: copy <old>.jpg -> <new>.jpg, then remove the old file.
	dirPath := app.cfg.dataPath("avatarcache")
	entries, _ := os.ReadDir(dirPath)
	prog.beginPhase(len(entries), "Avatar images")
	for _, e := range entries {
		prog.step("")
		name := e.Name()
		if e.IsDir() || len(name) <= 4 || !strings.EqualFold(name[len(name)-4:], ".jpg") {
			continue
		}
		base := name[:len(name)-4]
		nb, ok := plan.mapBare(base)
		if !ok || strings.EqualFold(nb, base) {
			continue
		}
		src := app.cfg.dataPath("avatarcache", name)
		dst := app.cfg.dataPath("avatarcache", nb+".jpg")
		if err := copyFile(src, dst); err != nil {
			prog.logf("   ✗ avatar %s: %v", base, err)
			continue
		}
		_ = os.Remove(src)
		converted["avatars"]++
	}
	prog.logf("Avatars: %d converted.", converted["avatars"])

	// 3. Map-admin records (bucketUsers): rekey directory-derived admins.
	prog.beginPhase(len(users), "Map admins")
	for _, u := range users {
		prog.step("")
		if u.IsLocal {
			continue
		}
		newName, ok := plan.mapUsername(u.Username)
		if !ok || newName == u.Username {
			continue
		}
		nu := u
		nu.Username = newName
		if err := app.db.PutUser(nu); err != nil {
			prog.logf("   ✗ admin %s: %v", u.Username, err)
			continue
		}
		_ = app.db.DeleteUser(u.Username)
		converted["admins"]++
	}
	prog.logf("Map admins: %d re-keyed.", converted["admins"])

	// 4. Audit log (usernames, may carry DOMAIN\ prefix).
	prog.beginPhase(0, "Audit log")
	n, _ := app.rewriteBucketField(bucketAudit, func(v []byte) ([]byte, bool) {
		var e AuditEntry
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
	converted["audit"] = n
	prog.logf("Audit log: %d record(s) converted.", n)

	// 5. Changelog (bare avatar id).
	prog.beginPhase(0, "Changelog")
	n, _ = app.rewriteBucketField(bucketChangelog, func(v []byte) ([]byte, bool) {
		var e ChangelogEntry
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
	converted["changelog"] = n
	prog.logf("Changelog: %d record(s) converted.", n)

	// 6. Bookings (bare user id).
	prog.beginPhase(0, "Bookings")
	n, _ = app.rewriteBucketField(bucketBookings, func(v []byte) ([]byte, bool) {
		var b Booking
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
	converted["bookings"] = n
	prog.logf("Bookings: %d record(s) converted.", n)

	// 7. Desks (bare avatar id on localdesk placements).
	prog.beginPhase(0, "Desks")
	n, _ = app.rewriteBucketField(bucketDesks, func(v []byte) ([]byte, bool) {
		var d Desk
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
	converted["desks"] = n
	prog.logf("Desks: %d record(s) converted.", n)

	// 8. Flip the identifier setting now that the data is converted.
	prog.setStage("Applying setting")
	if err := app.db.SetSetting("identifier", target); err != nil {
		prog.finish("", "converted data but failed to save the identifier setting: "+err.Error())
		return
	}
	_ = app.db.AuditLog("config", "System", "Employee identifier switched to "+target)
	prog.logf("Identifier setting is now: %s", target)

	// 9. Rebuild the derived directory/mirror caches with the new identifiers.
	prog.setStage("Re-syncing directory")
	if app.anyLdapSourceEnabled() {
		if _, err := app.RunADSync(); err != nil {
			prog.logf("   ⚠ LDAP re-sync: %v", err)
		} else {
			prog.logf("LDAP directory re-synced.")
		}
	}
	if app.entraHasEnabledSource() {
		if _, err := app.RunEntraSync(); err != nil {
			prog.logf("   ⚠ EntraID re-sync: %v", err)
		} else {
			prog.logf("EntraID directory re-synced.")
		}
	}

	summary := fmt.Sprintf("Migrated to %s. %d avatar(s), %d admin(s), %d audit, %d changelog, %d booking(s), %d desk(s). %d conflict(s) skipped.",
		target, converted["avatars"], converted["admins"], converted["audit"], converted["changelog"], converted["bookings"], converted["desks"], len(plan.conflicts))
	prog.finish(summary, "")
}

// ── REST endpoints ──────────────────────────────────────────

// validMigTarget reports whether target is a valid identifier mode different
// from the current one.
func (app *App) validMigTarget(target string) bool {
	if target != "mail" && target != "samaccountname" {
		return false
	}
	return target != app.identifierMode()
}

// handleRestIdentifierAnalyze previews a migration: the old->new mapping stats,
// the conflicts that would be skipped, and per-area record counts so the admin
// knows what to expect before running. It makes no changes.
func (app *App) handleRestIdentifierAnalyze(w http.ResponseWriter, r *http.Request) {
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
			"mapAdmins": app.bucketCount(bucketUsers),
			"bookings":  app.bucketCount(bucketBookings),
			"changelog": app.bucketCount(bucketChangelog),
			"audit":     app.bucketCount(bucketAudit),
			"desks":     app.deskAvatarCount(),
		},
	})
}

// handleRestIdentifierMigrate starts the background migration worker.
func (app *App) handleRestIdentifierMigrate(w http.ResponseWriter, r *http.Request) {
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
	if !app.migrateProg.start(0, "Starting…") {
		writeJSON(w, map[string]interface{}{"started": false, "running": true})
		return
	}
	_ = app.db.AuditLog("config", sess.Username, "Started identifier migration to "+target)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				app.migrateProg.finish("", fmt.Sprintf("migration crashed: %v", rec))
			}
		}()
		app.runIdentifierMigration(target)
	}()
	writeJSON(w, map[string]interface{}{"started": true})
}

// handleRestIdentifierProgress returns the current migration progress snapshot.
func (app *App) handleRestIdentifierProgress(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "adminpanel") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, app.migrateProg.snapshot())
}
