package main

import (
	"net/http"
)

// handleRestMeeting serves /rest/meeting?map=&usecache=. It returns the cached
// room status populated by the Robin background poller. When usecache is not set
// and Robin is configured, the requested map is refreshed first.
func (app *App) handleRestMeeting(w http.ResponseWriter, r *http.Request) {
	mapName := r.URL.Query().Get("map")
	useCache := r.URL.Query().Get("usecache")

	if useCache == "" && app.robinEnabled() && app.db.GetRobinSetting("robintoken") != "" {
		app.refreshRobin(mapName)
	}

	statuses, _ := app.db.ListMeetingStatus(mapName)
	rooms := make([]map[string]interface{}, 0, len(statuses))
	for _, s := range statuses {
		rooms = append(rooms, map[string]interface{}{
			"map":          s.Map,
			"name":         s.Room,
			"availability": s.Availability,
			"now_title":    s.NowTitle,
			"now_start":    s.NowStart,
			"now_end":      s.NowEnd,
			"now_tz":       s.NowTz,
			"next_title":   s.NextTitle,
			"next_start":   s.NextStart,
			"next_end":     s.NextEnd,
			"next_tz":      s.NextTz,
			"deskid":       s.Deskid,
		})
	}
	writeJSON(w, map[string]interface{}{"rooms": rooms})
}
