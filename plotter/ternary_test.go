// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"errors"
	"image/color"
	"math"
	"testing"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/recorder"
)

const eqTol = 1e-12

func approxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func TestCopyABCs(t *testing.T) {
	// Normalization: components are divided by their total.
	got, err := CopyABCs(ABCs{{1, 1, 1}, {2, 0, 2}, {3, 6, 9}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := ABCs{
		{1.0 / 3, 1.0 / 3, 1.0 / 3},
		{0.5, 0, 0.5},
		{3.0 / 18, 6.0 / 18, 9.0 / 18},
	}
	for i := range want {
		for j, v := range [3]float64{got[i].A, got[i].B, got[i].C} {
			w := [3]float64{want[i].A, want[i].B, want[i].C}[j]
			if !approxEqual(v, w, eqTol) {
				t.Errorf("point %d component %d: got %v, want %v", i, j, v, w)
			}
		}
		if sum := got[i].A + got[i].B + got[i].C; !approxEqual(sum, 1, eqTol) {
			t.Errorf("point %d does not sum to one: %v", i, sum)
		}
	}
}

func TestCopyABCsErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		data ABCer
		want error
	}{
		{"empty", ABCs{}, ErrNoData},
		{"negative A", ABCs{{-1, 1, 1}}, ErrNegativeComponent},
		{"negative B", ABCs{{1, -1, 1}}, ErrNegativeComponent},
		{"negative C", ABCs{{1, 1, -1}}, ErrNegativeComponent},
		{"zero total", ABCs{{0, 0, 0}}, ErrZeroTotal},
		{"NaN", ABCs{{math.NaN(), 1, 1}}, ErrNaN},
		{"+Inf", ABCs{{math.Inf(1), 1, 1}}, ErrInfinity},
		{"-Inf", ABCs{{1, math.Inf(-1), 1}}, ErrInfinity},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := CopyABCs(test.data)
			if !errors.Is(err, test.want) {
				t.Fatalf("got error %v, want %v", err, test.want)
			}
		})
	}
}

func TestTernaryTicker(t *testing.T) {
	ticks := TernaryTicker{Steps: 4}.Ticks(0, 1)
	if len(ticks) != 5 {
		t.Fatalf("got %d ticks, want 5", len(ticks))
	}
	for i, tk := range ticks {
		wantV := float64(i) / 4
		if !approxEqual(tk.Value, wantV, eqTol) {
			t.Errorf("tick %d: got value %v, want %v", i, tk.Value, wantV)
		}
		if tk.Label == "" {
			t.Errorf("tick %d has no label", i)
		}
		if tk.Value < 0 || tk.Value > 1 {
			t.Errorf("tick %d outside [0,1]: %v", i, tk.Value)
		}
	}
	// Non-positive Steps falls back to 4.
	if len(TernaryTicker{Steps: 0}.Ticks(0, 1)) != 5 {
		t.Error("zero Steps did not fall back to 4")
	}
}

