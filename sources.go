package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// This file implements the unified, priority-ordered directory-source model.
//
// Historically desk occupancy was decided by hardcoded rules: the LDAP mirror
// was always the base (addesk matches office==desknumber), Robin sat on top via
// the robinDeskMode overlay (all/unused/allclear) and EntraID was never shown on
// desks at all. That is replaced here by ONE ordered list of every configured
// source instance (each LDAP config, each EntraID config and Robin). The list
// order is the priority and each entry carries two per-source toggles. The
// decision is made entirely at RENDER time from the existing per-source caches,
// so reordering never requires a resync.

// UnifiedSource is one resolved row of the priority list, combining the stored
// rule with the live source's metadata. Used by both the admin UI and the
// render-time assignment engine.
type UnifiedSource struct {
	Ref            string
	Type           string // "ldap" | "entra" | "robin"
	ID             int    // 0 for Robin
	Description    string
	Disabled       bool // sync disabled (LDAP/Entra Disabled field, or Robin switched off)
	Assign         bool // participates in desk assignment
	KeepDuplicates bool
	Priority       int // 1-based, for display only
	// PopulatedSeats is how many desks this source effectively fills across all
	// published maps under the CURRENT priority/dedup/assign settings. Filled in
	// only for the admin display (see sourceSeatCounts); the engine ignores it.
	PopulatedSeats int
}

// The stored priority list lives in the meta bucket under
// "directorySourceOrder"; see store.DB.GetSourceOrder / SetSourceOrder.

// currentRules returns the fully reconciled priority list as SourceRules (every
// live source present, in current priority order, with its flags). Persisting
// this keeps the stored order authoritative even before the admin has ever
// reordered anything.
func (app *App) currentRules() []SourceRule {
	us := app.listUnifiedSources()
	rules := make([]SourceRule, 0, len(us))
	for _, s := range us {
		rules = append(rules, SourceRule{Ref: s.Ref, KeepDuplicates: s.KeepDuplicates, NoAssign: !s.Assign})
	}
	return rules
}

