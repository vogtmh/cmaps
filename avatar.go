package main

import (
	"bytes"
	"encoding/binary"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	xdraw "golang.org/x/image/draw"
)

const avatarSize = 400

// handleRestAvatar serves /rest/avatar (POST mode=upload|delete). The avatar is
// center-cropped to a square and stored as avatarcache/<userid>.jpg, where userid
// is the part of the session username after the domain backslash.
//
// Cropping and rotation happen client-side (Cropper.js): the browser uploads a
// finished square JPEG, so the server-side center-crop is normally a no-op safety net.
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
		_ = app.db.SetLdapAvatar(userid, false)
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

		data, err := io.ReadAll(src)
		if err != nil {
			http.Error(w, "cannot read upload", http.StatusBadRequest)
			return
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			http.Error(w, "unsupported image", http.StatusBadRequest)
			return
		}
		// Bake any EXIF orientation into the pixels so the stored file is always
		// upright (browsers auto-rotate <img> via EXIF, but the map renders the
		// avatar as a CSS background-image which ignores it -> sideways faces).
		img = applyOrientation(img, exifOrientation(data))

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
		_ = app.db.SetLdapAvatar(userid, true)
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

// orientationLabel returns a human-readable description of an EXIF orientation.
func orientationLabel(o int) string {
	switch o {
	case 2:
		return "Mirrored horizontally"
	case 3:
		return "Rotated 180\u00b0"
	case 4:
		return "Mirrored vertically"
	case 5:
		return "Mirrored + rotated 90\u00b0 CW"
	case 6:
		return "Rotated 90\u00b0 CW"
	case 7:
		return "Mirrored + rotated 90\u00b0 CCW"
	case 8:
		return "Rotated 90\u00b0 CCW"
	default:
		return "Normal"
	}
}

// handleRestAvatarOrientation powers the admin "Avatar orientation" tool.
//
// Most cached avatars are already-cropped squares with no EXIF metadata, so a
// sideways image cannot be detected automatically. The tool therefore lists every
// avatar for manual visual review and rotates only the ones the admin picks.
//   - mode=list:  return every cached avatar userid (plus which still carry an
//     EXIF orientation tag, shown as a hint) for the review grid.
//   - mode=apply: rotate the given avatars clockwise by the requested angle and
//     re-save them. Body param "rotations" = "userid:deg,userid:deg" (deg in 90/180/270).
//
// It requires config permission level 2 (same as the other config-tab tools).
func (app *App) handleRestAvatarOrientation(w http.ResponseWriter, r *http.Request) {
	sess, ok := app.currentSession(r)
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	if app.permLevel(sess, "config") < 2 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	mode := r.FormValue("mode")
	dir := app.cfg.dataPath("avatarcache")

	switch mode {
	case "list":
		entries, err := os.ReadDir(dir)
		if err != nil {
			writeJSON(w, map[string]interface{}{"ok": false, "message": err.Error()})
			return
		}
		type item struct {
			Userid  string `json:"userid"`
			HasExif bool   `json:"hasexif"`
		}
		items := []item{}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".jpg") {
				continue
			}
			id := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			hasExif := false
			if data, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil {
				hasExif = exifOrientation(data) > 1
			}
			items = append(items, item{Userid: id, HasExif: hasExif})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Userid < items[j].Userid })
		writeJSON(w, map[string]interface{}{"ok": true, "count": len(items), "items": items})

	case "apply":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		applied, failed := 0, 0
		for _, raw := range strings.Split(r.FormValue("rotations"), ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			parts := strings.SplitN(raw, ":", 2)
			if len(parts) != 2 {
				continue
			}
			id := safeAvatarID(parts[0])
			deg := parts[1]
			if id == "" || (deg != "90" && deg != "180" && deg != "270") {
				continue
			}
			path := filepath.Join(dir, id+".jpg")
			data, err := os.ReadFile(path)
			if err != nil {
				failed++
				continue
			}
			img, _, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				failed++
				continue
			}
			switch deg {
			case "90":
				img = rotate90(img)
			case "180":
				img = rotate180(img)
			case "270":
				img = rotate270(img)
			}
			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
				failed++
				continue
			}
			if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
				failed++
				continue
			}
			applied++
		}
		_ = app.db.AuditLog("Avatar", sess.Username, "Rotated "+itoa(applied)+" avatar(s)")
		writeJSON(w, map[string]interface{}{"ok": true, "applied": applied, "failed": failed,
			"message": itoa(applied) + " avatar(s) rotated" + func() string {
				if failed > 0 {
					return ", " + itoa(failed) + " failed"
				}
				return ""
			}()})

	default:
		http.Error(w, "mode not set", http.StatusBadRequest)
	}
}