func TestFitTernary(t *testing.T) {
	// A square 100×100 area: the triangle side is width-limited.
	c := draw.NewCanvas(new(recorder.Canvas), 100, 100)
	g := fitTernary(c)
	if !g.ok() {
		t.Fatal("expected a valid geometry")
	}
	if !approxEqual(float64(g.scale), 100, eqTol) {
		t.Errorf("square area: got side %v, want 100", g.scale)
	}
	// Centered in both dimensions: the width-limited triangle leaves
	// equal vertical margins above and below it.
	wantOX := vg.Length(0)
	wantOY := (vg.Length(100) - 100*ternaryHeight) / 2
	if g.origin.X != wantOX || !approxEqual(float64(g.origin.Y), float64(wantOY), eqTol) {
		t.Errorf("square area origin: got (%v,%v), want (%v,%v)", g.origin.X, g.origin.Y, wantOX, wantOY)
	}
	// The apex A must sit at the horizontal midpoint of the B-C base,
	// which is what makes the triangle equilateral rather than right.
	wantApexX := (g.vb.X + g.vc.X) / 2
	if !approxEqual(float64(g.va.X), float64(wantApexX), eqTol) {
		t.Errorf("apex not centered: A.X = %v, want %v", g.va.X, wantApexX)
	}
	// All three sides must be equal.
	side := float64(g.vc.X - g.vb.X)
	dAB := math.Hypot(float64(g.va.X-g.vb.X), float64(g.va.Y-g.vb.Y))
	dAC := math.Hypot(float64(g.va.X-g.vc.X), float64(g.va.Y-g.vc.Y))
	if !approxEqual(dAB, side, eqTol) || !approxEqual(dAC, side, eqTol) {
		t.Errorf("not equilateral: base %v, sides %v %v", side, dAB, dAC)
	}

	// A wide, short area: the side is height-limited.
	c = draw.NewCanvas(new(recorder.Canvas), 400, 86.60254037844386)
	g = fitTernary(c)
	if !g.ok() {
		t.Fatal("expected a valid geometry for wide area")
	}
	wantSide := vg.Length(86.60254037844386) / ternaryHeight
	if !approxEqual(float64(g.scale), float64(wantSide), eqTol) {
		t.Errorf("wide area: got side %v, want %v", g.scale, wantSide)
	}
	// The triangle must be centered in the otherwise empty width.
	triW := g.scale
	wantX := (400 - triW) / 2
	if !approxEqual(float64(g.origin.X), float64(wantX), eqTol) {
		t.Errorf("wide area not centered: got origin X %v, want %v", g.origin.X, wantX)
	}
}

func TestFitTernaryDegenerate(t *testing.T) {
	for _, size := range [][2]vg.Length{
		{0, 100},
		{100, 0},
		{0, 0},
		{-5, 100},
	} {
		c := draw.NewCanvas(new(recorder.Canvas), size[0], size[1])
		if g := fitTernary(c); g.ok() {
			t.Errorf("size %v: expected degenerate geometry, got scale %v", size, g.scale)
		}
	}
}

func TestTernaryProjectionVerticesAndCentroid(t *testing.T) {
	c := draw.NewCanvas(new(recorder.Canvas), 100, 100)
	g := fitTernary(c)

	for _, test := range []struct {
		name         string
		a, b, c      float64
		wantX, wantY vg.Length
	}{
		{"A vertex", 1, 0, 0, g.va.X, g.va.Y},
		{"B vertex", 0, 1, 0, g.vb.X, g.vb.Y},
		{"C vertex", 0, 0, 1, g.vc.X, g.vc.Y},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := g.project(test.a, test.b, test.c)
			if p != (vg.Point{X: test.wantX, Y: test.wantY}) {
				t.Errorf("got %v, want %v", p, vg.Point{X: test.wantX, Y: test.wantY})
			}
		})
	}

	// The (1,1,1) mixture is the centroid: average of the three vertices.
	centroid := g.project(1.0/3, 1.0/3, 1.0/3)
	want := vg.Point{
		X: (g.va.X + g.vb.X + g.vc.X) / 3,
		Y: (g.va.Y + g.vb.Y + g.vc.Y) / 3,
	}
	if !approxEqual(float64(centroid.X), float64(want.X), eqTol) ||
		!approxEqual(float64(centroid.Y), float64(want.Y), eqTol) {
		t.Errorf("centroid: got %v, want %v", centroid, want)
	}
}

