package main

import (
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// handleAdminPostItemTypes routes custom item-type submissions.
func (app *App) handleAdminPostItemTypes(r *http.Request, sess Session) string {
	if app.permLevel(sess, "desks") < 2 {
		return ""
	}
	return app.saveItemTypeFromForm(r, sess)
}

// handleAdminPostConfig handles the Config tab: item types and logo uploads.
func (app *App) handleAdminPostConfig(r *http.Request, sess Session) string {
	// Custom item-type management is rendered on the Config tab; route those
	// submissions to the item-type handler (gated on the Desks permission).
	if r.FormValue("deleteType") != "" || r.FormValue("typeLabel") != "" {
		if app.permLevel(sess, "desks") < 2 {
			return ""
		}
		return app.saveItemTypeFromForm(r, sess)
	}
	if app.permLevel(sess, "config") < 2 {
		return ""
	}
	return app.saveLogosFromForm(r, sess)
}

func itemTypeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	slug := strings.Trim(b.String(), "-")
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	return slug
}

// saveItemTypeFromForm creates, updates or deletes a custom item type and stores
// an optional uploaded PNG icon under data/itemtypes/<id>.png.
func (app *App) saveItemTypeFromForm(r *http.Request, sess Session) string {
	if del := r.FormValue("deleteType"); del != "" {
		_ = app.db.DeleteItemType(del)
		_ = os.Remove(app.cfg.DataPath("itemtypes", filepath.Base(del)+".png"))
		_ = app.db.AuditLog("Desks", sess.Username, "Custom item type removed ("+del+")")
		return "Item type removed."
	}

	label := strings.TrimSpace(r.FormValue("typeLabel"))
	if label == "" {
		return ""
	}
	orig := r.FormValue("typeOrigID")
	id := orig
	if id == "" {
		id = itemTypeSlug(label)
	}
	if id == "" {
		return "Error: the label must contain letters or digits."
	}

	t := CustomItemType{
		ID:          id,
		Label:       label,
		Description: strings.TrimSpace(r.FormValue("typeDescription")),
		Color:       orDefaultStr(strings.TrimSpace(r.FormValue("typeColor")), "#0979D8"),
		Size:        orDefaultStr(r.FormValue("typeSize"), "medium"),
	}
	// Preserve any existing icon on edit unless a new one is uploaded.
	if orig != "" {
		if prev, found, _ := app.db.GetItemType(orig); found {
			t.Icon = prev.Icon
		}
	}
	if r.MultipartForm != nil && len(r.MultipartForm.File["typeIcon"]) > 0 {
		if err := app.saveItemIcon(id, r.MultipartForm.File["typeIcon"][0]); err != nil {
			return "Error saving icon: " + err.Error()
		}
		t.Icon = "/itemicons/" + id + ".png"
	}
	if err := app.db.PutItemType(t); err != nil {
		return "Error saving item type: " + err.Error()
	}
	if orig == "" {
		_ = app.db.AuditLog("Desks", sess.Username, "Custom item type created ("+id+")")
		return "Item type created."
	}
	_ = app.db.AuditLog("Desks", sess.Username, "Custom item type updated ("+id+")")
	return "Item type updated."
}

// saveItemIcon decodes an uploaded image and writes it as a PNG into the data
// directory's itemtypes folder, named after the item type id.
func (app *App) saveItemIcon(id string, fh *multipart.FileHeader) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	img, _, err := image.Decode(src)
	if err != nil {
		return err
	}

	dst, err := os.Create(app.cfg.DataPath("itemtypes", filepath.Base(id)+".png"))
	if err != nil {
		return err
	}
	defer dst.Close()
	return png.Encode(dst, img)
} // normalizeMembers turns a user-entered member list (comma- and/or pipe-separated,
// possibly with surrounding spaces) into the stored format: full names joined by
func (app *App) saveLogosFromForm(r *http.Request, sess Session) string {
	uploads := []struct{ field, setting string }{
		{"logoRegular", "logo_regular"},
		{"logoHover", "logo_hover"},
	}
	saved := 0
	for _, u := range uploads {
		if r.MultipartForm == nil || len(r.MultipartForm.File[u.field]) == 0 {
			continue
		}
		if err := app.saveLogoImage(u.setting, r.MultipartForm.File[u.field][0]); err != nil {
			return "Error saving logo: " + err.Error()
		}
		_ = app.db.SetSetting(u.setting, "/logos/"+u.setting+".png")
		_ = app.db.AuditLog("Settings", sess.Username, "Logo updated ("+u.setting+")")
		saved++
	}
	if saved == 0 {
		return ""
	}
	return "Logo updated."
}

