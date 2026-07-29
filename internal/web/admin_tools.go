package web

import (
	"companymaps/internal/store"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// handleAdminPostTools handles the Tools tab: creating and removing application
// links. Every operation is gated on the "adminpanel" write permission so that
// only superadmins can manage the links, even though any admin-panel user can
// view them.
func (app *Server) handleAdminPostTools(r *http.Request, sess Session) string {
	if app.permLevel(sess, "adminpanel") < 2 {
		return ""
	}

	if del := r.FormValue("deleteTool"); del != "" {
		_ = app.db.DeleteAppTool(del)
		_ = os.Remove(app.cfg.DataPath("toolicons", filepath.Base(del)+".png"))
		_ = app.db.AuditLog("Tools", sess.Username, "Application link removed ("+del+")")
		return "Link removed."
	}

	name := strings.TrimSpace(r.FormValue("newName"))
	path := strings.TrimSpace(r.FormValue("newPath"))
	if name == "" || path == "" {
		return ""
	}

	id := app.uniqueToolID(itemTypeSlug(name))
	if id == "" {
		return "Error: the name must contain letters or digits."
	}

	// Place the new link at the end of the current list.
	order := 0
	if existing, err := app.db.ListAppTools(); err == nil {
		for _, t := range existing {
			if t.Order >= order {
				order = t.Order + 1
			}
		}
	}

	t := store.AppTool{ID: id, Name: name, Path: path, Order: order}
	if r.MultipartForm != nil && len(r.MultipartForm.File["newIcon"]) > 0 {
		if err := app.saveToolIcon(id, r.MultipartForm.File["newIcon"][0]); err != nil {
			return "Error saving icon: " + err.Error()
		}
		t.Icon = "/toolicons/" + id + ".png"
	}
	if err := app.db.PutAppTool(t); err != nil {
		return "Error saving link: " + err.Error()
	}
	_ = app.db.AuditLog("Tools", sess.Username, "Application link created ("+id+")")
	return "Link added."
}

// uniqueToolID returns a tool id based on the given slug, appending a numeric
// suffix if a link with that id already exists so distinct names never collide.
func (app *Server) uniqueToolID(slug string) string {
	if slug == "" {
		return ""
	}
	if _, found, _ := app.db.GetAppTool(slug); !found {
		return slug
	}
	for i := 2; ; i++ {
		candidate := slug + "-" + strconv.Itoa(i)
		if _, found, _ := app.db.GetAppTool(candidate); !found {
			return candidate
		}
	}
}

// saveToolIcon decodes an uploaded image and writes it as a PNG into the data
// directory's toolicons folder, named after the tool id.
func (app *Server) saveToolIcon(id string, fh *multipart.FileHeader) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	img, _, err := image.Decode(src)
	if err != nil {
		return err
	}

	dst, err := os.Create(app.cfg.DataPath("toolicons", filepath.Base(id)+".png"))
	if err != nil {
		return err
	}
	defer dst.Close()
	return png.Encode(dst, img)
}