// safeAvatarID sanitises a userid used to build an avatarcache path, rejecting
// anything with path separators or traversal so we only ever touch that folder.
func safeAvatarID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, "/\\") || strings.Contains(s, "..") {
		return ""
	}
	return s
}

// exifOrientation extracts the EXIF Orientation tag (0x0112) from a JPEG byte
// slice. It returns 1 (normal) when the tag is absent or the data can't be parsed.
func exifOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+2 <= len(data) {
		if data[i] != 0xFF {
			i++
			continue
		}
		marker := data[i+1]
		// Padding byte or standalone markers carry no length field.
		if marker == 0xFF {
			i++
			continue
		}
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD9) {
			i += 2
			continue
		}
		if i+4 > len(data) {
			break
		}
		segLen := int(data[i+2])<<8 | int(data[i+3])
		if segLen < 2 || i+2+segLen > len(data) {
			break
		}
		if marker == 0xE1 { // APP1 (Exif)
			if o, found := parseExifOrientation(data[i+4 : i+2+segLen]); found {
				return o
			}
		}
		if marker == 0xDA { // start of scan: image data follows, stop.
			break
		}
		i += 2 + segLen
	}
	return 1
}

// parseExifOrientation reads the Orientation tag out of an APP1 segment payload
// (the bytes after the marker+length, starting with "Exif\0\0").
func parseExifOrientation(seg []byte) (int, bool) {
	if len(seg) < 14 || string(seg[0:4]) != "Exif" {
		return 0, false
	}
	tiff := seg[6:] // skip "Exif\0\0"
	if len(tiff) < 8 {
		return 0, false
	}
	var bo binary.ByteOrder
	switch string(tiff[0:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 0, false
	}
	ifd := int(bo.Uint32(tiff[4:8]))
	if ifd < 0 || ifd+2 > len(tiff) {
		return 0, false
	}
	n := int(bo.Uint16(tiff[ifd : ifd+2]))
	for k := 0; k < n; k++ {
		off := ifd + 2 + k*12
		if off+12 > len(tiff) {
			break
		}
		if bo.Uint16(tiff[off:off+2]) == 0x0112 { // Orientation, type SHORT
			v := int(bo.Uint16(tiff[off+8 : off+10]))
			if v >= 1 && v <= 8 {
				return v, true
			}
			return 1, true
		}
	}
	return 0, false
}

// applyOrientation returns img transformed so that the given EXIF orientation
// value renders upright. Orientation 1 (or unknown) returns img unchanged.
func applyOrientation(img image.Image, o int) image.Image {
	switch o {
	case 2:
		return flipHorizontal(img)
	case 3:
		return rotate180(img)
	case 4:
		return flipVertical(img)
	case 5:
		return transpose(img)
	case 6:
		return rotate90(img)
	case 7:
		return transverse(img)
	case 8:
		return rotate270(img)
	default:
		return img
	}
}

// rotate90 rotates img 90° clockwise.
func rotate90(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(h-1-y, x, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// rotate270 rotates img 90° counter-clockwise.
func rotate270(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(y, w-1-x, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// rotate180 rotates img by 180°.
func rotate180(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(w-1-x, h-1-y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// flipHorizontal mirrors img left-to-right.
func flipHorizontal(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(w-1-x, y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// flipVertical mirrors img top-to-bottom.
func flipVertical(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, h-1-y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// transpose flips img across its main diagonal (EXIF orientation 5).
func transpose(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(y, x, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// transverse flips img across its anti-diagonal (EXIF orientation 7).
func transverse(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(h-1-y, w-1-x, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
