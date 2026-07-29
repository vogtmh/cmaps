package main

import (
	"fmt"

	fitz "github.com/gen2brain/go-fitz"
)

// pdfNumPages returns the number of pages in the PDF.
func pdfNumPages(path string) (int, error) {
	doc, err := fitz.New(path)
	if err != nil {
		return 0, err
	}
	defer doc.Close()
	return doc.NumPage(), nil
}

// pdfPageSVG renders the given page (0-based) to an SVG string. When off is
// non-empty, the named optional-content layers are hidden before rendering.
func pdfPageSVG(path string, page int, off []string) (string, error) {
	renderPath := path
	if len(off) > 0 {
		modified, cleanup, err := applyLayerVisibility(path, off)
		if err != nil {
			return "", fmt.Errorf("apply layer visibility: %w", err)
		}
		defer cleanup()
		renderPath = modified
	}

	doc, err := fitz.New(renderPath)
	if err != nil {
		return "", err
	}
	defer doc.Close()

	if page < 0 || page >= doc.NumPage() {
		page = 0
	}
	svg, err := doc.SVG(page)
	if err != nil {
		return "", err
	}
	return svg, nil
}