// saveLogoImage decodes an uploaded image and writes it as a PNG into the data
// directory's logos folder, named after the setting it backs.
func (app *App) saveLogoImage(name string, fh *multipart.FileHeader) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	img, _, err := image.Decode(src)
	if err != nil {
		return err
	}

	dst, err := os.Create(app.cfg.DataPath("logos", name+".png"))
	if err != nil {
		return err
	}
	defer dst.Close()
	return png.Encode(dst, img)
}

func (app *App) handleRestSetting(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "config") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" || name == "logo_regular" || name == "logo_hover" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	value := r.FormValue("value")
	_ = app.db.SetSetting(name, value)
	_ = app.db.AuditLog("Settings", sess.Username, "Base variable updated ("+name+")")
	writeJSON(w, map[string]string{"name": name, "value": value})
}

// handleRestMapCoords persists lat/lon (and optionally X/Y) for a single map.
// Used by the classic->modern switch review dialog to fill in coordinates that
var vipCategoryList = []struct{ Type, Color string }{
	{"Board", "#ffa500"},
	{"VP", "#800080"},
	{"Director", "#00bbff"},
	{"TeamManager", "#00CC00"},
}

// vipCategoriesPayload groups the stored VIP tags into the fixed categories so
// the admin chips UI can render (and the JS can re-render after edits).
func (app *App) vipCategoriesPayload() []map[string]interface{} {
	vips, _ := app.db.ListVips()
	byType := map[string][]string{}
	for _, v := range vips {
		if v.Title == "" {
			continue
		}
		byType[v.Type] = append(byType[v.Type], v.Title)
	}
	out := make([]map[string]interface{}, 0, len(vipCategoryList))
	for _, c := range vipCategoryList {
		tags := byType[c.Type]
		sort.Slice(tags, func(i, j int) bool { return strings.ToLower(tags[i]) < strings.ToLower(tags[j]) })
		out = append(out, map[string]interface{}{
			"type":  c.Type,
			"color": c.Color,
			"tags":  tags,
		})
	}
	return out
}

// handleRestVips powers the VIP chips UI: GET returns the grouped categories,
// POST adds or removes a tag and returns the updated grouping (so the page
// never has to reload).
func (app *App) handleRestVips(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok || app.permLevel(sess, "config") < 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if r.Method == http.MethodPost {
		if app.permLevel(sess, "config") < 2 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		action := r.FormValue("action")
		typ := strings.TrimSpace(r.FormValue("type"))
		tag := strings.TrimSpace(r.FormValue("tag"))
		valid := false
		for _, c := range vipCategoryList {
			if c.Type == typ {
				valid = true
				break
			}
		}
		if !valid || tag == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch action {
		case "add":
			_ = app.db.AddVipTag(typ, tag)
			_ = app.db.AuditLog("Settings", sess.Username, "VIP tag added ("+typ+": "+tag+")")
		case "remove":
			_ = app.db.DeleteVipTag(typ, tag)
			_ = app.db.AuditLog("Settings", sess.Username, "VIP tag removed ("+typ+": "+tag+")")
		default:
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
	}
	writeJSON(w, app.vipCategoriesPayload())
}

// resolveDirectoryEntry maps an entered value (samaccountname, DOMAIN\sam, or a
// display name) to a directory user. It returns false when nothing matches, in
