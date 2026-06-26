package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// robinSyncWorkers is the number of rooms polled concurrently during a Robin
// sync. Each room costs two Robin API calls, so polling them in parallel is the
// single biggest speed-up for large organisations.
const robinSyncWorkers = 8

// Robin API base URL.
const robinAPIBase = "https://api.robinpowered.com/v1.0"

// robinClient performs an authenticated GET against the Robin API.
func (app *App) robinGet(path string, out interface{}) error {
	token := app.db.GetRobinSetting("robintoken")
	if token == "" {
		return fmt.Errorf("robin token not configured")
	}
	req, err := http.NewRequest(http.MethodGet, robinAPIBase+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Access-Token "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("robin %s: status %d", path, resp.StatusCode)
	}
	return json.Unmarshal(body, out)
}

// --- Robin API response shapes (only the fields used) ---

type robinSpaceList struct {
	Data []robinSpaceEntry `json:"data"`
}

type robinSpaceEntry struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Behaviors []string `json:"behaviors"`
}

// --- Robin desk (seat) API response shapes ---

type robinSeatList struct {
	Data []robinSeatEntry `json:"data"`
}

type robinSeatEntry struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type robinSeatResvList struct {
	Data []robinSeatResv `json:"data"`
}

type robinSeatResv struct {
	SeatID   int           `json:"seat_id"`
	Type     string        `json:"type"`
	Start    robinDateTime `json:"start"`
	End      robinDateTime `json:"end"`
	Reservee struct {
		Email  string `json:"email"`
		UserID int    `json:"user_id"`
	} `json:"reservee"`
}

type robinDateTime struct {
	DateTime string `json:"date_time"`
	TimeZone string `json:"time_zone"`
}

type robinState struct {
	Data struct {
		Availability string `json:"availability"`
	} `json:"data"`
}

type robinEvents struct {
	Data []struct {
		Title string `json:"title"`
		Start struct {
			DateTime string `json:"date_time"`
			TimeZone string `json:"time_zone"`
		} `json:"start"`
		End struct {
			DateTime string `json:"date_time"`
			TimeZone string `json:"time_zone"`
		} `json:"end"`
	} `json:"data"`
}

// refreshRobin re-polls Robin for a single map. An empty mapName refreshes every
// configured space. Multiple spaces may map to the same CompanyMaps map.
func (app *App) refreshRobin(mapName string) {
	spaces, _ := app.db.ListRobinSpaces()
	for _, s := range spaces {
		if mapName != "" && s.MapName() != mapName {
			continue
		}
		if err := app.pollRobinLocation(s.Spaceid, s.MapName()); err != nil {
			log.Printf("robin poll %s: %v", s.Spacename, err)
		}
	}
}

// pollRobinLocation fetches every room in a Robin location and caches its status.
// The RobinSpace.Spacename is used as the CompanyMaps map name.
func (app *App) pollRobinLocation(locationID int, mapName string) error {
	var list robinSpaceList
	if err := app.robinGet(fmt.Sprintf("/locations/%d/spaces?page=1&per_page=200", locationID), &list); err != nil {
		return err
	}
	for _, room := range list.Data {
		if err := app.pollRobinRoom(room.ID, room.Name, mapName); err != nil {
			log.Printf("robin room %s/%s: %v", mapName, room.Name, err)
		}
	}
	return nil
}