func TestTernaryProjectionThirds(t *testing.T) {
	c := draw.NewCanvas(new(recorder.Canvas), 100, 100)
	g := fitTernary(c)

	// Two-thirds mixtures sit one third of the way along the edges,
	// and must stay inside the triangle under any fitted geometry.
	inside := func(p vg.Point) bool {
		// Reconstruct normalized components from the draw point and
		// check they are non-negative and sum to one.
		// Inverse of x = a/2 + c, y = a*h: a = y/h, c = x - a/2.
		x := float64((p.X - g.origin.X) / g.scale)
		y := float64((p.Y - g.origin.Y) / g.scale)
		a := y / ternaryHeight
		cc := x - a/2
		b := 1 - a - cc
		for _, v := range [3]float64{a, b, cc} {
			if v < -eqTol || v > 1+eqTol {
				return false
			}
		}
		return approxEqual(a+b+cc, 1, eqTol)
	}
	for _, q := range (ABCs{
		{2.0 / 3, 1.0 / 3, 0},
		{2.0 / 3, 0, 1.0 / 3},
		{1.0 / 3, 2.0 / 3, 0},
		{0, 2.0 / 3, 1.0 / 3},
		{1.0 / 3, 0, 2.0 / 3},
		{0, 1.0 / 3, 2.0 / 3},
	}) {
		p := g.project(q.A, q.B, q.C)
		if !inside(p) {
			t.Errorf("mixture %v projected outside triangle: %v", q, p)
		}
	}
}

func TestTernaryProjectionScaleInvariance(t *testing.T) {
	// The same normalized mixture must land at the same relative
	// position under any canvas size, so resizing never moves points
	// within the triangle.
	relative := func(w, h vg.Length, q ABC) (float64, float64) {
		c := draw.NewCanvas(new(recorder.Canvas), w, h)
		g := fitTernary(c)
		p := g.project(q.A, q.B, q.C)
		return float64((p.X - g.origin.X) / g.scale),
			float64((p.Y - g.origin.Y) / g.scale)
	}
	q := ABC{0.2, 0.3, 0.5}
	rx1, ry1 := relative(100, 100, q)
	rx2, ry2 := relative(640, 480, q)
	rx3, ry3 := relative(480, 640, q)
	if !approxEqual(rx1, rx2, eqTol) || !approxEqual(rx1, rx3, eqTol) ||
		!approxEqual(ry1, ry2, eqTol) || !approxEqual(ry1, ry3, eqTol) {
		t.Errorf("relative position not scale-invariant: (%v,%v) (%v,%v) (%v,%v)",
			rx1, ry1, rx2, ry2, rx3, ry3)
	}
}

func TestNewTernaryScatterValidation(t *testing.T) {
	if _, err := NewTernaryScatter(ABCs{{0, 0, 0}}); !errors.Is(err, ErrZeroTotal) {
		t.Errorf("zero total: got %v, want %v", err, ErrZeroTotal)
	}
	if _, err := NewTernaryScatter(ABCs{{-1, 1, 1}}); !errors.Is(err, ErrNegativeComponent) {
		t.Errorf("negative component: got %v, want %v", err, ErrNegativeComponent)
	}
	s, err := NewTernaryScatter(ABCs{{1, 1, 1}})
	if err != nil {
		t.Fatal(err)
	}
	// Stored data is normalized.
	if !approxEqual(s.ABCs[0].A, 1.0/3, eqTol) {
		t.Errorf("scatter did not normalize components: %+v", s.ABCs[0])
	}
	if s.GlyphStyle.Shape == nil {
		t.Error("expected a default glyph shape")
	}
}

func TestTernaryScatterDataRange(t *testing.T) {
	s, err := NewTernaryScatter(ABCs{{1, 1, 1}})
	if err != nil {
		t.Fatal(err)
	}
	xmin, xmax, ymin, ymax := s.DataRange()
	if xmin != 0 || xmax != 1 || ymin != 0 || !approxEqual(ymax, ternaryHeight, eqTol) {
		t.Errorf("unexpected data range: (%v,%v,%v,%v)", xmin, xmax, ymin, ymax)
	}
}

func TestTernaryScatterGlyphBoxes(t *testing.T) {
	s, err := NewTernaryScatter(ABCs{
		{1, 0, 0}, // A: normalized (0, 1)
		{0, 0, 1}, // C: normalized (1, 0)
	})
	if err != nil {
		t.Fatal(err)
	}
	boxes := s.GlyphBoxes(nil)
	if len(boxes) != 2 {
		t.Fatalf("got %d boxes, want 2", len(boxes))
	}
	// A = (1,0,0) projects to normalized (a/2+c, a) = (0.5, 1).
	if !approxEqual(boxes[0].X, 0.5, eqTol) || boxes[0].Y != 1 {
		t.Errorf("A vertex box: got (%v,%v), want (0.5,1)", boxes[0].X, boxes[0].Y)
	}
	// C = (0,0,1) projects to normalized (1, 0).
	if boxes[1].X != 1 || boxes[1].Y != 0 {
		t.Errorf("C vertex box: got (%v,%v), want (1,0)", boxes[1].X, boxes[1].Y)
	}
}

