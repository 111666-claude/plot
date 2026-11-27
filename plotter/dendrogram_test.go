// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter_test

import (
	"math"
	"testing"

	"gonum.org/v1/plot/cmpimg"
	"gonum.org/v1/plot/plotter"
)

func TestDendrogram(t *testing.T) {
	cmpimg.CheckPlot(ExampleDendrogram, t, "dendrogram.png", "dendrogram_horizontal.png")
}

// TestDendrogramMergePoints checks the fixed tree coordinates of a
// dendrogram of three leaves where A and B merge at height 2 and the
// {A, B} group merges with C at height 5.
func TestDendrogramMergePoints(t *testing.T) {
	d, err := plotter.NewDendrogram(
		[]string{"A", "B", "C"},
		[]plotter.Merge{
			{Left: 0, Right: 1, Height: 2},
			{Left: 3, Right: 2, Height: 5},
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The A/B branches must join at height 2, halfway between the
	// leaves A and B, and the root branches must join at height 5,
	// halfway between the {A, B} group and C.
	for i, want := range [2]struct{ x, y float64 }{
		{0.5, 2},
		{1.25, 5},
	} {
		x, y := d.MergePoint(i)
		if x != want.x || y != want.y {
			t.Errorf("merge %d: got join point (%v, %v), want (%v, %v)", i, x, y, want.x, want.y)
		}
	}

	// The leaf ticks must sit at the leaf coordinates.
	ticks := d.Ticks()
	for i, want := range []struct {
		value float64
		label string
	}{
		{0, "A"},
		{1, "B"},
		{2, "C"},
	} {
		if ticks[i].Value != want.value || ticks[i].Label != want.label {
			t.Errorf("tick %d: got (%v, %q), want (%v, %q)",
				i, ticks[i].Value, ticks[i].Label, want.value, want.label)
		}
	}

	xmin, xmax, ymin, ymax := d.DataRange()
	if xmin != 0 || xmax != 2 || ymin != 0 || ymax != 5 {
		t.Errorf("got data range (%v, %v, %v, %v), want (0, 2, 0, 5)", xmin, xmax, ymin, ymax)
	}

	// Reordering the leaves must move the leaf coordinates without
	// changing the merge heights.
	if err := d.SetLeafOrder([]int{2, 0, 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, y := d.MergePoint(0); y != 2 {
		t.Errorf("after reordering: got A/B join height %v, want 2", y)
	}
	if _, y := d.MergePoint(1); y != 5 {
		t.Errorf("after reordering: got root join height %v, want 5", y)
	}
	ticks = d.Ticks()
	for i, want := range []float64{1, 2, 0} {
		if ticks[i].Value != want {
			t.Errorf("after reordering: leaf %d at %v, want %v", i, ticks[i].Value, want)
		}
	}
}

func TestDendrogramSingleLeaf(t *testing.T) {
	d, err := plotter.NewDendrogram([]string{"A"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ticks := d.Ticks()
	if len(ticks) != 1 || ticks[0].Label != "A" || ticks[0].Value != 0 {
		t.Errorf("unexpected ticks: %+v", ticks)
	}
	xmin, xmax, ymin, ymax := d.DataRange()
	if xmin != 0 || xmax != 0 || ymin != 0 || ymax != 0 {
		t.Errorf("got data range (%v, %v, %v, %v), want (0, 0, 0, 0)", xmin, xmax, ymin, ymax)
	}
}

func TestDendrogramErrors(t *testing.T) {
	for _, test := range []struct {
		name   string
		labels []string
		merges []plotter.Merge
	}{
		{
			name:   "no leaves",
			labels: nil,
			merges: nil,
		},
		{
			name:   "missing merges",
			labels: []string{"A", "B", "C"},
			merges: []plotter.Merge{{Left: 0, Right: 1, Height: 1}},
		},
		{
			name:   "extra merges",
			labels: []string{"A", "B"},
			merges: []plotter.Merge{{Left: 0, Right: 1, Height: 1}, {Left: 2, Right: 0, Height: 2}},
		},
		{
			name:   "negative height",
			labels: []string{"A", "B"},
			merges: []plotter.Merge{{Left: 0, Right: 1, Height: -1}},
		},
		{
			name:   "NaN height",
			labels: []string{"A", "B"},
			merges: []plotter.Merge{{Left: 0, Right: 1, Height: math.NaN()}},
		},
		{
			name:   "infinite height",
			labels: []string{"A", "B"},
			merges: []plotter.Merge{{Left: 0, Right: 1, Height: math.Inf(1)}},
		},
		{
			name:   "parent below child",
			labels: []string{"A", "B", "C"},
			merges: []plotter.Merge{
				{Left: 0, Right: 1, Height: 5},
				{Left: 3, Right: 2, Height: 2},
			},
		},
		{
			name:   "duplicate leaf",
			labels: []string{"A", "B", "C"},
			merges: []plotter.Merge{
				{Left: 0, Right: 1, Height: 1},
				{Left: 0, Right: 2, Height: 2},
			},
		},
		{
			name:   "self merge",
			labels: []string{"A", "B"},
			merges: []plotter.Merge{{Left: 0, Right: 0, Height: 1}},
		},
		{
			name:   "invalid child",
			labels: []string{"A", "B"},
			merges: []plotter.Merge{{Left: 0, Right: 2, Height: 1}},
		},
		{
			name:   "forward reference",
			labels: []string{"A", "B", "C"},
			merges: []plotter.Merge{
				{Left: 4, Right: 2, Height: 2},
				{Left: 0, Right: 1, Height: 1},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := plotter.NewDendrogram(test.labels, test.merges); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

func TestDendrogramSetLeafOrderErrors(t *testing.T) {
	d, err := plotter.NewDendrogram(
		[]string{"A", "B", "C"},
		[]plotter.Merge{
			{Left: 0, Right: 1, Height: 2},
			{Left: 3, Right: 2, Height: 5},
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, order := range [][]int{
		{0, 1},       // wrong length
		{0, 1, 3},    // invalid leaf
		{0, 1, 1},    // duplicate leaf
		{-1, 0, 1},   // negative leaf
		{0, 1, 2, 3}, // too long
	} {
		if err := d.SetLeafOrder(order); err == nil {
			t.Errorf("expected an error for order %v, got nil", order)
		}
	}
}