// pollRobinRoom caches state + current/next event for a single Robin room.
func (app *App) pollRobinRoom(roomID int, roomName, mapName string) error {
	var state robinState
	if err := app.robinGet(fmt.Sprintf("/spaces/%d/state", roomID), &state); err != nil {
		return err
	}

	after := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02T15:04:05Z")
	before := time.Now().UTC().Add(144 * time.Hour).Format("2006-01-02T15:04:05Z")
	var events robinEvents
	if err := app.robinGet(fmt.Sprintf("/spaces/%d/events?after=%s&before=%s&page=1&per_page=200", roomID, after, before), &events); err != nil {
		return err
	}

	ev := roomEventWindows(events)
	deskid := app.findMeetingDeskID(mapName, roomName)

	return app.db.PutMeetingStatus(MeetingStatus{
		Map: mapName, Room: roomName, Availability: state.Data.Availability,
		NowTitle: ev.nowTitle, NowStart: ev.nowStart, NowEnd: ev.nowEnd, NowTz: ev.nowTz,
		NextTitle: ev.nextTitle, NextStart: ev.nextStart, NextEnd: ev.nextEnd, NextTz: ev.nextTz,
		Deskid: deskid,
	})
}

// roomWindows holds the formatted current/next event details for a room.
type roomWindows struct {
	nowTitle, nowStart, nowEnd, nowTz     string
	nextTitle, nextStart, nextEnd, nextTz string
}

// parseRobinTime parses a Robin event timestamp. The PHP version relied on
// strtotime(), which accepts a wide range of formats. Robin's date_time fields
// may come with a timezone offset, with a trailing Z, with fractional seconds,
// or as a naive local datetime (the timezone is carried separately in the
// time_zone field). time.Parse(time.RFC3339, …) is strict and rejects several
// of these, which left the current/next windows empty even though the room
// availability still resolved. Try the common layouts in turn, mirroring the
// lenient behaviour PHP had.
func parseRobinTime(s string) (time.Time, bool) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05Z07:00",
		// Robin returns colon-less numeric offsets, e.g. "+1200" / "+0930".
		"2006-01-02T15:04:05.999999999Z0700",
		"2006-01-02T15:04:05Z0700",
		"2006-01-02T15:04:05.999999999", // naive, with fractional seconds
		"2006-01-02T15:04:05",           // naive
		"2006-01-02 15:04:05",
	}
	for _, l := range layouts {
		// Naive layouts (no zone token) are interpreted in the local timezone,
		// matching strtotime()'s handling of offset-less timestamps.
		if !strings.ContainsAny(l, "Z7") {
			if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
				return t, true
			}
			continue
		}
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// roomEventWindows derives the current and next event windows from a Robin
// events response.
func roomEventWindows(events robinEvents) roomWindows {
	now := time.Now()
	var w roomWindows
	for _, e := range events.Data {
		start, ok1 := parseRobinTime(e.Start.DateTime)
		end, ok2 := parseRobinTime(e.End.DateTime)
		if !ok1 || !ok2 {
			continue
		}
		if start.Before(now) && now.Before(end) && w.nowStart == "" {
			w.nowStart = start.Format("3:04 PM")
			w.nowEnd = end.Format("3:04 PM")
			w.nowTz = e.End.TimeZone
			w.nowTitle = clampTitle(e.Title, "In use")
		}
		if start.After(now) && w.nextStart == "" {
			w.nextStart = start.Format("3:04 PM")
			w.nextEnd = end.Format("3:04 PM")
			w.nextTz = e.End.TimeZone
			w.nextTitle = clampTitle(e.Title, "Booked for")
		}
	}
	return w
}

// findMeetingDeskID returns the desk ID of the meeting desk named roomName on a
// given map, or "" when none exists.
func (app *App) findMeetingDeskID(mapName, roomName string) string {
	desks, _ := app.db.ListDesks(mapName)
	for _, d := range desks {
		if strings.EqualFold(d.Desktype, "meeting") && strings.EqualFold(d.Desknumber, roomName) {
			return fmt.Sprintf("%d", d.ID)
		}
	}
	return ""
}

func clampTitle(title, fallback string) string {
	if strings.TrimSpace(title) == "" {
		return fallback
	}
	if len(title) > 40 {
		return title[:40] + "..."
	}
	return title
}

// StartRobinScheduler refreshes the Robin meeting cache on a fixed interval.
// No-op while no Robin token is configured.
func (app *App) StartRobinScheduler(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if app.db.GetRobinSetting("robintoken") == "" {
				continue
			}
			// A scheduled run also records the last-sync time and per-room match
			// results so the admin Sync tab can show what happened.
			app.RunRobinSyncStructured()
		}
	}()
}

