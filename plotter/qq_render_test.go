// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter_test

import (
	"bytes"
	"image"
	"math"
	"regexp"
	"strconv"
	"testing"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"
)

// ringPath matches the SVG path emitted for a ring glyph: a move to
// the rightmost point of the ring followed by an arc with the same
// radius. The radius is captured because other tests in the package
// mutate the global default glyph style.
var ringPath = regexp.MustCompile(`M([0-9.eE+-]+),([0-9.eE+-]+)A([0-9.]+),[0-9.]+`)

// TestQQRenderedCoordinates verifies, at both the SVG and PNG output
// levels, that the Q-Q points of [0,1,2,3] against [10,11,12,13] are
// drawn at the paired quantiles (k, k+10) and therefore lie on
// y = x + 10. The expected quantiles are the hand-computed order
// statistics at probabilities 0, 1/3, 2/3 and 1.
func TestQQRenderedCoordinates(t *testing.T) {
	const size = 4 * vg.Inch

	q, err := plotter.NewQQ(
		plotter.Values{0, 1, 2, 3},
		plotter.Values{10, 11, 12, 13},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Pin the glyph style so the test does not depend on global
	// defaults that other tests in the package may mutate.
	q.GlyphStyle.Shape = draw.RingGlyph{}
	q.GlyphStyle.Radius = vg.Points(2.5)

	p := plot.New()
	p.X.Min, p.X.Max = 0, 13
	p.Y.Min, p.Y.Max = 0, 13
	p.X.Padding, p.Y.Padding = 0, 0
	p.Add(q)

	// Expected glyph positions in canvas (vg.Length) coordinates,
	// obtained through the same cropped data canvas the plotter uses.
	geom := draw.New(vgimg.New(size, size))
	dataDC := p.DataCanvas(geom)
	trX, trY := p.Transforms(&dataDC)
	type pt struct{ x, y float64 }
	want := make([]pt, q.Len())
	for i := range q.Len() {
		x, y := q.XY(i)
		want[i] = pt{x: trX(x).Points(), y: trY(y).Points()}
		if y != x+10 {
			t.Errorf("point %d: (%.3g, %.3g) is not on y = x + 10", i, x, y)
		}
	}

	// SVG: every ring glyph appears as a circle whose path starts at
	// its rightmost point, so the center is (mx - r, my). Circles are
	// emitted in drawing order, which is the point order.
	svgWT, err := p.WriterTo(size, size, "svg")
	if err != nil {
		t.Fatalf("failed to build SVG: %v", err)
	}
	var svg bytes.Buffer
	if _, err := svgWT.WriteTo(&svg); err != nil {
		t.Fatalf("failed to write SVG: %v", err)
	}
	matches := ringPath.FindAllStringSubmatch(svg.String(), -1)
	if len(matches) < len(want) {
		t.Fatalf("found %d ring glyphs in SVG, want at least %d", len(matches), len(want))
	}
	next := 0
	for _, m := range matches {
		r := atof(t, m[3])
		cx := atof(t, m[1]) - r
		cy := atof(t, m[2])
		if next >= len(want) {
			break
		}
		if math.Abs(cx-want[next].x) > 0.02 || math.Abs(cy-want[next].y) > 0.02 {
			// A ring outside the data area (for example a legend
			// thumbnail); keep looking for the expected one.
			continue
		}
		next++
	}
	if next != len(want) {
		t.Errorf("SVG contains only %d/%d expected glyphs in order", next, len(want))
	}

	// PNG: decode the raster output and confirm an inked (non-white)
	// pixel near each expected glyph center. vg uses y-up coordinates;
	// image rows are top-down.
	pngWT, err := p.WriterTo(size, size, "png")
	if err != nil {
		t.Fatalf("failed to build PNG: %v", err)
	}
	var raw bytes.Buffer
	if _, err := pngWT.WriteTo(&raw); err != nil {
		t.Fatalf("failed to write PNG: %v", err)
	}
	img, _, err := image.Decode(&raw)
	if err != nil {
		t.Fatalf("failed to decode PNG: %v", err)
	}
	bounds := img.Bounds()
	const dpi = 96.0 / 72.0 // vgimg renders at 96 DPI; lengths are points.
	px := func(c pt) (int, int) {
		return int(math.Round(c.x * dpi)), bounds.Max.Y - 1 - int(math.Round(c.y*dpi))
	}
	for i, c := range want {
		x, y := px(c)
		// A ring glyph is stroked (not filled), so the ink is on
		// the circumference, not at the center.
		if !inkedNear(img, x, y, 5) {
			t.Errorf("point %d: no inked pixel near (%d, %d)", i, x, y)
		}
	}
}

// inkedNear reports whether a non-white pixel is within radius r of
// (x, y).
func inkedNear(img image.Image, x, y, r int) bool {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			rr, gg, bb, _ := img.At(x+dx, y+dy).RGBA()
			if rr|gg|bb != 0xffff {
				return true
			}
		}
	}
	return false
}

func atof(t *testing.T, s string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("bad float %q: %v", s, err)
	}
	return v
}
