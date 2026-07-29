package main

import (
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// project holds the state for one uploaded file.
type project struct {
	ID       string
	Kind     string // "pdf" or "png"
	SrcPath  string // original uploaded file
	Filename string
	NumPages int
	Layers   []layer  // OCG layers (PDFs only)
	Off      []string // layer names currently hidden
}

// server is the HTTP application.
type server struct {
	workDir  string
	mu       sync.Mutex
	projects map[string]*project
}

func (s *server) getProject(id string) *project {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.projects[id]
}

func (s *server) putProject(p *project) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.projects[p.ID] = p
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func httpErr(w http.ResponseWriter, code int, msg string) {
	http.Error(w, msg, code)
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join("static", "index.html"))
}

// handleUpload accepts a multipart "file" (PDF or PNG), stores it, renders the
// first page to SVG and returns project metadata + the SVG markup.
func (s *server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpErr(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		httpErr(w, http.StatusBadRequest, "parse form: "+err.Error())
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		httpErr(w, http.StatusBadRequest, "missing file: "+err.Error())
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	kind := ""
	switch ext {
	case ".pdf":
		kind = "pdf"
	case ".png", ".jpg", ".jpeg":
		kind = "png"
	default:
		httpErr(w, http.StatusBadRequest, "unsupported file type: "+ext)
		return
	}

	id := newID()
	dir := filepath.Join(s.workDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		httpErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	srcPath := filepath.Join(dir, "source"+ext)
	dst, err := os.Create(srcPath)
	if err != nil {
		httpErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := io.Copy(dst, file); err != nil {
		dst.Close()
		httpErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	dst.Close()

	p := &project{ID: id, Kind: kind, SrcPath: srcPath, Filename: hdr.Filename}

	if kind != "pdf" {
		httpErr(w, http.StatusBadRequest, "raster vectorization is not implemented yet (PDF only for now)")
		return
	}

	pages, err := pdfNumPages(srcPath)
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "open pdf: "+err.Error())
		return
	}
	p.NumPages = pages

	if layers, err := pdfLayers(srcPath); err == nil {
		p.Layers = layers
	}

	s.putProject(p)

	writeJSON(w, map[string]any{
		"id":       p.ID,
		"kind":     p.Kind,
		"filename": p.Filename,
		"pages":    p.NumPages,
		"page":     0,
		"layers":   layerNames(p.Layers),
	})
}

// handleSVG renders and streams the SVG for a page, honoring the project's
// current hidden-layer set. The response is gzip-encoded when accepted.
func (s *server) handleSVG(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	p := s.getProject(id)
	if p == nil {
		httpErr(w, http.StatusNotFound, "unknown project")
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 0 || page >= p.NumPages {
		page = 0
	}

	var off []string
	if only := r.URL.Query().Get("only"); only != "" {
		// Isolate a single layer: hide every other layer (used for hover preview).
		for _, l := range p.Layers {
			if l.Name != only {
				off = append(off, l.Name)
			}
		}
	} else {
		s.mu.Lock()
		off = append([]string(nil), p.Off...)
		s.mu.Unlock()
	}

	svg, err := pdfPageSVG(p.SrcPath, page, off)
	if err != nil {
		httpErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		_, _ = io.WriteString(gz, svg)
		return
	}
	_, _ = io.WriteString(w, svg)
}

// handleLayers re-renders the current page with the requested layers turned off.
func (s *server) handleLayers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpErr(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req struct {
		ID  string   `json:"id"`
		Off []string `json:"off"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpErr(w, http.StatusBadRequest, err.Error())
		return
	}
	p := s.getProject(req.ID)
	if p == nil {
		httpErr(w, http.StatusNotFound, "unknown project")
		return
	}
	s.mu.Lock()
	p.Off = req.Off
	s.mu.Unlock()
	writeJSON(w, map[string]any{"ok": true})
}

// handleExport receives a rasterized PNG from the browser and saves it.
func (s *server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpErr(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		httpErr(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.FormValue("id")
	p := s.getProject(id)
	if p == nil {
		httpErr(w, http.StatusNotFound, "unknown project")
		return
	}
	name := sanitizeName(r.FormValue("name"))
	if name == "" {
		name = "map"
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		httpErr(w, http.StatusBadRequest, "missing image: "+err.Error())
		return
	}
	defer file.Close()

	outPath := filepath.Join(s.workDir, id, name+".png")
	out, err := os.Create(outPath)
	if err != nil {
		httpErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		httpErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out.Close()

	writeJSON(w, map[string]any{"ok": true, "path": outPath})
	fmt.Printf("exported %s\n", outPath)
}

func sanitizeName(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	return b.String()
}
