package main

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"strings"

	xdraw "golang.org/x/image/draw"
)

const avatarSize = 400

// handleRestAvatar serves /rest/avatar (POST mode=upload|delete). The avatar is
// center-cropped to a square and stored as avatarcache/<userid>.jpg, where userid
// is the part of the session username after the domain backslash.
//
// The legacy interactive crop UI (imgareaselect) is replaced with an automatic
// centered square crop. EXIF auto-rotation is not applied.
func (app *App) handleRestAvatar(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}

	userid := sess.Samaccountname
	if idx := strings.LastIndex(sess.Username, "\\"); idx >= 0 {
		userid = sess.Username[idx+1:]
	}
	if userid == "" {
		http.Error(w, "no user id", http.StatusBadRequest)
		return
	}

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		_ = r.ParseForm()
	}
	mode := r.FormValue("mode")
	target := app.cfg.dataPath("avatarcache", userid+".jpg")

	switch mode {
	case "delete":
		_ = os.Remove(target)
		_ = app.db.AuditLog("Avatar", sess.Username, "Avatar deleted for "+userid)
		writeJSON(w, map[string]string{"status": "ok", "message": "avatar deleted"})

	case "upload":
		if r.MultipartForm == nil || len(r.MultipartForm.File["images"]) == 0 {
			http.Error(w, "no image uploaded", http.StatusBadRequest)
			return
		}
		fh := r.MultipartForm.File["images"][0]
		src, err := fh.Open()
		if err != nil {
			http.Error(w, "cannot open upload", http.StatusBadRequest)
			return
		}
		defer src.Close()

		img, _, err := image.Decode(src)
		if err != nil {
			http.Error(w, "unsupported image", http.StatusBadRequest)
			return
		}

		square := cropCenterSquare(img)
		out := image.NewRGBA(image.Rect(0, 0, avatarSize, avatarSize))
		xdraw.CatmullRom.Scale(out, out.Bounds(), square, square.Bounds(), xdraw.Over, nil)

		dst, err := os.Create(target)
		if err != nil {
			http.Error(w, "cannot save avatar", http.StatusInternalServerError)
			return
		}
		defer dst.Close()
		if err := jpeg.Encode(dst, out, &jpeg.Options{Quality: 90}); err != nil {
			http.Error(w, "encode failed", http.StatusInternalServerError)
			return
		}
		_ = app.db.AuditLog("Avatar", sess.Username, "Avatar uploaded for "+userid)
		writeJSON(w, map[string]string{"status": "ok", "message": "avatar updated", "userid": userid})

	default:
		http.Error(w, "mode not set", http.StatusBadRequest)
	}
}

// cropCenterSquare returns the largest centered square sub-image of img.
func cropCenterSquare(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	side := w
	if h < side {
		side = h
	}
	x0 := b.Min.X + (w-side)/2
	y0 := b.Min.Y + (h-side)/2
	rect := image.Rect(x0, y0, x0+side, y0+side)

	if sub, ok := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}); ok {
		return sub.SubImage(rect)
	}
	// Fallback: copy the region.
	dst := image.NewRGBA(image.Rect(0, 0, side, side))
	xdraw.Draw(dst, dst.Bounds(), img, rect.Min, xdraw.Src)
	return dst
}
