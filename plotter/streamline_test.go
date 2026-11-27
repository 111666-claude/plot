// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter_test

import (
	"math"
	"testing"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/cmpimg"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"
)

func TestStreamline(t *testing.T) {
	cmpimg.CheckPlot(ExampleStreamline, t, "streamline.png")
}

// rightField returns a constant rightward vector field on the
// fixed 5×5 integer grid used by the Field plotter tests, with
// coordinates spanning [-2, 2] on both axes.
func rightField() field {
	return field{
		r: 5, c: 5,
		fn: func(x, y float64) plotter.XY {
			return plotter.XY{X: 1, Y: 0}
		},
	}
}

// TestStreamlineConstantField checks that a streamline traced two
// units through a constant rightward field from (0, 0) passes
// through (1, 0) and (2, 0) and stops at the grid boundary.
func TestStreamlineConstantField(t *testing.T) {
	s, err := plotter.NewStreamline(rightField(), plotter.XYs{{X: 0, Y: 0}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.StepSize = 1
	s.MaxLength = 10 // Deliberately long: tracing must stop at the boundary.
	s.Bidirectional = false

	lines := s.Streamlines()
	if len(lines) != 1 {
		t.Fatalf("unexpected number of streamlines: got %d want 1", len(lines))
	}
	want := plotter.XYs{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 2, Y: 0}}
	if len(lines[0]) != len(want) {
		t.Fatalf("unexpected number of points: got %d want %d: %v", len(lines[0]), len(want), lines[0])
	}
	for i, p := range lines[0] {
		if p != want[i] {
			t.Errorf("unexpected point %d: got %v want %v", i, p, want[i])
		}
	}
}

// TestStreamlineBidirectional checks that a bidirectionally traced
// streamline spans the grid on both sides of the seed, ordered
// along the flow.
func TestStreamlineBidirectional(t *testing.T) {
	s, err := plotter.NewStreamline(rightField(), plotter.XYs{{X: 0, Y: 0}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.StepSize = 1

	lines := s.Streamlines()
	if len(lines) != 1 {
		t.Fatalf("unexpected number of streamlines: got %d want 1", len(lines))
	}
	want := plotter.XYs{
		{X: -2, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 0}, {X: 1, Y: 0}, {X: 2, Y: 0},
	}
	if len(lines[0]) != len(want) {
		t.Fatalf("unexpected number of points: got %d want %d: %v", len(lines[0]), len(want), lines[0])
	}
	for i, p := range lines[0] {
		if p != want[i] {
			t.Errorf("unexpected point %d: got %v want %v", i, p, want[i])
		}
	}
}

// TestStreamlineMaxLength checks that tracing stops once the
// configured maximum arc length is reached.
func TestStreamlineMaxLength(t *testing.T) {
	s, err := plotter.NewStreamline(rightField(), plotter.XYs{{X: -2, Y: 0}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.StepSize = 0.5
	s.MaxLength = 2
	s.Bidirectional = false

	lines := s.Streamlines()
	if len(lines) != 1 {
		t.Fatalf("unexpected number of streamlines: got %d want 1", len(lines))
	}
	last := lines[0][len(lines[0])-1]
	if last.X != 0 || last.Y != 0 {
		t.Errorf("unexpected last point: got %v want {0 0}", last)
	}
}

// TestStreamlineZeroField checks that a seed in a stagnant field
// produces a degenerate streamline containing only the seed.
func TestStreamlineZeroField(t *testing.T) {
	f := field{
		r: 5, c: 5,
		fn: func(x, y float64) plotter.XY {
			return plotter.XY{}
		},
	}
	s, err := plotter.NewStreamline(f, plotter.XYs{{X: 0, Y: 0}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := s.Streamlines()
	if len(lines) != 1 {
		t.Fatalf("unexpected number of streamlines: got %d want 1", len(lines))
	}
	if len(lines[0]) != 1 || lines[0][0] != (plotter.XY{X: 0, Y: 0}) {
		t.Errorf("unexpected streamline: got %v want [{0 0}]", lines[0])
	}
}

// TestStreamlineNearZero checks that tracing stops where the
// interpolated field speed vanishes and does not cross into the
// stagnant region.
func TestStreamlineNearZero(t *testing.T) {
	f := field{
		r: 5, c: 5,
		fn: func(x, y float64) plotter.XY {
			// The speed decreases linearly to zero at x=1
			// and vanishes beyond it.
			return plotter.XY{X: math.Max(0, 1-x), Y: 0}
		},
	}
	s, err := plotter.NewStreamline(f, plotter.XYs{{X: 0, Y: 0}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.StepSize = 0.1
	s.Bidirectional = false

	lines := s.Streamlines()
	if len(lines) != 1 {
		t.Fatalf("unexpected number of streamlines: got %d want 1", len(lines))
	}
	last := lines[0][len(lines[0])-1]
	if last.X < 0.85 || last.X > 1+1e-6 {
		t.Errorf("streamline did not stop at the stagnant point: got x=%g want in [0.85, 1]", last.X)
	}
}

// TestStreamlineClosed checks that a closed streamline terminates
// instead of looping forever, and that it follows the circular
// flow of a rotational field.
func TestStreamlineClosed(t *testing.T) {
	f := field{
		r: 9, c: 9,
		fn: func(x, y float64) plotter.XY {
			return plotter.XY{X: -y, Y: x}
		},
	}
	s, err := plotter.NewStreamline(f, plotter.XYs{{X: 2, Y: 0}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.StepSize = 0.05
	s.MaxLength = 100 // Deliberately long: tracing must stop by closure.
	s.Bidirectional = false

	lines := s.Streamlines()
	if len(lines) != 1 {
		t.Fatalf("unexpected number of streamlines: got %d want 1", len(lines))
	}
	line := lines[0]
	// The circumference of the radius-2 circle is about 12.6, far
	// below MaxLength, so the trace must have stopped by closure.
	if got := float64(len(line)) * s.StepSize; got > 20 {
		t.Errorf("streamline did not stop on closure: traced length %g", got)
	}
	for _, p := range line {
		if r := math.Hypot(p.X, p.Y); math.Abs(r-2) > 1e-4 {
			t.Fatalf("point %v off the radius-2 circle: r=%g", p, r)
		}
	}
	// The final point must be near an earlier point of the line.
	last := line[len(line)-1]
	closed := false
	for _, p := range line[:len(line)-3] {
		if math.Hypot(last.X-p.X, last.Y-p.Y) < s.StepSize/2 {
			closed = true
			break
		}
	}
	if !closed {
		t.Errorf("streamline did not close on itself: last point %v", last)
	}
}

// TestStreamlineDuplicateSeeds checks that duplicate seeds are
// traced only once.
func TestStreamlineDuplicateSeeds(t *testing.T) {
	s, err := plotter.NewStreamline(rightField(), plotter.XYs{
		{X: 0, Y: 0},
		{X: 0, Y: 0},
		{X: 0, Y: 1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.StepSize = 1
	if got := len(s.Streamlines()); got != 2 {
		t.Errorf("unexpected number of streamlines: got %d want 2", got)
	}
}

// TestStreamlineCanvasIndependent checks that the computed
// streamlines do not depend on the canvas the plot is drawn on.
func TestStreamlineCanvasIndependent(t *testing.T) {
	f := field{
		r: 9, c: 9,
		fn: func(x, y float64) plotter.XY {
			return plotter.XY{X: -y, Y: x}
		},
	}
	seeds := plotter.XYs{{X: 0.3, Y: 0.2}, {X: -1, Y: 1}, {X: 2.5, Y: -1.5}}

	compute := func(w, h float64) []plotter.XYs {
		s, err := plotter.NewStreamline(f, seeds)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		s.StepSize = 0.05
		p := plot.New()
		p.Add(s)
		img := vgimg.New(vg.Length(w), vg.Length(h))
		p.Draw(draw.New(img))
		return s.Streamlines()
	}

	a := compute(250, 175)
	b := compute(1000, 40)
	if len(a) != len(b) {
		t.Fatalf("streamline count depends on canvas size: got %d and %d", len(a), len(b))
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			t.Fatalf("streamline %d length depends on canvas size: got %d and %d", i, len(a[i]), len(b[i]))
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				t.Fatalf("streamline %d point %d depends on canvas size: got %v and %v", i, j, a[i][j], b[i][j])
			}
		}
	}
}

// TestStreamlineDims checks that degenerate grids do not panic,
// mirroring TestFieldDims for the Field plotter.
func TestStreamlineDims(t *testing.T) {
	for _, test := range []struct {
		rows int
		cols int
	}{
		{rows: 1, cols: 2},
		{rows: 2, cols: 1},
		{rows: 2, cols: 2},
	} {
		func() {
			defer func() {
				r := recover()
				if r != nil {
					t.Errorf("unexpected panic for rows=%d cols=%d: %v", test.rows, test.cols, r)
				}
			}()

			f, err := plotter.NewStreamline(field{
				r: test.rows, c: test.cols,
				fn: func(x, y float64) plotter.XY {
					return plotter.XY{
						X: y,
						Y: -x,
					}
				},
			}, plotter.XYs{{X: 0, Y: 0}})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			p := plot.New()
			p.Add(f)

			img := vgimg.New(250, 175)
			dc := draw.New(img)

			p.Draw(dc)
		}()
	}
}
