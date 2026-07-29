// Command mapeditor is a standalone proof-of-concept web app that loads a vector
// floorplan PDF into a browser-based editor so furniture/text can be removed and
// the result exported as a clean PNG.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	addr := flag.String("addr", ":8099", "listen address")
	work := flag.String("work", "work", "working directory for uploads and exports")
	flag.Parse()

	if err := os.MkdirAll(*work, 0o755); err != nil {
		log.Fatalf("create work dir: %v", err)
	}
	absWork, err := filepath.Abs(*work)
	if err != nil {
		log.Fatalf("resolve work dir: %v", err)
	}

	srv := &server{workDir: absWork, projects: map[string]*project{}}

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handleIndex)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("/api/upload", srv.handleUpload)
	mux.HandleFunc("/api/svg", srv.handleSVG)
	mux.HandleFunc("/api/image", srv.handleImage)
	mux.HandleFunc("/api/layers", srv.handleLayers)
	mux.HandleFunc("/api/export", srv.handleExport)

	log.Printf("mapeditor listening on http://localhost%s (work dir: %s)", *addr, absWork)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
