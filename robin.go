package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Robin API base URL.
const robinAPIBase = "https://api.robinpowered.com/v1.0"

// robinClient performs an authenticated GET against the Robin API.
func (app *App) robinGet(path string, out interface{}) error {
	token := app.db.GetSetting("robintoken")
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
	Data []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"data"`
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

// roomEventWindows derives the current and next event windows from a Robin
// events response.
func roomEventWindows(events robinEvents) roomWindows {
	now := time.Now()
	var w roomWindows
	for _, e := range events.Data {
		start, err1 := time.Parse(time.RFC3339, e.Start.DateTime)
		end, err2 := time.Parse(time.RFC3339, e.End.DateTime)
		if err1 != nil || err2 != nil {
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
			if app.db.GetSetting("robintoken") == "" {
				continue
			}
			app.refreshRobin("")
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

	if app.db.GetSetting("robintoken") == "" {
		add("ERROR: Robin access token is not configured. Enter it above and save first.")
		return logs
	}
	add("Robin access token is configured.")
	if org := app.db.GetSetting("robinOrganisation"); org != "" {
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
