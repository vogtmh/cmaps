# Map Editor (PoC)

A standalone proof-of-concept web app that loads a **vector floorplan PDF** into a
browser-based editor so you can strip out furniture, text, dimensions etc. and
export a clean **PNG** — replacing the manual Photoshop cleanup that used to take
hours per location.

It is a separate Go module (its own `go.mod`) so its cgo/PDF dependencies never
touch the main `companymaps` build.

## What it does

1. **Upload a PDF** — the first page is rendered to SVG (via MuPDF/go-fitz) and
   shown in an interactive canvas.
2. **Drop whole layers** — CAD exports (AutoCAD/Revit) carry named *Optional
   Content Groups* (layers) such as `int-built-in furniture`, `A_Möbel`,
   `_17.03 Txt Planung`. Uncheck them and hit **Apply** to re-render without them.
   This alone removes most furniture/text in seconds.
3. **Erase strays** — click a shape (Shift-click to add), or drag a box to
   select many, then **Delete**. **Ctrl+Z** undoes.
4. **Export PNG** — rasterizes the cleaned drawing at a chosen width, downloads
   it and saves a copy under `work/<id>/<name>.png`.

### Sample results (missing_plans/)

| File | Vector | Layers | Notes |
|------|--------|--------|-------|
| Berlin – 4th Floor.pdf | yes | **49** | furniture/text/people/walls all separated |
| OFFICE FLOOR PLAN – Singapore.pdf | yes | **15** | `int-built-in furniture`, `int-text`, … |
| Office Layout – Noida – 10th Floor.pdf | yes | **74** | fully layered |
| Office Layout – Tampa.pdf | yes | 0 | Bluebeam export, no layers → use the eraser |
| Mumbai / Tokyo / Seoul / Amman (.png) | no | — | raster; needs tracing (experimental, WIP) |

## Requirements

- **Go 1.25+** (pdfcpu requires it) with **cgo enabled** and a C compiler
  (Xcode CLT on macOS, `build-essential` on Linux). go-fitz bundles MuPDF, so no
  separate MuPDF install is needed.
- Optional (raster tracing, not finished): `potrace`.

## Run

```sh
cd mapeditor
go build -o mapeditor .
# macOS only: clear the stale signature so it isn't SIGKILL'd
codesign --force --sign - ./mapeditor 2>/dev/null || true
./mapeditor -addr :8099
```

Then open <http://localhost:8099>, click **Open PDF** (or drag a PDF in).

Flags: `-addr` (listen address, default `:8099`), `-work` (working dir, default
`work`).

## How it works

- `pdf.go` — go-fitz (MuPDF) renders a page to SVG and reports page count.
- `layers.go` — pdfcpu reads the `/OCProperties` optional-content groups and, to
  hide layers, adds their object refs to the `/D` config's `/OFF` array in a
  temporary copy of the PDF; MuPDF then honours that when rendering.
- `handlers.go` — `/api/upload`, `/api/svg` (gzip-streamed), `/api/layers`,
  `/api/export`.
- `static/` — the editor UI. Selection uses event delegation + the native
  `getIntersectionList` so it stays responsive even on very large drawings.

## Known limitations (it's a PoC)

- Big CAD files render to a lot of SVG (Tampa ≈ 200k paths / 37 MB). Drop layers
  first for smooth per-object editing; very large exports can hit the browser's
  canvas size limit — reduce the export width.
- Text is rendered as vector outlines (not editable text), which is fine for
  removal.
- Projects live in memory + `work/`; there is no auth, cleanup job, or
  multi-user support.
- Raster PNG → vector tracing is not implemented yet.