// RunRobinSyncVerbose performs a full Robin meeting sync while collecting a
// human-readable log of every step. It is used by the admin "Test meeting sync"
// button so an operator can see exactly what happens (and why nothing shows up).
func (app *App) RunRobinSyncVerbose() []string {
	var logs []string
	add := func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}

	if app.db.GetRobinSetting("robintoken") == "" {
		add("ERROR: Robin access token is not configured. Enter it above and save first.")
		return logs
	}
	add("Robin access token is configured.")
	if org := app.db.GetRobinSetting("robinOrganisation"); org != "" {
		add("Robin organisation: %s", org)
	}

	spaces, _ := app.db.ListRobinSpaces()
	add("Configured spaces (map name -> Robin location id): %d", len(spaces))
	if len(spaces) == 0 {
		add("No spaces configured. Add at least one mapping below before rooms can sync.")
		return logs
	}

	totalRooms := 0
	for _, s := range spaces {
		add("")
		if s.MapName() != s.Spacename {
			add("== Space \"%s\" (location id %d) -> map \"%s\" ==", s.Spacename, s.Spaceid, s.MapName())
		} else {
			add("== Space \"%s\" (location id %d) ==", s.Spacename, s.Spaceid)
		}
		var list robinSpaceList
		if err := app.robinGet(fmt.Sprintf("/locations/%d/spaces?page=1&per_page=200", s.Spaceid), &list); err != nil {
			add("  ERROR fetching rooms for location %d: %v", s.Spaceid, err)
			continue
		}
		add("  Robin returned %d room(s) for this location.", len(list.Data))
		for _, room := range list.Data {
			logs = append(logs, app.pollRobinRoomVerbose(room.ID, room.Name, s.MapName())...)
			totalRooms++
		}
	}
	add("")
	add("Done. Processed %d room(s) across %d space(s). Cache updated.", totalRooms, len(spaces))
	return logs
}

// pollRobinRoomVerbose caches a single room's status and returns a log of what
// happened, mirroring pollRobinRoom but with diagnostics for the admin modal.
func (app *App) pollRobinRoomVerbose(roomID int, roomName, mapName string) []string {
	var logs []string
	add := func(format string, args ...interface{}) {
		logs = append(logs, "  "+fmt.Sprintf(format, args...))
	}

	var state robinState
	if err := app.robinGet(fmt.Sprintf("/spaces/%d/state", roomID), &state); err != nil {
		add("Room \"%s\" (#%d): ERROR fetching state: %v", roomName, roomID, err)
		return logs
	}
	add("Room \"%s\" (#%d): availability=%s", roomName, roomID, state.Data.Availability)

	after := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02T15:04:05Z")
	before := time.Now().UTC().Add(144 * time.Hour).Format("2006-01-02T15:04:05Z")
	var events robinEvents
	if err := app.robinGet(fmt.Sprintf("/spaces/%d/events?after=%s&before=%s&page=1&per_page=200", roomID, after, before), &events); err != nil {
		add("    ERROR fetching events: %v", err)
		return logs
	}

	ev := roomEventWindows(events)
	if ev.nowStart != "" {
		add("    now: %s (%s - %s)", ev.nowTitle, ev.nowStart, ev.nowEnd)
	} else {
		add("    now: free")
	}
	if ev.nextStart != "" {
		add("    next: %s (%s - %s)", ev.nextTitle, ev.nextStart, ev.nextEnd)
	}

	deskid := app.findMeetingDeskID(mapName, roomName)
	if deskid == "" {
		add("    no meeting desk named \"%s\" on map \"%s\" — status cached but not shown on the map.", roomName, mapName)
	} else {
		add("    matched meeting desk id %s on map \"%s\".", deskid, mapName)
	}

	if err := app.db.PutMeetingStatus(MeetingStatus{
		Map: mapName, Room: roomName, Availability: state.Data.Availability,
		NowTitle: ev.nowTitle, NowStart: ev.nowStart, NowEnd: ev.nowEnd, NowTz: ev.nowTz,
		NextTitle: ev.nextTitle, NextStart: ev.nextStart, NextEnd: ev.nextEnd, NextTz: ev.nextTz,
		Deskid: deskid,
	}); err != nil {
		add("    ERROR caching status: %v", err)
	}
	return logs
}