// moveSource shifts a source one step up (dir=-1, higher priority) or down
// (dir=+1) in the priority list. Returns false if the source is unknown or
// already at the edge.
func (app *App) moveSource(ref string, dir int) bool {
	rules := app.currentRules()
	idx := -1
	for i, r := range rules {
		if r.Ref == ref {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	j := idx + dir
	if j < 0 || j >= len(rules) {
		return false
	}
	rules[idx], rules[j] = rules[j], rules[idx]
	_ = app.db.SetSourceOrder(rules)
	return true
}

// setSourceFlags updates a source's per-source toggles (nil = leave unchanged)
// and persists the order.
func (app *App) setSourceFlags(ref string, assign, keepDup *bool) {
	rules := app.currentRules()
	for i := range rules {
		if rules[i].Ref == ref {
			if assign != nil {
				rules[i].NoAssign = !*assign
			}
			if keepDup != nil {
				rules[i].KeepDuplicates = *keepDup
			}
			break
		}
	}
	_ = app.db.SetSourceOrder(rules)
}

// robinConfigured reports whether Robin is set up enough to be a source (a token
// is present). Robin is a single instance, unlike the multi-config LDAP/Entra.
func (app *App) robinConfigured() bool {
	return strings.TrimSpace(app.db.GetRobinSetting("robintoken")) != ""
}

// listUnifiedSources assembles the priority list: it reconciles the stored order
// with the sources that currently exist (appending newly added ones at the end
// with default flags and dropping deleted ones), then stamps 1-based priorities.
func (app *App) listUnifiedSources() []UnifiedSource {
	type liveInfo struct {
		typ      string
		id       int
		desc     string
		disabled bool
	}
	live := map[string]liveInfo{}
	var liveRefs []string // stable natural order (ldap, then entra, then robin)

	ldapSrcs, _ := app.db.ListLdapSources()
	for _, s := range ldapSrcs {
		ref := fmt.Sprintf("ldap:%d", s.ID)
		live[ref] = liveInfo{"ldap", s.ID, s.Description, s.Disabled}
		liveRefs = append(liveRefs, ref)
	}
	entraSrcs, _ := app.db.ListEntraSources()
	for _, s := range entraSrcs {
		ref := fmt.Sprintf("entra:%d", s.ID)
		live[ref] = liveInfo{"entra", s.ID, s.Description, s.Disabled}
		liveRefs = append(liveRefs, ref)
	}
	if app.robinConfigured() {
		live["robin"] = liveInfo{"robin", 0, "Robin", !app.robinEnabled()}
		liveRefs = append(liveRefs, "robin")
	}

	rules := app.db.GetSourceOrder()
	if len(rules) == 0 {
		rules = app.defaultSourceOrder(liveRefs)
	}

	seen := map[string]bool{}
	var out []UnifiedSource
	add := func(r SourceRule) {
		info, ok := live[r.Ref]
		if !ok || seen[r.Ref] {
			return
		}
		seen[r.Ref] = true
		out = append(out, UnifiedSource{
			Ref:            r.Ref,
			Type:           info.typ,
			ID:             info.id,
			Description:    info.desc,
			Disabled:       info.disabled,
			Assign:         !r.NoAssign,
			KeepDuplicates: r.KeepDuplicates,
		})
	}
	for _, r := range rules {
		add(r)
	}
	// Any live source not present in the stored order is a newly added one:
	// append it at the lowest priority with default flags.
	for _, ref := range liveRefs {
		if !seen[ref] {
			add(SourceRule{Ref: ref})
		}
	}
	for i := range out {
		out[i].Priority = i + 1
	}
	return out
}

// defaultSourceOrder seeds a priority list for installs that have never set one,
// preserving the previous behavior as closely as possible: the old robinDeskMode
// of "all"/"allclear" meant Robin overrode LDAP, so Robin goes first in that
// case; otherwise the directory sources lead and Robin trails.
func (app *App) defaultSourceOrder(liveRefs []string) []SourceRule {
	mode := app.db.GetRobinSetting("robinDeskMode")
	robinFirst := mode == "all" || mode == "allclear"
	var rules []SourceRule
	if robinFirst {
		for _, ref := range liveRefs {
			if ref == "robin" {
				rules = append(rules, SourceRule{Ref: ref})
			}
		}
	}
	for _, ref := range liveRefs {
		if ref == "robin" {
			continue
		}
		rules = append(rules, SourceRule{Ref: ref})
	}
	if !robinFirst {
		for _, ref := range liveRefs {
			if ref == "robin" {
				rules = append(rules, SourceRule{Ref: ref})
			}
		}
	}
	return rules
}

// deskOccupant is one person a source places at a desk, normalized across the
// LDAP/EntraID mirror shape and the Robin occupancy shape.
type deskOccupant struct {
	sourceType string // "ldap" | "entra" | "robin"
	sourceRef  string // owning source ref ("ldap:<id>"/"entra:<id>"/"robin")
	desknumber string // canonical desknumber (as stored on the desk)
	userid     string
	mail       string
	aliases    []string
	name       string // full display name (Robin only; LDAP/Entra use fname/lname)
	fname      string
	lname      string
	phone      string
	title      string
	mobile     string
	dept       string
	hasAvatar  bool
}

// samePerson reports whether two occupants are the same individual, comparing
// the userid first and then the primary mail against the other's mail or any of
// its aliases (case-insensitively). Mirrors the old sameRobinPerson logic but
// works between any two sources.
func samePerson(a, b deskOccupant) bool {
	if id := strings.ToLower(strings.TrimSpace(a.userid)); id != "" {
		if id == strings.ToLower(strings.TrimSpace(b.userid)) {
			return true
		}
	}
	am := strings.ToLower(strings.TrimSpace(a.mail))
	bm := strings.ToLower(strings.TrimSpace(b.mail))
	if am != "" {
		if am == bm {
			return true
		}
		for _, al := range b.aliases {
			if am == strings.ToLower(strings.TrimSpace(al)) {
				return true
			}
		}
	}
	if bm != "" {
		for _, al := range a.aliases {
			if bm == strings.ToLower(strings.TrimSpace(al)) {
				return true
			}
		}
	}
	return false
}

// containsPerson reports whether the placed slice already holds this person.
func containsPerson(placed []deskOccupant, o deskOccupant) bool {
	for _, p := range placed {
		if samePerson(p, o) {
			return true
		}
	}
	return false
}

// sourceOccupancy returns everyone a single source would place on the given map,
// restricted to desks that actually exist on that map. deskByNum maps a
// lowercased desknumber to its desk. Directory sources (LDAP/EntraID) only fill
// AD-mirrored "addesk" desks, preserving the historical addesk semantics; Robin
// live occupancy may take over any desk (as the old overlay did).
func (app *App) sourceOccupancy(src UnifiedSource, mapName string, deskByNum map[string]Desk, avatarByUser map[string]bool) []deskOccupant {
	var out []deskOccupant
	switch src.Type {
	case "ldap", "entra":
		users, _ := app.db.GetSourceMirror(src.Type, src.ID)
		for _, u := range users {
			office := strings.TrimSpace(u.Office)
			if office == "" {
				continue
			}
			d, ok := deskByNum[strings.ToLower(office)]
			if !ok || d.Desktype != "addesk" {
				continue
			}
			out = append(out, deskOccupant{
				sourceType: src.Type,
				sourceRef:  src.Ref,
				desknumber: d.Desknumber,
				userid:     u.Userid,
				mail:       u.Mail,
				aliases:    u.Aliases,
				fname:      u.Givenname,
				lname:      u.Surname,
				phone:      u.Telephonenumber,
				title:      u.Description,
				mobile:     u.Mobile,
				dept:       u.Department,
				hasAvatar:  avatarByUser[strings.ToLower(strings.TrimSpace(u.Userid))],
			})
		}
	case "robin":
		if !app.robinEnabled() {
			return nil
		}
		sts, _ := app.db.ListRobinDeskStatus(mapName)
		for _, s := range sts {
			d, ok := deskByNum[strings.ToLower(strings.TrimSpace(s.Desknumber))]
			if !ok {
				continue
			}
			out = append(out, deskOccupant{
				sourceType: "robin",
				sourceRef:  src.Ref,
				desknumber: d.Desknumber,
				userid:     s.Userid,
				mail:       s.Mail,
				name:       s.Name,
				phone:      s.Phone,
				title:      s.Title,
				mobile:     s.Mobile,
				hasAvatar:  avatarByUser[strings.ToLower(strings.TrimSpace(s.Userid))],
			})
		}
	}
	return out
}

// assignMapOccupancy runs the priority-ordered assignment for one map and
// returns, per canonical desknumber, the occupants to display (up to 4 for a
// shared desk). Higher-priority sources win each desk outright; a lower-priority
// source may still fill an empty desk. A person already placed on this map by a
// higher-priority source is only placed again when this source has
// KeepDuplicates enabled. Because this runs per map, deduplication is naturally
// scoped to a single map (a person with seats on two different maps keeps both).
func (app *App) assignMapOccupancy(mapName string, desks []Desk, avatarByUser map[string]bool) map[string][]deskOccupant {
	deskByNum := map[string]Desk{}
	for _, d := range desks {
		dn := strings.TrimSpace(d.Desknumber)
		if dn == "" {
			continue
		}
		deskByNum[strings.ToLower(dn)] = d
	}

	result := map[string][]deskOccupant{} // key = lowercased desknumber
	deskOwner := map[string]string{}      // lowercased desknumber -> owning source ref
	var placed []deskOccupant             // people placed by higher-priority sources

	for _, src := range app.listUnifiedSources() {
		if src.Disabled || !src.Assign {
			continue
		}
		occ := app.sourceOccupancy(src, mapName, deskByNum, avatarByUser)
		var placedThis []deskOccupant
		for _, o := range occ {
			key := strings.ToLower(o.desknumber)
			// A desk owned by a different (higher-priority) source is never
			// overwritten; the same source may add more occupants (shared desk).
			if owner, ok := deskOwner[key]; ok && owner != src.Ref {
				continue
			}
			// Drop a duplicate of someone a higher-priority source already placed
			// on this map, unless this source is allowed to keep duplicates.
			if !src.KeepDuplicates && containsPerson(placed, o) {
				continue
			}
			if len(result[key]) >= 4 {
				continue
			}
			deskOwner[key] = src.Ref
			result[key] = append(result[key], o)
			placedThis = append(placedThis, o)
		}
		// Only expose this source's placements to later sources after finishing
		// it, so two seats of the SAME source (office="A|B") are never self-deduped.
		placed = append(placed, placedThis...)
	}
	return result
}

// sourceSeatCounts returns, per source ref, how many desks that source
// effectively fills across all published maps under the current
// priority/dedup/assign settings. It runs the same assignment engine used to
// render the maps, so the numbers match exactly what appears on screen. Used by
// the admin priority list; it is recomputed on every tab render (including after
// a move/toggle), so the counts always reflect the latest order.
func (app *App) sourceSeatCounts() map[string]int {
	counts := map[string]int{}
	avatarIdx := app.buildAvatarIndex()
	maps, _ := app.db.ListMaps()
	for _, m := range maps {
		if m.Published == "no" || m.Mapname == "overview" {
			continue
		}
		desks, _ := app.db.ListDesks(m.Mapname)
		occupancy := app.assignMapOccupancy(m.Mapname, desks, avatarIdx)
		for _, occ := range occupancy {
			for _, o := range occ {
				counts[o.sourceRef]++
			}
		}
	}
	return counts
}

// sourceSeat is one effectively assigned seat, for the "seats filled" popup.
type sourceSeat struct {
	Name string `json:"name"`
	Map  string `json:"map"`
	Desk string `json:"desk"`
}

// sourceSeatDetails returns, per source ref, the people that source effectively
// seats across all published maps (same engine used to render the maps), sorted
// by name. Powers the popup opened from the admin priority list's seat count.
func (app *App) sourceSeatDetails() map[string][]sourceSeat {
	out := map[string][]sourceSeat{}
	avatarIdx := app.buildAvatarIndex()
	maps, _ := app.db.ListMaps()
	for _, m := range maps {
		if m.Published == "no" || m.Mapname == "overview" {
			continue
		}
		desks, _ := app.db.ListDesks(m.Mapname)
		occupancy := app.assignMapOccupancy(m.Mapname, desks, avatarIdx)
		for _, occ := range occupancy {
			for _, o := range occ {
				name := o.name
				if name == "" {
					name = strings.TrimSpace(o.fname + " " + o.lname)
				}
				if name == "" {
					name = o.mail
				}
				out[o.sourceRef] = append(out[o.sourceRef], sourceSeat{Name: name, Map: m.Mapname, Desk: o.desknumber})
			}
		}
	}
	for ref := range out {
		s := out[ref]
		sort.Slice(s, func(i, j int) bool {
			ni, nj := strings.ToLower(s[i].Name), strings.ToLower(s[j].Name)
			if ni != nj {
				return ni < nj
			}
			if s[i].Map != s[j].Map {
				return s[i].Map < s[j].Map
			}
			return s[i].Desk < s[j].Desk
		})
	}
	return out
}

// handleRestSourceSeats serves /rest/sourceseats?ref=<ref>, returning the people
// a single source effectively seats right now, sorted by name.
func (app *App) handleRestSourceSeats(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "ldap") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	ref := r.URL.Query().Get("ref")
	seats := app.sourceSeatDetails()[ref]
	if seats == nil {
		seats = []sourceSeat{}
	}
	desc := ref
	for _, s := range app.listUnifiedSources() {
		if s.Ref == ref {
			desc = s.Description
			break
		}
	}
	writeJSON(w, struct {
		Ref         string       `json:"ref"`
		Description string       `json:"description"`
		Count       int          `json:"count"`
		Seats       []sourceSeat `json:"seats"`
	}{Ref: ref, Description: desc, Count: len(seats), Seats: seats})
}
