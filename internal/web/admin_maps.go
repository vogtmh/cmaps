package web

import (
	"net/http"
	"os"
	"strconv"
	"strings"
)

// handleAdminPostMaps handles map create/update/delete and placeholder removal.
func (app *Server) handleAdminPostMaps(r *http.Request, sess Session) string {
	if app.permLevel(sess, "maps") < 2 {
		return ""
	}
	if id := r.FormValue("deleteMapID"); id != "" {
		name := r.FormValue("deleteMapname")
		_ = app.db.DeleteMap(name)
		// Remove the desks for that map and the map image.
		if desks, _ := app.db.ListDesks(name); desks != nil {
			for _, d := range desks {
				_ = app.db.DeleteDesk(name, d.ID)
			}
		}
		_ = removeFileIfExists(app.cfg.DataPath("maps", name+".png"))
		_ = app.db.AuditLog("Maps", sess.Username, "Map deleted ("+name+")")
		return "Map deleted."
	}
	if name := r.FormValue("nomapMapname"); name != "" {
		// Convert an existing map into a placeholder ("nomap"): remove the
		// image file but keep the map record so the location still shows on
		// the overview. Desks are left intact and reappear if an image is
		// uploaded again later.
		if name == "overview" {
			return ""
		}
		_ = removeFileIfExists(app.cfg.DataPath("maps", name+".png"))
		_ = app.db.AuditLog("Maps", sess.Username, "Map image removed, converted to placeholder ("+name+")")
		return "Map image removed. The map is now shown as a placeholder."
	}
	if r.FormValue("editMapOrigName") != "" {
		return app.updateMapFromForm(r, sess)
	}
	return app.createMapFromForm(r, sess)
}

func (app *Server) createMapFromForm(r *http.Request, sess Session) string {
	name := strings.ToLower(strings.TrimSpace(r.FormValue("newMapName")))
	if name == "" {
		return ""
	}
	if _, found, _ := app.db.GetMap(name); found {
		return "Error: a map with that name already exists."
	}

	x, _ := strconv.Atoi(r.FormValue("newMapX"))
	y, _ := strconv.Atoi(r.FormValue("newMapY"))
	m := MapInfo{
		Mapname:     name,
		DisplayName: strings.TrimSpace(r.FormValue("newMapDisplayName")),
		Itemscale:   orDefaultStr(r.FormValue("newMapItemscale"), "1"),
		Published:   orDefaultStr(r.FormValue("newMapPublished"), "yes"),
		Country:     strings.ToLower(r.FormValue("newMapCountry")),
		Timezone:    orDefaultStr(r.FormValue("newMapTimezone"), "Europe/Berlin"),
		Address:     addBR(r.FormValue("newMapAddress")),
		MapX:        x,
		MapY:        y,
	}
	if v := strings.TrimSpace(r.FormValue("newMapLat")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			m.Lat = f
		}
	}
	if v := strings.TrimSpace(r.FormValue("newMapLon")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			m.Lon = f
		}
	}

	// Save the uploaded map image if present.
	if r.MultipartForm != nil && len(r.MultipartForm.File["image"]) > 0 {
		if err := app.saveMapImage(name, r.MultipartForm.File["image"][0]); err != nil {
			return "Error saving map image: " + err.Error()
		}
	}

	_ = app.db.PutMap(m)
	_ = app.db.AuditLog("Maps", sess.Username, "Map created ("+name+")")
	return "Map created."
}

// stripBR converts stored HTML line breaks back to plain newlines so the admin
func (app *Server) updateMapFromForm(r *http.Request, sess Session) string {
	orig := strings.ToLower(strings.TrimSpace(r.FormValue("editMapOrigName")))
	if orig == "" {
		return ""
	}
	m, found, _ := app.db.GetMap(orig)
	if !found {
		return "Error: map not found."
	}
	newName := strings.ToLower(strings.TrimSpace(r.FormValue("editMapName")))
	if newName == "" {
		return "Error: map name cannot be empty."
	}

	// Apply the rename first so subsequent attribute writes target the new key.
	if newName != orig {
		if _, exists, _ := app.db.GetMap(newName); exists {
			return "Error: a map with that name already exists."
		}
		if err := app.db.RenameMap(orig, newName); err != nil {
			return "Error renaming map: " + err.Error()
		}
		oldPath := app.cfg.DataPath("maps", orig+".png")
		if _, err := os.Stat(oldPath); err == nil {
			_ = os.Rename(oldPath, app.cfg.DataPath("maps", newName+".png"))
		}
		if updated, ok, _ := app.db.GetMap(newName); ok {
			m = updated
		}
	}

	m.DisplayName = strings.TrimSpace(r.FormValue("editMapDisplayName"))
	m.Itemscale = orDefaultStr(r.FormValue("editMapItemscale"), "1")
	m.Published = orDefaultStr(r.FormValue("editMapPublished"), "yes")
	m.Country = strings.ToLower(r.FormValue("editMapCountry"))
	m.Timezone = orDefaultStr(r.FormValue("editMapTimezone"), "Europe/Berlin")
	m.Address = addBR(r.FormValue("editMapAddress"))
	if v := r.FormValue("editMapX"); v != "" {
		if x, err := strconv.Atoi(v); err == nil {
			m.MapX = x
		}
	}
	if v := r.FormValue("editMapY"); v != "" {
		if y, err := strconv.Atoi(v); err == nil {
			m.MapY = y
		}
	}
	if v := strings.TrimSpace(r.FormValue("editMapLat")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			m.Lat = f
		}
	}
	if v := strings.TrimSpace(r.FormValue("editMapLon")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			m.Lon = f
		}
	}

	// Replace the image only if a new one was uploaded.
	if r.MultipartForm != nil && len(r.MultipartForm.File["editImage"]) > 0 {
		if err := app.saveMapImage(m.Mapname, r.MultipartForm.File["editImage"][0]); err != nil {
			return "Error saving map image: " + err.Error()
		}
	}

	_ = app.db.PutMap(m)
	if newName != orig {
		_ = app.db.AuditLog("Maps", sess.Username, "Map renamed ("+orig+" -> "+newName+")")
	} else {
		_ = app.db.AuditLog("Maps", sess.Username, "Map updated ("+m.Mapname+")")
	}
	return "Map updated."
}

func (app *Server) handleRestMapCoords(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "maps") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	mapname := strings.ToLower(strings.TrimSpace(r.FormValue("mapname")))
	if mapname == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	m, found, _ := app.db.GetMap(mapname)
	if !found {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "map not found"})
		return
	}
	if v := strings.TrimSpace(r.FormValue("lat")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			m.Lat = f
		}
	}
	if v := strings.TrimSpace(r.FormValue("lon")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			m.Lon = f
		}
	}
	if v := strings.TrimSpace(r.FormValue("x")); v != "" {
		if x, err := strconv.Atoi(v); err == nil {
			m.MapX = x
		}
	}
	if v := strings.TrimSpace(r.FormValue("y")); v != "" {
		if y, err := strconv.Atoi(v); err == nil {
			m.MapY = y
		}
	}
	if err := app.db.PutMap(m); err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "message": err.Error()})
		return
	}
	_ = app.db.AuditLog("Maps", sess.Username, "Map coordinates set ("+mapname+")")
	writeJSON(w, map[string]interface{}{"ok": true, "mapname": mapname, "lat": m.Lat, "lon": m.Lon})
}

// vipCategoryList defines the fixed VIP categories and the border colors the