// --- Structured sync result (powers the admin Sync tab "last sync" view) ---

// RobinSyncRoom is a single room's outcome during a sync.
type RobinSyncRoom struct {
	Name         string `json:"name"`
	ID           int    `json:"id"`
	Availability string `json:"availability"`
	Matched      bool   `json:"matched"`
	Deskid       string `json:"deskid"`
	NowTitle     string `json:"now_title"`
	NowStart     string `json:"now_start"`
	NowEnd       string `json:"now_end"`
	NextTitle    string `json:"next_title"`
	NextStart    string `json:"next_start"`
	NextEnd      string `json:"next_end"`
	Err          string `json:"err"`
}

// RobinSyncLocation groups the rooms returned for one configured Robin location.
type RobinSyncLocation struct {
	Spacename string          `json:"spacename"`
	Mapname   string          `json:"mapname"`
	Spaceid   int             `json:"spaceid"`
	Err       string          `json:"err"`
	Rooms     []RobinSyncRoom `json:"rooms"`
}

// RobinSyncResult is the persisted summary of the most recent Robin sync.
type RobinSyncResult struct {
	Time         string              `json:"time"`
	Org          string              `json:"org"`
	TotalRooms   int                 `json:"total_rooms"`
	MatchedRooms int                 `json:"matched_rooms"`
	Note         string              `json:"note"`
	Locations    []RobinSyncLocation `json:"locations"`
}

// RunRobinSyncStructured performs a full Robin meeting sync, updates the meeting
// cache, and records a structured per-room result (with match status and the
// time of the run) so the admin Sync tab can show exactly what was synced.
func (app *App) RunRobinSyncStructured() RobinSyncResult {
	return app.runRobinSyncStructured(nil)
}

