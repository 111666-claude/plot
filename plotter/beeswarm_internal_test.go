// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"math"
	"testing"

	"gonum.org/v1/plot/vg"
)

func TestSwarmLayoutThreeEqualPoints(t *testing.T) {
	r := vg.Points(2)
	ys := []vg.Length{10, 10, 10}
	radii := []vg.Length{r, r, r}

	offs := swarmLayout(ys, radii)

	// All three points must remain countable: no two glyphs may
	// share the same center, and centers must be at least one
	// diameter apart.
	seen := make(map[vg.Length]int)
	for i, off := range offs {
		if _, ok := seen[off]; ok {
			t.Errorf("point %d overlaps another point at offset %v", i, off)
		}
		seen[off] = i
	}
	for i := range offs {
		for j := i + 1; j < len(offs); j++ {
			d := offs[i] - offs[j]
			if d < 0 {
				d = -d
			}
			if d < 2*r {
				t.Errorf("points %d and %d are %v apart, want at least %v", i, j, d, 2*r)
			}
		}
	}

	// The densest points should be closest to the center: with
	// three equal points the offsets are 0, -d, +d.
	want := map[vg.Length]bool{0: true, -2 * r: true, 2 * r: true}
	for _, off := range offs {
		if !want[off] {
			t.Errorf("unexpected offset %v, want one of 0, %v, %v", off, -2*r, 2*r)
		}
		delete(want, off)
	}
}

func TestSwarmLayoutNoOverlap(t *testing.T) {
	r := vg.Points(1.5)
	// A column of points spaced by less than a diameter forces a
	// multi-row layout.
	ys := []vg.Length{0, 1, 2, 3, 4, 5, 0.5, 2.5, 4.5, 1.5}
	radii := make([]vg.Length, len(ys))
	for i := range radii {
		radii[i] = r
	}

	offs := swarmLayout(ys, radii)

	for i := range ys {
		for j := i + 1; j < len(ys); j++ {
			dx := offs[i] - offs[j]
			dy := ys[i] - ys[j]
			if math.Hypot(float64(dx), float64(dy)) < float64(2*r) {
				t.Errorf("points %d and %d overlap: dx=%v dy=%v, diameter=%v", i, j, dx, dy, 2*r)
			}
		}
	}
}

func TestSwarmLayoutDeterministic(t *testing.T) {
	r := vg.Points(2)
	ys := []vg.Length{3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5}
	radii := make([]vg.Length, len(ys))
	for i := range radii {
		radii[i] = r
	}

	first := swarmLayout(ys, radii)
	for range 10 {
		got := swarmLayout(ys, radii)
		for i := range got {
			if got[i] != first[i] {
				t.Fatalf("non-deterministic layout at %d: got %v, want %v", i, got, first)
			}
		}
	}
}

func TestSwarmLayoutEmpty(t *testing.T) {
	if offs := swarmLayout(nil, nil); len(offs) != 0 {
		t.Errorf("expected no offsets for empty input, got %v", offs)
	}
}

func TestSwarmLayoutZeroRadius(t *testing.T) {
	ys := []vg.Length{1, 1, 1}
	radii := []vg.Length{0, -1, 0}

	offs := swarmLayout(ys, radii)
	for i, off := range offs {
		if off != 0 {
			t.Errorf("offset %d: got %v, want 0 for non-positive radius", i, off)
		}
	}
}

func TestSwarmLayoutMixedRadii(t *testing.T) {
	ys := []vg.Length{0, 0, 0}
	radii := []vg.Length{vg.Points(1), vg.Points(3), vg.Points(2)}

	offs := swarmLayout(ys, radii)

	for i := range ys {
		for j := i + 1; j < len(ys); j++ {
			dx := offs[i] - offs[j]
			if dx < 0 {
				dx = -dx
			}
			min := radii[i] + radii[j]
			if dx < min {
				t.Errorf("points %d and %d are %v apart, want at least %v", i, j, dx, min)
			}
		}
	}
}
