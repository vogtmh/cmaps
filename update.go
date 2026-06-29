package main

import (
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
		if mapName == "" || get("x") == "" || get("y") == "" || desknumber == "" || (employee == "" && desktype != "localdesk") {
			_ = app.db.AuditLog("Desks", "System", "Missing parameters on create")
			http.Error(w, "parameters missing", http.StatusBadRequest)
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
		if mapName == "" || get("id") == "" || get("x") == "" || get("y") == "" || desknumber == "" || (employee == "" && desktype != "localdesk") {
			http.Error(w, "parameters missing", http.StatusBadRequest)
			return
		}
		_ = app.db.PutDesk(Desk{
			ID: id, Map: mapName, Desktype: desktype, X: x, Y: y,
			Desknumber: desknumber, Employee: employee, Avatar: avatar, Department: department,
		})
		status, info, data = strconv.Itoa(id), desknumber, employee
		_ = app.db.AuditLog("Desks", user, "ID "+status+" updated: Dsk="+desknumber+" Empl="+employee)

	case "delete":
		if mapName == "" || get("id") == "" {
			http.Error(w, "parameters missing", http.StatusBadRequest)
			return
		}
		_ = app.db.DeleteDesk(mapName, id)
		status, info, data = strconv.Itoa(id), "deleted", "deleted"
		_ = app.db.AuditLog("Desks", user, "ID "+status+" deleted")

	case "createmap":
		itemscale := get("itemscale")
		published := get("published")
		mapflag := get("mapflag")
		timezone := get("timezone")
		address := get("address")
		if mapName == "" || itemscale == "" || published == "" || mapflag == "" || timezone == "" || address == "" || get("x") == "" || get("y") == "" {
			http.Error(w, "parameters missing", http.StatusBadRequest)
			return
		}
		mapName = strings.ToLower(mapName)
		if _, exists, _ := app.db.GetMap(mapName); exists {
			http.Error(w, "mapname already in use", http.StatusConflict)
			return
		}
		_ = app.db.PutMap(MapInfo{
			Mapname: mapName, Itemscale: itemscale, Published: published, Country: mapflag,
			Timezone: timezone, Address: address, MapX: x, MapY: y,
		})
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

	dst, err := os.Create(app.cfg.dataPath("maps", mapName+".png"))
	if err != nil {
		return err
	}
	defer dst.Close()
	return png.Encode(dst, img)
}