// runRobinSyncStructured is the worker behind RunRobinSyncStructured. When prog
// is non-nil it reports determinate progress (room-by-room) and a live log so
// the admin Sync tab can render a progress bar during the (often slow) sync.
func (app *App) runRobinSyncStructured(prog *syncProgress) RobinSyncResult {
	res := RobinSyncResult{
		Time: time.Now().Format("2006-01-02 15:04:05"),
		Org:  app.db.GetRobinSetting("robinOrganisation"),
	}
	if app.db.GetRobinSetting("robintoken") == "" {
		res.Note = "Robin access token is not configured."
		if prog != nil {
			prog.logf("Robin access token is not configured.")
		}
		app.saveRobinSyncResult(res)
		return res
	}
	spaces, _ := app.db.ListRobinSpaces()
	sort.Slice(spaces, func(i, j int) bool { return spaces[i].Spacename < spaces[j].Spacename })
	if len(spaces) == 0 {
		res.Note = "No Robin locations configured yet."
		if prog != nil {
			prog.logf("No Robin locations configured yet.")
		}
		app.saveRobinSyncResult(res)
		return res
	}

	// Phase 1: fetch the room list for every location so we know the total room
	// count up-front (this makes the progress bar determinate).
	type locRooms struct {
		loc   RobinSyncLocation
		rooms []robinSpaceEntry
	}
	var work []locRooms
	totalRooms := 0
	if prog != nil {
		prog.setStage("Fetching locations…")
		prog.logf("Found %d configured location(s). Fetching room lists…", len(spaces))
	}
	for _, s := range spaces {
		loc := RobinSyncLocation{Spacename: s.Spacename, Mapname: s.MapName(), Spaceid: s.Spaceid}
		var list robinSpaceList
		if err := app.robinGet(fmt.Sprintf("/locations/%d/spaces?page=1&per_page=200", s.Spaceid), &list); err != nil {
			loc.Err = err.Error()
			if prog != nil {
				prog.logf("✗ %s (id %d): %s", s.Spacename, s.Spaceid, err.Error())
			}
			work = append(work, locRooms{loc: loc})
			continue
		}
		if prog != nil {
			prog.logf("• %s → %s (id %d): %d room(s)", s.Spacename, s.MapName(), s.Spaceid, len(list.Data))
		}
		totalRooms += len(list.Data)
		work = append(work, locRooms{loc: loc, rooms: list.Data})
	}
	if prog != nil {
		prog.setTotal(totalRooms)
		prog.setStage("Polling rooms…")
	}

	// Phase 2: poll each room (the slow part: two API calls per room). Rooms are
	// independent, so a bounded worker pool polls several at once. Results are
	// written into pre-sized slots keyed by (location, room) index so the final
	// output keeps the same deterministic order as the sequential version.
	type job struct {
		li, ri int
		room   robinSpaceEntry
		mapnm  string
	}
	roomResults := make([][]RobinSyncRoom, len(work))
	var jobs []job
	for li := range work {
		roomResults[li] = make([]RobinSyncRoom, len(work[li].rooms))
		for ri, room := range work[li].rooms {
			jobs = append(jobs, job{li: li, ri: ri, room: room, mapnm: work[li].loc.Mapname})
		}
	}

	jobCh := make(chan job)
	var wg sync.WaitGroup
	for i := 0; i < robinSyncWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobCh {
				r := app.pollRobinRoomStructured(j.room.ID, j.room.Name, j.mapnm)
				roomResults[j.li][j.ri] = r
				if prog != nil {
					switch {
					case r.Err != "":
						prog.logf("    ✗ %s: %s", j.room.Name, r.Err)
					case r.Matched:
						prog.logf("    ✓ %s → desk %s (%s)", j.room.Name, r.Deskid, r.Availability)
					default:
						prog.logf("    – %s: no matching desk (%s)", j.room.Name, r.Availability)
					}
					prog.step("")
				}
			}
		}()
	}
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)
	wg.Wait()

	// Assemble the per-location results in their original order.
	for li := range work {
		loc := work[li].loc
		for _, r := range roomResults[li] {
			loc.Rooms = append(loc.Rooms, r)
			res.TotalRooms++
			if r.Matched {
				res.MatchedRooms++
			}
		}
		res.Locations = append(res.Locations, loc)
	}
	app.saveRobinSyncResult(res)
	return res
}

// pollRobinRoomStructured caches a single room's status and returns a structured
// summary of the outcome (mirrors pollRobinRoom).
func (app *App) pollRobinRoomStructured(roomID int, roomName, mapName string) RobinSyncRoom {
	r := RobinSyncRoom{Name: roomName, ID: roomID}

	var state robinState
	if err := app.robinGet(fmt.Sprintf("/spaces/%d/state", roomID), &state); err != nil {
		r.Err = err.Error()
		return r
	}
	r.Availability = state.Data.Availability

	after := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02T15:04:05Z")
	before := time.Now().UTC().Add(144 * time.Hour).Format("2006-01-02T15:04:05Z")
	var events robinEvents
	if err := app.robinGet(fmt.Sprintf("/spaces/%d/events?after=%s&before=%s&page=1&per_page=200", roomID, after, before), &events); err != nil {
		r.Err = err.Error()
		return r
	}

	ev := roomEventWindows(events)
	r.NowTitle = ev.nowTitle
	r.NowStart = ev.nowStart
	r.NowEnd = ev.nowEnd
	r.NextTitle = ev.nextTitle
	r.NextStart = ev.nextStart
	r.NextEnd = ev.nextEnd

	deskid := app.findMeetingDeskID(mapName, roomName)
	r.Deskid = deskid
	r.Matched = deskid != ""

	_ = app.db.PutMeetingStatus(MeetingStatus{
		Map: mapName, Room: roomName, Availability: state.Data.Availability,
		NowTitle: ev.nowTitle, NowStart: ev.nowStart, NowEnd: ev.nowEnd, NowTz: ev.nowTz,
		NextTitle: ev.nextTitle, NextStart: ev.nextStart, NextEnd: ev.nextEnd, NextTz: ev.nextTz,
		Deskid: deskid,
	})
	return r
}

