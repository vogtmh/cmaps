package main

import (
	"os"
	"strings"
	"testing"
)

// TestRenderSampleSVG renders a real sample PDF to SVG to validate go-fitz output.
// Set MAPEDITOR_SAMPLE_PDF to a PDF path; otherwise the test is skipped.
func TestRenderSampleSVG(t *testing.T) {
	path := os.Getenv("MAPEDITOR_SAMPLE_PDF")
	if path == "" {
		t.Skip("set MAPEDITOR_SAMPLE_PDF to run")
	}
	n, err := pdfNumPages(path)
	if err != nil {
		t.Fatalf("numpages: %v", err)
	}
	t.Logf("pages: %d", n)

	svg, err := pdfPageSVG(path, 0, nil)
	if err != nil {
		t.Fatalf("svg: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Fatalf("output does not look like svg (len=%d): %.120q", len(svg), svg)
	}
	t.Logf("svg length: %d bytes", len(svg))
	t.Logf("path elements: %d, image elements: %d, text elements: %d",
		strings.Count(svg, "<path"), strings.Count(svg, "<image"), strings.Count(svg, "<text"))

	if out := os.Getenv("MAPEDITOR_SAMPLE_OUT"); out != "" {
		if err := os.WriteFile(out, []byte(svg), 0o644); err != nil {
			t.Fatalf("write out: %v", err)
		}
		t.Logf("wrote %s", out)
	}
}
