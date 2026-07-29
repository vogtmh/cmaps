package main

import (
	"os"
	"strings"
	"testing"
)

// TestLayers lists OCG layers and verifies that hiding some reduces the rendered
// element count. Set MAPEDITOR_SAMPLE_PDF to a layered PDF.
func TestLayers(t *testing.T) {
	path := os.Getenv("MAPEDITOR_SAMPLE_PDF")
	if path == "" {
		t.Skip("set MAPEDITOR_SAMPLE_PDF to run")
	}
	ls, err := pdfLayers(path)
	if err != nil {
		t.Fatalf("pdfLayers: %v", err)
	}
	t.Logf("found %d layers", len(ls))
	for _, l := range ls {
		t.Logf("  [%s] %s", l.Ref, l.Name)
	}
	if len(ls) == 0 {
		return
	}

	full, err := pdfPageSVG(path, 0, nil)
	if err != nil {
		t.Fatalf("render full: %v", err)
	}
	fullPaths := strings.Count(full, "<path")

	// Hide every layer and re-render; element count must drop.
	off := layerNames(ls)
	hidden, err := pdfPageSVG(path, 0, off)
	if err != nil {
		t.Fatalf("render hidden: %v", err)
	}
	hiddenPaths := strings.Count(hidden, "<path")
	t.Logf("paths: full=%d, all-layers-off=%d", fullPaths, hiddenPaths)
	if hiddenPaths >= fullPaths {
		t.Errorf("expected fewer paths with all layers off (full=%d hidden=%d)", fullPaths, hiddenPaths)
	}
}