// saveRobinSyncResult persists the most recent structured sync result as JSON.
func (app *App) saveRobinSyncResult(res RobinSyncResult) {
	if b, err := json.Marshal(res); err == nil {
		_ = app.db.SetRobinSetting("robinLastSync", string(b))
	}
}

// LastRobinSyncResult returns the most recently persisted sync result, if any.
func (app *App) LastRobinSyncResult() (RobinSyncResult, bool) {
	js := app.db.GetRobinSetting("robinLastSync")
	if js == "" {
		return RobinSyncResult{}, false
	}
	var res RobinSyncResult
	if err := json.Unmarshal([]byte(js), &res); err != nil {
		return RobinSyncResult{}, false
	}
	return res, true
}

// --- Robin desk-data diagnostic dump ---
//
// This is a read-only diagnostic that walks the entire Robin sync surface
// (every configured location → every space → state/events/seats/reservations)
// and captures the RAW JSON of every response into a bundle. It does NOT write
// anything to the meeting cache, the booking feature, or the map; it only reads
// from Robin so the captured JSON can be inspected later. "Today" is the time
// window used for events and seat reservations.

// robinDumpFile is a single captured raw-JSON response in the diagnostic bundle.
type robinDumpFile struct {
	Name string
	Data []byte
}

// robinGetRaw performs an authenticated GET and returns the raw response body
// (plus HTTP status) without unmarshalling, so the diagnostic can capture
// exactly what Robin sent.
func (app *App) robinGetRaw(path string) ([]byte, int, error) {
	token := app.db.GetRobinSetting("robintoken")
	if token == "" {
		return nil, 0, fmt.Errorf("robin token not configured")
	}
	req, err := http.NewRequest(http.MethodGet, robinAPIBase+path, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Access-Token "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return body, resp.StatusCode, err
	}
	if resp.StatusCode >= 300 {
		return body, resp.StatusCode, fmt.Errorf("status %d", resp.StatusCode)
	}
	return body, resp.StatusCode, nil
}

// robinDataCount counts the elements in a Robin {"data":[...]} envelope without
// robinDataCount counts the elements in a Robin {"data":[...]} envelope without
// sanitizeDumpSegment makes a string safe to use as a file-path segment inside
// the diagnostic zip.
func sanitizeDumpSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unnamed"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ' || r == '/' || r == '\\' || r == '.':
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		return "unnamed"
	}
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

// RobinDeskDumpResult carries the headline counts of a desk diagnostic run so
// the caller can build a one-line summary for the progress bar.
type RobinDeskDumpResult struct {
	Locations   int
	Seats       int
	OccupiedNow int
	Matched     int
	Unmatched   int
	Files       int
}

// RunRobinDeskDump runs the desk diagnostic without progress reporting.
func (app *App) RunRobinDeskDump() ([]string, []robinDumpFile) {
	logs, files, _ := app.runRobinDeskDump(nil)
	return logs, files
}

