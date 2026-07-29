package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// layer represents one optional-content group (OCG) in a PDF.
type layer struct {
	Name string `json:"name"`
	Ref  string `json:"ref"` // "obj gen", e.g. "14 0"
}

func layerNames(ls []layer) []string {
	names := make([]string, 0, len(ls))
	for _, l := range ls {
		names = append(names, l.Name)
	}
	return names
}

type ocg struct {
	name string
	ref  types.IndirectRef
}

// ocProperties returns the /D config dict and the list of OCGs (with names).
func ocProperties(xt *model.XRefTable) (types.Dict, []ocg, error) {
	cat, err := xt.Catalog()
	if err != nil {
		return nil, nil, err
	}
	ocp, err := xt.DereferenceDict(cat["OCProperties"])
	if err != nil || ocp == nil {
		return nil, nil, nil // no optional content
	}
	arr, err := xt.DereferenceArray(ocp["OCGs"])
	if err != nil {
		return nil, nil, err
	}
	var ocgs []ocg
	for _, e := range arr {
		ref, ok := asIndirectRef(e)
		if !ok {
			continue
		}
		d, err := xt.DereferenceDict(ref)
		if err != nil || d == nil {
			continue
		}
		ocgs = append(ocgs, ocg{name: decodePDFString(d["Name"]), ref: ref})
	}
	dConfig, _ := xt.DereferenceDict(ocp["D"])
	return dConfig, ocgs, nil
}

// pdfLayers lists the optional-content groups (layers) in a PDF.
func pdfLayers(path string) ([]layer, error) {
	ctx, err := api.ReadContextFile(path)
	if err != nil {
		return nil, err
	}
	_, ocgs, err := ocProperties(ctx.XRefTable)
	if err != nil {
		return nil, err
	}
	out := make([]layer, 0, len(ocgs))
	for _, g := range ocgs {
		name := g.name
		if name == "" {
			name = fmt.Sprintf("(unnamed %d)", g.ref.ObjectNumber.Value())
		}
		out = append(out, layer{
			Name: name,
			Ref:  fmt.Sprintf("%d %d", g.ref.ObjectNumber.Value(), g.ref.GenerationNumber.Value()),
		})
	}
	return out, nil
}

// applyLayerVisibility writes a copy of the PDF with the named layers hidden
// (added to the /D config's /OFF array) and returns the temp path + cleanup.
func applyLayerVisibility(path string, off []string) (string, func(), error) {
	noop := func() {}
	offSet := map[string]bool{}
	for _, n := range off {
		offSet[n] = true
	}

	ctx, err := api.ReadContextFile(path)
	if err != nil {
		return "", noop, err
	}
	dConfig, ocgs, err := ocProperties(ctx.XRefTable)
	if err != nil {
		return "", noop, err
	}
	if dConfig == nil {
		// No optional content / default config: nothing to hide.
		return path, noop, nil
	}

	// Existing OFF entries (by object number) so we don't duplicate.
	offArr, _ := ctx.XRefTable.DereferenceArray(dConfig["OFF"])
	present := map[int]bool{}
	for _, e := range offArr {
		if ref, ok := asIndirectRef(e); ok {
			present[ref.ObjectNumber.Value()] = true
		}
	}
	for _, g := range ocgs {
		if offSet[g.name] && !present[g.ref.ObjectNumber.Value()] {
			offArr = append(offArr, g.ref)
			present[g.ref.ObjectNumber.Value()] = true
		}
	}
	dConfig["OFF"] = offArr

	tmp, err := os.CreateTemp(filepath.Dir(path), "layers-*.pdf")
	if err != nil {
		return "", noop, err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if err := api.WriteContextFile(ctx, tmpPath); err != nil {
		cleanup()
		return "", noop, err
	}
	return tmpPath, cleanup, nil
}

func asIndirectRef(o types.Object) (types.IndirectRef, bool) {
	switch v := o.(type) {
	case types.IndirectRef:
		return v, true
	case *types.IndirectRef:
		return *v, true
	}
	return types.IndirectRef{}, false
}

func decodePDFString(o types.Object) string {
	switch s := o.(type) {
	case types.StringLiteral:
		if r, err := types.StringLiteralToString(s); err == nil {
			return r
		}
		return string(s)
	case types.HexLiteral:
		if r, err := types.HexLiteralToString(s); err == nil {
			return r
		}
		return string(s)
	}
	return ""
}
