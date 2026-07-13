package web

import (
	"embed"
	"os"
	"strconv"
	"time"
)

// Embedded assets. go:embed paths are package-relative, so these directives
// live here next to templates/, static/ and sample/.

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// sampleFS holds the bundled demo maps and avatars used by the "set up a new
// server" wizard path (Phase 2).
//
//go:embed sample
var sampleFS embed.FS

// assetVersion is appended as a ?v= query to versioned assets (JS/CSS) so a new
// deployment busts the browser cache. It is derived from the running binary's
// modification time, which changes on every deploy/rebuild.
var assetVersion = computeAssetVersion()

func computeAssetVersion() string {
	if exe, err := os.Executable(); err == nil {
		if fi, err := os.Stat(exe); err == nil {
			return strconv.FormatInt(fi.ModTime().Unix(), 36)
		}
	}
	return strconv.FormatInt(time.Now().Unix(), 36)
}