// runRobinDeskDump walks every configured Robin location and captures the raw
// JSON of the full sync surface (spaces, and for each space its state, events,
// seats and seat reservations for today). For every seat reservation that is
// active *right now*, it resolves the occupant (reservee email → LDAP mirror
// name) and matches the Robin seat name to a CompanyMaps desk (Desknumber) on
// the mapped map, logging each match. When prog is non-nil it reports a
// determinate progress bar (one step per Robin space) and a live log. Nothing
// is persisted to the meeting cache, the booking feature, or the map.
func (app *App) runRobinDeskDump(prog *syncProgress) ([]string, []robinDumpFile, RobinDeskDumpResult) {
	var logs []string
	add := func(format string, args ...interface{}) {
		line := fmt.Sprintf(format, args...)
		logs = append(logs, line)
		if prog != nil {
			prog.logf("%s", line)
		}
	}
	var files []robinDumpFile
	capture := func(name string, raw []byte) {
		if raw == nil {
			raw = []byte{}
		}
		files = append(files, robinDumpFile{Name: name, Data: raw})
	}
	var res RobinDeskDumpResult

	if app.db.GetRobinSetting("robintoken") == "" {
		add("ERROR: Robin access token is not configured. Enter it on the Credentials card and save first.")
		return logs, files, res
	}

	// Today's window (local day → UTC), used for events and seat reservations.
	now := time.Now()
	startLocal := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endLocal := startLocal.Add(24 * time.Hour)
	after := startLocal.UTC().Format("2006-01-02T15:04:05Z")
	before := endLocal.UTC().Format("2006-01-02T15:04:05Z")
	add("Window (today): %s → %s (UTC)", after, before)

	spaces, _ := app.db.ListRobinSpaces()
	sort.Slice(spaces, func(i, j int) bool { return spaces[i].Spacename < spaces[j].Spacename })
	res.Locations = len(spaces)
	add("Configured location(s): %d", len(spaces))
	if len(spaces) == 0 {
		add("No Robin locations configured. Add at least one mapping first.")
		return logs, files, res
	}

	// LDAP mirror for resolving reservee emails → display names.
	ldap, _ := app.db.ListLdap()
	emailUser := make(map[string]LdapUser)
	for _, u := range ldap {
		if u.Mail != "" {
			emailUser[strings.ToLower(u.Mail)] = u
		}
	}

	// Desk lookups per CompanyMaps map (built lazily, reused across spaces).
	deskByMap := make(map[string]map[string]Desk)
	deskLookup := func(mapName string) map[string]Desk {
		if m, ok := deskByMap[mapName]; ok {
			return m
		}
		m := make(map[string]Desk)
		ds, _ := app.db.ListDesks(mapName)
		for _, d := range ds {
			m[strings.ToLower(strings.TrimSpace(d.Desknumber))] = d
		}
		deskByMap[mapName] = m
		return m
	}

	// Phase 1: fetch + capture every location's space list so the progress bar
	// is determinate (one step per space).
	type locWork struct {
		s      RobinSpace
		dir    string
		spaces []robinSpaceEntry
	}
	var work []locWork
	totalSpaces := 0
	if prog != nil {
		prog.setStage("Fetching locations…")
	}
	for _, s := range spaces {
		dir := fmt.Sprintf("location_%d_%s", s.Spaceid, sanitizeDumpSegment(s.Spacename))
		rawSpaces, status, err := app.robinGetRaw(fmt.Sprintf("/locations/%d/spaces?page=1&per_page=200", s.Spaceid))
		capture(dir+"/spaces.json", rawSpaces)
		if err != nil {
			add("✗ %s (location id %d): ERROR fetching spaces (HTTP %d): %v", s.Spacename, s.Spaceid, status, err)
			work = append(work, locWork{s: s, dir: dir})
			continue
		}
		var list robinSpaceList
		_ = json.Unmarshal(rawSpaces, &list)
		totalSpaces += len(list.Data)
		work = append(work, locWork{s: s, dir: dir, spaces: list.Data})
	}
	if prog != nil {
		prog.setTotal(totalSpaces)
		prog.setStage("Polling spaces…")
	}

	for _, lw := range work {
		s := lw.s
		add("")
		add("== %s (location id %d) → map %s ==", s.Spacename, s.Spaceid, s.MapName())
		desks := deskLookup(s.MapName())

		for _, sp := range lw.spaces {
			base := fmt.Sprintf("%s/space_%d_%s", lw.dir, sp.ID, sanitizeDumpSegment(sp.Name))

			// State + events (captured for the bundle; not needed for desks).
			if raw, _, _ := app.robinGetRaw(fmt.Sprintf("/spaces/%d/state", sp.ID)); true {
				capture(base+"_state.json", raw)
			}
			if raw, _, _ := app.robinGetRaw(fmt.Sprintf("/spaces/%d/events?after=%s&before=%s&page=1&per_page=200", sp.ID, after, before)); true {
				capture(base+"_events.json", raw)
			}

			// Seats (paginated) → seat id → name lookup.
			seatName := make(map[int]string)
			seatCount := 0
			for page := 1; page <= 50; page++ {
				raw, _, e := app.robinGetRaw(fmt.Sprintf("/spaces/%d/seats?page=%d&per_page=200", sp.ID, page))
				capture(fmt.Sprintf("%s_seats_p%d.json", base, page), raw)
				if e != nil {
					break
				}
				var sl robinSeatList
				if json.Unmarshal(raw, &sl) != nil {
					break
				}
				for _, st := range sl.Data {
					seatName[st.ID] = st.Name
					seatCount++
				}
				if len(sl.Data) < 200 {
					break
				}
			}
			res.Seats += seatCount

			// Seat reservations for today (paginated).
			var resvs []robinSeatResv
			for page := 1; page <= 50; page++ {
				raw, _, e := app.robinGetRaw(fmt.Sprintf("/reservations/seats?space_ids=%d&after=%s&before=%s&page=%d&per_page=200", sp.ID, after, before, page))
				capture(fmt.Sprintf("%s_reservations_p%d.json", base, page), raw)
				if e != nil {
					break
				}
				var rl robinSeatResvList
				if json.Unmarshal(raw, &rl) != nil {
					break
				}
				resvs = append(resvs, rl.Data...)
				if len(rl.Data) < 200 {
					break
				}
			}

			// Evaluate occupancy active right now and match to a CompanyMaps desk.
			for _, rv := range resvs {
				st, ok1 := parseRobinTime(rv.Start.DateTime)
				en, ok2 := parseRobinTime(rv.End.DateTime)
				if !ok1 || !ok2 {
					continue
				}
				if !(st.Before(now) && now.Before(en)) {
					continue // only reservations active right now
				}
				res.OccupiedNow++

				nm := seatName[rv.SeatID]
				who := rv.Reservee.Email
				if u, ok := emailUser[strings.ToLower(rv.Reservee.Email)]; ok {
					full := strings.TrimSpace(u.Givenname + " " + u.Surname)
					if full != "" {
						who = full + " <" + rv.Reservee.Email + ">"
					}
				}
				if who == "" {
					who = fmt.Sprintf("user #%d", rv.Reservee.UserID)
				}

				if d, matched := desks[strings.ToLower(strings.TrimSpace(nm))]; matched {
					res.Matched++
					add("  ✓ %s → desk #%d on %s — occupied now by %s (%s, until %s)",
						nm, d.ID, s.MapName(), who, rv.Type, en.Format("15:04"))
				} else {
					res.Unmatched++
					seatLabel := nm
					if seatLabel == "" {
						seatLabel = fmt.Sprintf("seat #%d", rv.SeatID)
					}
					add("  – %s (in \"%s\") occupied now by %s — no matching desk on %s",
						seatLabel, sp.Name, who, s.MapName())
				}
			}

			if prog != nil {
				prog.step("")
			}
		}
	}

	res.Files = len(files)
	add("")
	add("Done. %d location(s), %d seat(s) seen. Occupied right now: %d (matched a CompanyMaps desk: %d, no matching desk: %d).",
		res.Locations, res.Seats, res.OccupiedNow, res.Matched, res.Unmatched)
	add("Captured %d raw JSON file(s). Use \"Download JSON bundle (zip)\" to export everything.", res.Files)
	return logs, files, res
}
