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

// refreshRobin re-polls Robin for a single map (RobinSpace.Spacename == map). An
// empty mapName refreshes every configured space.
func (app *App) refreshRobin(mapName string) {
	spaces, _ := app.db.ListRobinSpaces()
	for _, s := range spaces {
		if mapName != "" && s.Spacename != mapName {
			continue
		}
		if err := app.pollRobinLocation(s.Spaceid, s.Spacename); err != nil {
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

	now := time.Now()
	var nowTitle, nowStart, nowEnd, nowTz string
	var nextTitle, nextStart, nextEnd, nextTz string

	for _, e := range events.Data {
		start, err1 := time.Parse(time.RFC3339, e.Start.DateTime)
		end, err2 := time.Parse(time.RFC3339, e.End.DateTime)
		if err1 != nil || err2 != nil {
			continue
		}
		if start.Before(now) && now.Before(end) && nowStart == "" {
			nowStart = start.Format("3:04 PM")
			nowEnd = end.Format("3:04 PM")
			nowTz = e.End.TimeZone
			nowTitle = clampTitle(e.Title, "In use")
		}
		if start.After(now) && nextStart == "" {
			nextStart = start.Format("3:04 PM")
			nextEnd = end.Format("3:04 PM")
			nextTz = e.End.TimeZone
			nextTitle = clampTitle(e.Title, "Booked for")
		}
	}

	deskid := app.findMeetingDeskID(mapName, roomName)

	return app.db.PutMeetingStatus(MeetingStatus{
		Map: mapName, Room: roomName, Availability: state.Data.Availability,
		NowTitle: nowTitle, NowStart: nowStart, NowEnd: nowEnd, NowTz: nowTz,
		NextTitle: nextTitle, NextStart: nextStart, NextEnd: nextEnd, NextTz: nextTz,
		Deskid: deskid,
	})
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
