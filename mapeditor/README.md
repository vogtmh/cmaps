# Map Editor (PoC)

A standalone proof-of-concept web app that loads a **vector floorplan PDF** into a
browser-based editor so you can strip out furniture, text, dimensions etc. and
export a clean **PNG** — replacing the manual Photoshop cleanup that used to take
hours per location.

It is a separate Go module (its own `go.mod`) so its cgo/PDF dependencies never
touch the main `companymaps` build.

## What it does

**Vector PDFs** — edited as vectors:

1. **Upload a PDF** — the first page is rendered to SVG (via MuPDF/go-fitz) and
   shown in an interactive canvas.
2. **Toggle whole layers** — CAD exports (AutoCAD/Revit) carry named *Optional
   Content Groups* (layers) such as `int-built-in furniture`, `A_Möbel`,
   `_17.03 Txt Planung`. Toggle them off (they hide immediately) to remove most
   furniture/text in seconds. Hovering a layer highlights it on the map.
3. **Erase strays** — click a shape (Shift-click to add), or drag a box to
   select everything inside it (Alt+drag also grabs what it touches), then
   **Delete**. **Ctrl+Z** undoes. A grey mask shows the margins that export trims.
4. **Export PNG** — rasterizes the cleaned drawing at a chosen width, downloads
   it and saves a copy under `work/<id>/<name>.png`.

**Raster images (PNG/JPG)** — edited as pixels (tracing degrades low-res scans):

1. **Upload an image** — shown at full resolution on a canvas.
2. **Erase furniture/text** — **Rectangle** or **Brush** paints over details with
   a chosen background colour; an **eyedropper** matches the sheet colour.
   **Ctrl+Z** undoes.
3. **Export PNG** — saves the edited image at native resolution.

### Sample results (missing_plans/)

| File | Vector | Layers | Notes |
|------|--------|--------|-------|
| Berlin – 4th Floor.pdf | yes | **49** | furniture/text/people/walls all separated |
| OFFICE FLOOR PLAN – Singapore.pdf | yes | **15** | `int-built-in furniture`, `int-text`, … |
| Office Layout – Noida – 10th Floor.pdf | yes | **74** | fully layered |
| Office Layout – Tampa.pdf | yes | 0 | Bluebeam export, no layers → use the eraser |
| Mumbai / Tokyo / Seoul / Amman (.png) | raster | — | edited as pixels with the erase tools (rectangle/brush) |

## Requirements

- **Go 1.25+** (pdfcpu requires it) with **cgo enabled** and a C compiler
  (Xcode CLT on macOS, `build-essential` on Linux). go-fitz bundles MuPDF, so no
  separate MuPDF install is needed.

## Run

```sh
cd mapeditor
go build -o mapeditor .
# macOS only: clear the stale signature so it isn't SIGKILL'd
codesign --force --sign - ./mapeditor 2>/dev/null || true
./mapeditor -addr :8099
```

Then open <http://localhost:8099>, click **Open file** (or drag a PDF/image in).

Flags: `-addr` (listen address, default `:8099`), `-work` (working dir, default
`work`).

## How it works

- `pdf.go` — go-fitz (MuPDF) renders a page to SVG and reports page count.
- `layers.go` — pdfcpu reads the `/OCProperties` optional-content groups and, to
  hide layers, adds their object refs to the `/D` config's `/OFF` array in a
  temporary copy of the PDF; MuPDF then honours that when rendering.
- `handlers.go` — `/api/upload`, `/api/svg` (gzip-streamed), `/api/layers`,
  `/api/image` (raster source), `/api/export`.
- `static/` — the editor UI. Vector selection uses event delegation + the native
  `getIntersectionList`; raster editing paints onto a `<canvas>`.

## Known limitations (it's a PoC)

- Big CAD files render to a lot of SVG (Tampa ≈ 200k paths / 37 MB). Drop layers
  first for smooth per-object editing; very large exports can hit the browser's
  canvas size limit — reduce the export width.
- Text is rendered as vector outlines (not editable text), which is fine for
  removal.
- Projects live in memory + `work/`; there is no auth, cleanup job, or
  multi-user support.
- Raster tracing (vtracer) produces no layers and outlines strokes on both sides
  (no centerline), so those maps need manual cleanup with the eraser.
