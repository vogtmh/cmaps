package main

import (
	"image/png"
	"os"
	"testing"

	fitz "github.com/gen2brain/go-fitz"
)

// TestRasterizeSVG renders an SVG file to PNG for visual inspection.
// MAPEDITOR_SVG=in.svg MAPEDITOR_PNG=out.png go test -run TestRasterizeSVG
func TestRasterizeSVG(t *testing.T) {
	in := os.Getenv("MAPEDITOR_SVG")
	out := os.Getenv("MAPEDITOR_PNG")
	if in == "" || out == "" {
		t.Skip("set MAPEDITOR_SVG and MAPEDITOR_PNG")
	}
	doc, err := fitz.New(in)
	if err != nil {
		t.Fatalf("open svg: %v", err)
	}
	defer doc.Close()
	img, err := doc.ImageDPI(0, 150)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s (%dx%d)", out, img.Bounds().Dx(), img.Bounds().Dy())
}
