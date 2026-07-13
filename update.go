package main

import (
	"encoding/json"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// batchDeskOp is a single create/update operation in a mode=batch request.
type batchDeskOp struct {
	Op         string `json:"op"`
	ID         int    `json:"id"`
	Desktype   string `json:"desktype"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Desknumber string `json:"desknumber"`
	Employee   string `json:"employee"`
	Avatar     string `json:"avatar"`
	Department string `json:"department"`
}

// handleRestUpdate serves /rest/update for desk and map mutations. The legacy
// predictable date token is REPLACED with a session + "desks" write permission
// check (level >= 2).
func (app *App) handleRestUpdate(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "desks") < 2 {
		_ = app.db.AuditLog("Desks", sess.Username, "Unauthorized /rest/update attempt")
		http.Error(w, "not authorized", http.StatusForbidden)
		return
	}

	// Parse form (multipart when a map image is attached).
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		_ = r.ParseMultipartForm(20 << 20)
	} else {
		_ = r.ParseForm()
	}

	get := func(k string) string { return strings.TrimSpace(r.FormValue(k)) }

	mode := get("mode")
	mapName := get("map")
	desktype := get("desktype")
	desknumber := get("desknumber")
	employee := get("employee")
	avatar := get("avatar")
	department := get("department")
	user := get("user")
	if user == "" {
		user = sess.Username
	}
	if department == "- none -" || department == "NULL" {
		department = ""
	}
	x, _ := strconv.Atoi(get("x"))
	y, _ := strconv.Atoi(get("y"))
	id, _ := strconv.Atoi(get("id"))

	// Optional map image upload (createmap step 1).
	if r.MultipartForm != nil {
		if files := r.MultipartForm.File["image"]; len(files) > 0 {
			if err := app.saveMapImage(strings.ToLower(mapName), files[0]); err != nil {
				http.Error(w, "map upload failed: "+err.Error(), http.StatusBadRequest)
				return
			}
		}
	}

	var status, info, data string

	switch mode {
	case "create":
		if mapName == "" || get("x") == "" || get("y") == "" || desknumber == "" || (employee == "" && desktype != "localdesk" && !strings.HasPrefix(desktype, "custom_")) {
			_ = app.db.AuditLog("Desks", "System", "Missing parameters on create")
			http.Error(w, "parameters missing", http.StatusBadRequest)
			return
		}
		if (desktype == "hotseat" || desktype == "booking") && !app.internalBookingEnabled() {
			http.Error(w, "internal booking is disabled", http.StatusForbidden)
			return
		}
		newID, _ := app.db.NextDeskID(mapName)
		_ = app.db.PutDesk(Desk{
			ID: newID, Map: mapName, Desktype: desktype, X: x, Y: y,
			Desknumber: desknumber, Employee: employee, Avatar: avatar, Department: department,
		})
		status, info, data = strconv.Itoa(newID), desknumber, employee
		_ = app.db.AuditLog("Desks", user, "ID "+status+" created: Dsk="+desknumber+" Empl="+employee)

	case "update":
		if mapName == "" || get("id") == "" || get("x") == "" || get("y") == "" || desknumber == "" || (employee == "" && desktype != "localdesk" && !strings.HasPrefix(desktype, "custom_")) {
			http.Error(w, "parameters missing", http.StatusBadRequest)
			return
		}
		_ = app.db.PutDesk(Desk{
			ID: id, Map: mapName, Desktype: desktype, X: x, Y: y,
			Desknumber: desknumber, Employee: employee, Avatar: avatar, Department: department,
		})
		status, info, data = strconv.Itoa(id), desknumber, employee
		_ = app.db.AuditLog("Desks", user, "ID "+status+" updated: Dsk="+desknumber+" Empl="+employee)

	case "move":
		// Repositioning an existing desk (drag on the map). ONLY the coordinates
		// change; every other field is preserved from the stored record. A desk
		// occupied via an overlay (Robin/LDAP) is rendered with the transient
		// occupant's name/avatar and a synthetic desktype, so writing those back
		// on a move would bake the occupant into the desk (e.g. turn an addesk
		// into a localdesk named after whoever happened to sit there). Loading
		// the stored desk and touching only X/Y avoids that entirely — same
		// approach as the batch/auto-align update op.
		if mapName == "" || get("id") == "" || get("x") == "" || get("y") == "" {
			http.Error(w, "parameters missing", http.StatusBadRequest)
			return
		}
		existing, found, _ := app.db.GetDesk(mapName, id)
		if !found {
			http.Error(w, "desk not found", http.StatusNotFound)
			return
		}
		existing.X = x
		existing.Y = y
		_ = app.db.PutDesk(existing)
		status, info, data = strconv.Itoa(id), existing.Desknumber, "moved"
		_ = app.db.AuditLog("Desks", user, "ID "+status+" moved")

	case "delete":
		if mapName == "" || get("id") == "" {
			http.Error(w, "parameters missing", http.StatusBadRequest)
			return
		}
		_ = app.db.DeleteDesk(mapName, id)
		status, info, data = strconv.Itoa(id), "deleted", "deleted"
		_ = app.db.AuditLog("Desks", user, "ID "+status+" deleted")

	case "batch":
		// Bulk create/update of desks in a single round-trip. Used by cluster
		// placement and auto-align. The ops are supplied as a JSON array in the
		// "ops" form field so the request still goes through the normal form
		// parsing above (the body is already consumed by ParseForm).
		if mapName == "" || get("ops") == "" {
			http.Error(w, "parameters missing", http.StatusBadRequest)
			return
		}
		var ops []batchDeskOp
		if err := json.Unmarshal([]byte(get("ops")), &ops); err != nil {
			http.Error(w, "invalid ops: "+err.Error(), http.StatusBadRequest)
			return
		}
		results := make([]map[string]string, 0, len(ops))
		for _, op := range ops {
			dept := op.Department
			if dept == "- none -" || dept == "NULL" {
				dept = ""
			}
			switch op.Op {
			case "create":
				if op.Desknumber == "" || (op.Employee == "" && op.Desktype != "localdesk" && !strings.HasPrefix(op.Desktype, "custom_")) {
					continue
				}
				if (op.Desktype == "hotseat" || op.Desktype == "booking") && !app.internalBookingEnabled() {
					continue
				}
				newID, _ := app.db.NextDeskID(mapName)
				_ = app.db.PutDesk(Desk{
					ID: newID, Map: mapName, Desktype: op.Desktype, X: op.X, Y: op.Y,
					Desknumber: op.Desknumber, Employee: op.Employee, Avatar: op.Avatar, Department: dept,
				})
				results = append(results, map[string]string{"status": strconv.Itoa(newID), "info": op.Desknumber, "data": op.Employee})
			case "update":
				if op.ID == 0 {
					continue
				}
				existing, found, _ := app.db.GetDesk(mapName, op.ID)
				if !found {
					continue
				}
				// Auto-align only moves desks: keep all other fields intact and
				// just overwrite the coordinates.
				existing.X = op.X
				existing.Y = op.Y
				_ = app.db.PutDesk(existing)
				results = append(results, map[string]string{"status": strconv.Itoa(op.ID), "info": "moved", "data": "moved"})
			}
		}
		_ = app.db.AuditLog("Desks", user, "Batch desk write ("+strconv.Itoa(len(results))+" ops)")
		writeJSON(w, map[string]interface{}{"update": results})
		return

	case "createmap":
		itemscale := get("itemscale")
		published := get("published")
		mapflag := get("mapflag")
		timezone := get("timezone")
		address := get("address")
		// Coordinates are required as a pair: either pixel X/Y (classic overview)
		// or geographic lat/lon (dynamic world map). Address is optional.
		hasXY := get("x") != "" && get("y") != ""
		hasLatLon := get("lat") != "" && get("lon") != ""
		if mapName == "" || itemscale == "" || published == "" || mapflag == "" || timezone == "" || (!hasXY && !hasLatLon) {
			http.Error(w, "parameters missing", http.StatusBadRequest)
			return
		}
		mapName = strings.ToLower(mapName)
		if _, exists, _ := app.db.GetMap(mapName); exists {
			http.Error(w, "mapname already in use", http.StatusConflict)
			return
		}
		newMap := MapInfo{
			Mapname: mapName, Itemscale: itemscale, Published: published, Country: mapflag,
			Timezone: timezone, Address: address, MapX: x, MapY: y,
		}
		if lat, err := strconv.ParseFloat(get("lat"), 64); err == nil {
			newMap.Lat = lat
		}
		if lon, err := strconv.ParseFloat(get("lon"), 64); err == nil {
			newMap.Lon = lon
		}
		_ = app.db.PutMap(newMap)
		_ = app.db.AuditLog("Maps", user, "Map has been created ("+mapName+")")
		http.Redirect(w, r, "/?map=overview", http.StatusSeeOther)
		return

	default:
		http.Error(w, "unknown mode", http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]interface{}{
		"update": []map[string]string{{"status": status, "info": info, "data": data}},
	})
}

// saveMapImage decodes an uploaded map image (jpg/png/gif) and stores it as
// <map>.png in the data maps directory.
func (app *App) saveMapImage(mapName string, fh *multipart.FileHeader) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	img, _, err := image.Decode(src)
	if err != nil {
		return err
	}

	dst, err := os.Create(app.cfg.DataPath("maps", mapName+".png"))
	if err != nil {
		return err
	}
	defer dst.Close()
	return png.Encode(dst, img)
}