func TestTernaryPlotDegenerateCanvasDoesNotPanic(t *testing.T) {
	frame := NewTernary("A", "B", "C")
	scatter, err := NewTernaryScatter(ABCs{{1, 1, 1}, {1, 0, 0}})
	if err != nil {
		t.Fatal(err)
	}
	// A canvas with no usable area must be a no-op rather than panic.
	c := draw.NewCanvas(new(recorder.Canvas), 0, 0)
	frame.Plot(c, nil)
	scatter.Plot(c, nil)

	// A very narrow but non-zero canvas must also render without
	// panic or division-by-zero in either orientation.
	for _, size := range [][2]vg.Length{{1, 100}, {100, 1}, {1, 1}} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("size %v panicked: %v", size, r)
				}
			}()
			nc := draw.NewCanvas(new(recorder.Canvas), size[0], size[1])
			frame.Plot(nc, nil)
			scatter.Plot(nc, nil)
		}()
	}

	// A zero-area geometry must not draw anything.
	empty := new(recorder.Canvas)
	frame.Plot(draw.NewCanvas(empty, 0, 100), nil)
	if len(empty.Actions) != 0 {
		t.Errorf("zero-width canvas recorded %d actions, want none", len(empty.Actions))
	}
}

func TestTernaryScatterPlotRecordsGlyphs(t *testing.T) {
	p := plot.New()
	p.HideAxes()
	frame := NewTernary("A", "B", "C")
	scatter, err := NewTernaryScatter(ABCs{
		{1, 0, 0},
		{0, 1, 0},
		{0, 0, 1},
		{1, 1, 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	scatter.GlyphStyle.Shape = draw.CircleGlyph{}
	scatter.GlyphStyle.Color = color.RGBA{R: 255, A: 255}
	p.Add(frame, scatter)

	rec := new(recorder.Canvas)
	p.Draw(draw.NewCanvas(rec, 120, 120))

	// Filled paths are emitted by CircleGlyph (one per data point) and
	// the white plot background. Count the glyph fills by color changes.
	var fills int
	for _, action := range rec.Actions {
		if fill, ok := action.(*recorder.Fill); ok {
			if len(fill.Path) > 0 {
				fills++
			}
		}
	}
	// One background fill plus four glyph fills.
	if fills < 5 {
		t.Errorf("recorded %d fill actions, want at least 5 (background + 4 glyphs)", fills)
	}
}

func TestTernaryFrameReplaysOnCanvas(t *testing.T) {
	// End-to-end replay of the recorded actions must succeed on a
	// second canvas, exercising every drawing primitive used by the
	// frame and grid.
	p := plot.New()
	p.HideAxes()
	p.Add(NewTernary("alpha", "beta", "gamma"))

	rec := new(recorder.Canvas)
	p.Draw(draw.NewCanvas(rec, 120, 120))

	target := new(recorder.Canvas)
	if err := rec.ReplayOn(target); err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if len(target.Actions) != len(rec.Actions) {
		t.Errorf("replayed %d actions, want %d", len(target.Actions), len(rec.Actions))
	}
}

// TestTernaryVerticesAndCentroidEndToEnd drives a full Plot.Draw and
// verifies from the recorded glyph fills that the three pure mixtures
// land exactly on the triangle vertices and the equal mixture lands on
// the centroid, with the three vertices forming an equilateral triangle.
func TestTernaryVerticesAndCentroidEndToEnd(t *testing.T) {
	p := plot.New()
	p.HideAxes()
	p.Add(NewTernary("A", "B", "C"))
	scatter, err := NewTernaryScatter(ABCs{
		{1, 0, 0}, // A vertex
		{0, 1, 0}, // B vertex
		{0, 0, 1}, // C vertex
		{1, 1, 1}, // centroid
	})
	if err != nil {
		t.Fatal(err)
	}
	scatter.GlyphStyle.Shape = draw.CircleGlyph{}
	scatter.GlyphStyle.Radius = vg.Points(3)
	p.Add(scatter)

	rec := new(recorder.Canvas)
	p.Draw(draw.NewCanvas(rec, 160, 160))

	// A CircleGlyph fill path is Move(center+r, center), Arc, Close.
	// The background fill has no Arc component, so it is excluded.
	var centers []vg.Point
	for _, action := range rec.Actions {
		fill, ok := action.(*recorder.Fill)
		if !ok {
			continue
		}
		hasArc := false
		for _, comp := range fill.Path {
			if comp.Type == vg.ArcComp {
				hasArc = true
			}
		}
		if !hasArc || len(fill.Path) == 0 {
			continue
		}
		move := fill.Path[0].Pos
		centers = append(centers, vg.Point{X: move.X - scatter.GlyphStyle.Radius, Y: move.Y})
	}
	if len(centers) != 4 {
		t.Fatalf("recorded %d glyph centers, want 4", len(centers))
	}

	// The triangle centroid coincides with the mean of all four
	// glyph centers: the three vertices average to the centroid, so
	// adding the centroid itself does not change the mean. Identify it
	// as the point closest to that overall mean, leaving the three
	// vertices. This avoids any winding-order heuristic.
	dist := func(a, b vg.Point) float64 {
		return math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y))
	}
	mean := vg.Point{}
	for _, q := range centers {
		mean.X += q.X
		mean.Y += q.Y
	}
	mean.X /= vg.Length(len(centers))
	mean.Y /= vg.Length(len(centers))

	centroidIdx := 0
	for i := range centers {
		if dist(centers[i], mean) < dist(centers[centroidIdx], mean) {
			centroidIdx = i
		}
	}
	var (
		vertices []vg.Point
		centroid vg.Point
	)
	for i, q := range centers {
		if i == centroidIdx {
			centroid = q
		} else {
			vertices = append(vertices, q)
		}
	}
	if len(vertices) != 3 {
		t.Fatalf("selected %d vertices, want 3", len(vertices))
	}

	// The apex is the unique top vertex; the other two share the
	// horizontal base line.
	apex := 0
	for i := 1; i < len(vertices); i++ {
		if vertices[i].Y > vertices[apex].Y {
			apex = i
		}
	}
	var base []vg.Point
	for i, q := range vertices {
		if i != apex {
			base = append(base, q)
		}
	}

	// Equilateral: every side, including both sloping sides measured
	// from the apex, must equal the base length.
	baseLen := dist(base[0], base[1])
	sideL := dist(vertices[apex], base[0])
	sideR := dist(vertices[apex], base[1])
	for name, got := range map[string]float64{
		"base":       baseLen,
		"left side":  sideL,
		"right side": sideR,
	} {
		if !approxEqual(got, baseLen, 1e-9) {
			t.Errorf("%s length %v != base %v: triangle not equilateral", name, got, baseLen)
		}
	}

	// Height-to-base ratio is sqrt(3)/2, proving the triangle is not
	// distorted by the (possibly non-square) draw area.
	height := float64(vertices[apex].Y) - float64(min(base[0].Y, base[1].Y))
	if !approxEqual(height/baseLen, ternaryHeight, 1e-9) {
		t.Errorf("height/base = %v, want sqrt(3)/2 = %v", height/baseLen, ternaryHeight)
	}

	// Explicitly confirm the centroid glyph is at the average vertex.
	wantCentroid := vg.Point{
		X: (vertices[0].X + vertices[1].X + vertices[2].X) / 3,
		Y: (vertices[0].Y + vertices[1].Y + vertices[2].Y) / 3,
	}
	if !approxEqual(dist(centroid, wantCentroid), 0, 1e-9) {
		t.Errorf("centroid glyph at %v, want %v", centroid, wantCentroid)
	}
}
