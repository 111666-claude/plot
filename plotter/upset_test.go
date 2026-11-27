// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter_test

import (
	"strings"
	"testing"

	"gonum.org/v1/plot/cmpimg"
	"gonum.org/v1/plot/plotter"
)

func TestUpSet(t *testing.T) {
	cmpimg.CheckPlot(ExampleUpSet, t, "upset.png", "upset.svg")
}

// TestUpSet_counts checks the exact combination counts, set
// totals and layout coordinates of the canonical two-set
// example A={1,2}, B={2,3}.
func TestUpSet_counts(t *testing.T) {
	up, err := plotter.NewUpSet([]plotter.Set{
		{Name: "A", Elements: []string{"1", "2"}},
		{Name: "B", Elements: []string{"2", "3"}},
	})
	if err != nil {
		t.Fatalf("could not create UpSet plotter: %+v", err)
	}

	// The exact counts of the only-A, A∩B and only-B
	// combinations must all be 1.
	want := []plotter.Combination{
		{Mask: 0b01, Count: 1}, // only A
		{Mask: 0b11, Count: 1}, // A∩B
		{Mask: 0b10, Count: 1}, // only B
	}
	got := up.Shown()
	if len(got) != len(want) {
		t.Fatalf("unexpected number of combinations: got %d, want %d", len(got), len(want))
	}
	for i, c := range want {
		if got[i] != c {
			t.Errorf("combination %d: got %+v, want %+v", i, got[i], c)
		}
	}
	if h := up.Hidden(); h != 0 {
		t.Errorf("unexpected number of hidden combinations: got %d, want 0", h)
	}

	// The total size of each set must be 2.
	for j, want := range []int{2, 2} {
		if got := up.Totals[j]; got != want {
			t.Errorf("total of set %d: got %d, want %d", j, got, want)
		}
	}

	// Fixed layout coordinates.
	for i, want := range []float64{0.5, 1.5, 2.5} {
		if got := up.ColumnX(i); got != want {
			t.Errorf("column %d x: got %v, want %v", i, got, want)
		}
	}
	for j, want := range []float64{1.5, 0.5} {
		if got := up.RowY(j); got != want {
			t.Errorf("row %d y: got %v, want %v", j, got, want)
		}
	}
	if got, want := up.BarBase(), 2.5; got != want {
		t.Errorf("bar base: got %v, want %v", got, want)
	}

	xmin, xmax, ymin, ymax := up.DataRange()
	if xmin != -2.5 || xmax != 3 || ymin != 0 || ymax != 4 {
		t.Errorf("unexpected data range: got (%v, %v, %v, %v), want (-2.5, 3, 0, 4)",
			xmin, xmax, ymin, ymax)
	}

	ticks := up.Ticks()
	if len(ticks) != 2 || ticks[0].Label != "A" || ticks[0].Value != 1.5 ||
		ticks[1].Label != "B" || ticks[1].Value != 0.5 {
		t.Errorf("unexpected ticks: got %+v", ticks)
	}
}

// TestUpSet_duplicates checks that duplicate elements within
// a set are counted once.
func TestUpSet_duplicates(t *testing.T) {
	up, err := plotter.NewUpSet([]plotter.Set{
		{Name: "A", Elements: []string{"1", "1", "2", "2", "2"}},
		{Name: "B", Elements: []string{"2", "2", "3"}},
	})
	if err != nil {
		t.Fatalf("could not create UpSet plotter: %+v", err)
	}

	for j, want := range []int{2, 2} {
		if got := up.Totals[j]; got != want {
			t.Errorf("total of set %d: got %d, want %d", j, got, want)
		}
	}
	want := []plotter.Combination{
		{Mask: 0b01, Count: 1}, // only A: "1"
		{Mask: 0b11, Count: 1}, // A∩B: "2"
		{Mask: 0b10, Count: 1}, // only B: "3"
	}
	got := up.Shown()
	if len(got) != len(want) {
		t.Fatalf("unexpected number of combinations: got %d, want %d", len(got), len(want))
	}
	for i, c := range want {
		if got[i] != c {
			t.Errorf("combination %d: got %+v, want %+v", i, got[i], c)
		}
	}
}

