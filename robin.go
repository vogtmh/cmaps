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
	ID   int    `json:"id"`
	Name string `json:"name"`
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
