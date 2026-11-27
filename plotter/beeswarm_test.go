// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter_test

import (
	"image/color"
	"math"
	"testing"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/cmpimg"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"
)

func TestBeeswarm(t *testing.T) {
	cmpimg.CheckPlot(ExampleBeeswarm, t, "beeswarm.png", "beeswarm.svg")
}

// TestBeeswarmEmpty ensures an empty beeswarm does not panic and
// draws nothing.
func TestBeeswarmEmpty(t *testing.T) {
	s, err := plotter.NewBeeswarm(plotter.XYs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p := plot.New()
	p.Add(s)

	img := vgimg.New(100, 100)
	c := draw.New(img)
	p.Draw(c)

	xmin, xmax, ymin, ymax := s.DataRange()
	if !math.IsInf(xmin, 1) || !math.IsInf(xmax, -1) ||
		!math.IsInf(ymin, 1) || !math.IsInf(ymax, -1) {
		t.Errorf("unexpected data range for empty input: %v %v %v %v", xmin, xmax, ymin, ymax)
	}
}

// TestBeeswarmNonFinite ensures points with NaN or infinite
// coordinates are ignored and do not affect the layout or the data
// range of the remaining points.
func TestBeeswarmNonFinite(t *testing.T) {
	xys := plotter.XYs{
		{X: 0, Y: 1},
		{X: 0, Y: math.NaN()},
		{X: math.Inf(1), Y: 2},
		{X: 0, Y: math.Inf(-1)},
		{X: math.NaN(), Y: 3},
		{X: 0, Y: 2},
	}
	s, err := plotter.NewBeeswarm(xys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	xmin, xmax, ymin, ymax := s.DataRange()
	if xmin != 0 || xmax != 0 || ymin != 1 || ymax != 2 {
		t.Errorf("unexpected data range: got %v %v %v %v, want 0 0 1 2", xmin, xmax, ymin, ymax)
	}

	p := plot.New()
	p.Add(s)
	img := vgimg.New(100, 100)
	c := draw.New(img)
	p.Draw(c) // must not panic
}

// TestBeeswarmZeroRadius ensures a zero glyph radius disables the
// horizontal spread: all points of a category are drawn at the
// category center.
func TestBeeswarmZeroRadius(t *testing.T) {
	xys := plotter.XYs{
		{X: 0, Y: 1},
		{X: 0, Y: 1},
		{X: 0, Y: 1},
	}
	s, err := plotter.NewBeeswarm(xys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.GlyphStyle.Radius = 0

	p := plot.New()
	p.Add(s)
	img := vgimg.New(100, 100)
	c := draw.New(img)
	p.Draw(c) // must not panic

	for i, b := range s.GlyphBoxes(p) {
		if want := p.X.Norm(0); b.X != want {
			t.Errorf("box %d: got x=%v, want category center %v", i, b.X, want)
		}
	}
}

// TestBeeswarmClip ensures points outside the canvas are not drawn
// and do not influence the layout of the visible points.
func TestBeeswarmClip(t *testing.T) {
	// Two visible points with the same value, plus a crowd of
	// points far outside the y axis range. If the out-of-range
	// points influenced the layout, the two visible points would
	// be pushed away from the center.
	xys := plotter.XYs{
		{X: 0, Y: 5},
		{X: 0, Y: 5},
	}
	for i := 0; i < 50; i++ {
		xys = append(xys, plotter.XY{X: 0, Y: 1e6 + float64(i)})
	}

	s, err := plotter.NewBeeswarm(xys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.GlyphStyle.Radius = vg.Points(2)

	p := plot.New()
	p.Add(s)
	p.Y.Min = 0
	p.Y.Max = 10
	img := vgimg.New(200, 200)
	c := draw.New(img)
	p.Draw(c)

	boxes := s.GlyphBoxes(p)
	if len(boxes) != 2 {
		t.Fatalf("got %d glyph boxes, want 2 visible points", len(boxes))
	}
	// The two visible points must sit at the center and one
	// diameter away from it; the clipped points must not have
	// pushed them further out.
	dc := p.DataCanvas(c)
	width := float64(dc.Max.X - dc.Min.X)
	center := p.X.Norm(0)
	d := float64(vg.Points(4)) / width // one diameter, normalized.
	offs := []float64{boxes[0].X - center, boxes[1].X - center}
	want := []float64{0, -d}
	for _, off := range offs {
		found := false
		for i, w := range want {
			if math.Abs(off-w) < 1e-9 {
				want = append(want[:i], want[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			t.Errorf("unexpected offset %v, want one of %v", off, want)
		}
	}
}

// TestBeeswarmDeterministicLayout ensures the same data drawn twice
// to same-sized canvases produces identical output.
func TestBeeswarmDeterministicLayout(t *testing.T) {
	xys := plotter.XYs{
		{X: 0, Y: 1}, {X: 0, Y: 1}, {X: 0, Y: 1},
		{X: 0, Y: 2}, {X: 0, Y: 2}, {X: 0, Y: 3},
		{X: 1, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 2},
	}

	render := func() []plot.GlyphBox {
		s, err := plotter.NewBeeswarm(xys)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		s.GlyphStyle.Radius = vg.Points(2)
		p := plot.New()
		p.Add(s)
		img := vgimg.New(200, 200)
		p.Draw(draw.New(img))
		return s.GlyphBoxes(p)
	}

	first := render()
	second := render()
	if len(first) != len(second) {
		t.Fatalf("box count mismatch: %d != %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("box %d differs between identical renders: %+v != %+v", i, first[i], second[i])
		}
	}
}

// TestBeeswarmGlyphStyleFunc ensures per-point styles are honored
// and indexed by the original data order.
func TestBeeswarmGlyphStyleFunc(t *testing.T) {
	xys := plotter.XYs{
		{X: 0, Y: 1},
		{X: 0, Y: 1},
	}
	s, err := plotter.NewBeeswarm(xys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.GlyphStyle = draw.GlyphStyle{
		Color:  color.Black,
		Radius: vg.Points(2),
		Shape:  draw.CircleGlyph{},
	}
	called := make(map[int]bool)
	s.GlyphStyleFunc = func(i int) draw.GlyphStyle {
		called[i] = true
		return s.GlyphStyle
	}

	p := plot.New()
	p.Add(s)
	img := vgimg.New(100, 100)
	p.Draw(draw.New(img))

	for i := range xys {
		if !called[i] {
			t.Errorf("GlyphStyleFunc not called for point %d", i)
		}
	}
}