// TestUpSet_emptySets checks that empty sets are valid,
// contribute a zero total and take part in no combination.
func TestUpSet_emptySets(t *testing.T) {
	up, err := plotter.NewUpSet([]plotter.Set{
		{Name: "A", Elements: []string{"1"}},
		{Name: "B"},
	})
	if err != nil {
		t.Fatalf("could not create UpSet plotter: %+v", err)
	}
	if got := up.Totals[1]; got != 0 {
		t.Errorf("total of empty set: got %d, want 0", got)
	}
	if got := up.Shown(); len(got) != 1 || got[0] != (plotter.Combination{Mask: 0b01, Count: 1}) {
		t.Errorf("unexpected combinations: got %+v", got)
	}

	// All-empty input yields no combinations but a valid
	// data range.
	up, err = plotter.NewUpSet([]plotter.Set{{Name: "A"}, {Name: "B"}})
	if err != nil {
		t.Fatalf("could not create UpSet plotter: %+v", err)
	}
	if got := up.Shown(); len(got) != 0 {
		t.Errorf("unexpected combinations: got %+v", got)
	}
	xmin, xmax, ymin, ymax := up.DataRange()
	if !(xmin < xmax && ymin < ymax) {
		t.Errorf("degenerate data range: got (%v, %v, %v, %v)", xmin, xmax, ymin, ymax)
	}
}

// TestUpSet_duplicateNames checks that two sets sharing a
// name are rejected.
func TestUpSet_duplicateNames(t *testing.T) {
	_, err := plotter.NewUpSet([]plotter.Set{
		{Name: "A", Elements: []string{"1"}},
		{Name: "A", Elements: []string{"2"}},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate set name") {
		t.Errorf("expected duplicate-name error, got %v", err)
	}
}

// TestUpSet_noSets checks that an empty set list is rejected.
func TestUpSet_noSets(t *testing.T) {
	_, err := plotter.NewUpSet(nil)
	if err == nil {
		t.Error("expected an error for no sets, got nil")
	}
}

// TestUpSet_tooManySets checks the resource limit on the
// number of sets.
func TestUpSet_tooManySets(t *testing.T) {
	sets := make([]plotter.Set, plotter.MaxUpSetSets+1)
	for i := range sets {
		sets[i] = plotter.Set{
			Name:     strings.Repeat("s", i+1),
			Elements: []string{"e"},
		}
	}
	_, err := plotter.NewUpSet(sets)
	if err == nil || !strings.Contains(err.Error(), "too many sets") {
		t.Errorf("expected too-many-sets error, got %v", err)
	}

	// Exactly MaxUpSetSets sets must be accepted.
	up, err := plotter.NewUpSet(sets[:plotter.MaxUpSetSets])
	if err != nil {
		t.Fatalf("could not create UpSet plotter with %d sets: %+v", plotter.MaxUpSetSets, err)
	}
	if got := up.Shown(); len(got) != 1 || got[0].Mask != ^uint64(0) || got[0].Count != 1 {
		t.Errorf("unexpected combinations: got %+v", got)
	}
}

// TestUpSet_maxCombos checks the limit on the number of
// displayed combinations and the stable sort by size.
func TestUpSet_maxCombos(t *testing.T) {
	up, err := plotter.NewUpSet([]plotter.Set{
		{Name: "A", Elements: []string{"a1", "a2", "ab", "abc"}},
		{Name: "B", Elements: []string{"b1", "ab", "abc"}},
		{Name: "C", Elements: []string{"abc"}},
	})
	if err != nil {
		t.Fatalf("could not create UpSet plotter: %+v", err)
	}

	// Combinations by first appearance: A-only (2), A∩B (1),
	// A∩B∩C (1), B-only (1). The sort by decreasing count is
	// stable, so the three size-1 combinations keep their
	// order of first appearance.
	want := []plotter.Combination{
		{Mask: 0b001, Count: 2},
		{Mask: 0b011, Count: 1},
		{Mask: 0b111, Count: 1},
		{Mask: 0b010, Count: 1},
	}
	if len(up.Combinations) != len(want) {
		t.Fatalf("unexpected number of combinations: got %d, want %d",
			len(up.Combinations), len(want))
	}
	for i, c := range want {
		if up.Combinations[i] != c {
			t.Errorf("combination %d: got %+v, want %+v", i, up.Combinations[i], c)
		}
	}

	up.MaxCombos = 2
	if got := up.Shown(); len(got) != 2 {
		t.Errorf("unexpected number of shown combinations: got %d, want 2", len(got))
	}
	if got := up.Hidden(); got != 2 {
		t.Errorf("unexpected number of hidden combinations: got %d, want 2", got)
	}

	// A non-positive MaxCombos falls back to the default.
	up.MaxCombos = 0
	if got := up.Shown(); len(got) != len(want) {
		t.Errorf("unexpected number of shown combinations: got %d, want %d",
			len(got), len(want))
	}
}
