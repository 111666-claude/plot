// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter_test

import (
	"math"
	"testing"

	"gonum.org/v1/plot/cmpimg"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

func TestViolinPlot(t *testing.T) {
	cmpimg.CheckPlot(ExampleViolinPlot, t, "violinPlot.png",
		"horizontalViolinPlot.png")
}

// TestViolinProfile checks the kernel density profile of a
// violin made from a single sample 0 with bandwidth 1: the
// density at the center must be 1/√(2π) and the density one
// unit away must be the center value times exp(-1/2).
func TestViolinProfile(t *testing.T) {
	v, err := plotter.NewViolinPlot(vg.Points(20), 0, plotter.Values{0}, 1)
	if err != nil {
		t.Fatalf("could not create violin plot: %v", err)
	}

	center := 1 / math.Sqrt(2*math.Pi)

	// Check the exact density evaluator.
	if got := v.DensityAt(0); got != center {
		t.Errorf("unexpected center density: got %v want %v", got, center)
	}
	if got, want := v.DensityAt(1), center*math.Exp(-0.5); got != want {
		t.Errorf("unexpected density at 1: got %v want %v", got, want)
	}

	// Check the sampled profile coordinates: the grid must
	// contain the center and the points one bandwidth away.
	prof := v.Profile()
	at := func(x float64) (float64, bool) {
		for _, p := range prof {
			if p.X == x {
				return p.Y, true
			}
		}
		return 0, false
	}
	for _, tc := range []struct {
		x    float64
		want float64
	}{
		{x: 0, want: center},
		{x: 1, want: center * math.Exp(-0.5)},
		{x: -1, want: center * math.Exp(-0.5)},
	} {
		got, ok := at(tc.x)
		if !ok {
			t.Errorf("profile has no point at x=%v", tc.x)
			continue
		}
		if got != tc.want {
			t.Errorf("unexpected profile density at x=%v: got %v want %v",
				tc.x, got, tc.want)
		}
	}
}

// TestViolinStats checks the median and quartiles of a violin.
func TestViolinStats(t *testing.T) {
	v, err := plotter.NewViolinPlot(vg.Points(20), 1,
		plotter.Values{5, 1, 3, 2, 4}, 1)
	if err != nil {
		t.Fatalf("could not create violin plot: %v", err)
	}
	if v.Location != 1 {
		t.Errorf("unexpected location: got %v want 1", v.Location)
	}
	if v.Median != 3 {
		t.Errorf("unexpected median: got %v want 3", v.Median)
	}
	if v.Quartile1 != 1.5 {
		t.Errorf("unexpected first quartile: got %v want 1.5", v.Quartile1)
	}
	if v.Quartile3 != 4 {
		t.Errorf("unexpected third quartile: got %v want 4", v.Quartile3)
	}
	if v.Min != 1 || v.Max != 5 {
		t.Errorf("unexpected extremes: got [%v, %v] want [1, 5]", v.Min, v.Max)
	}
}

// TestViolinStable checks that degenerate inputs — repeated
// values, a single sample, negative values, empty groups and
// very narrow bandwidths — give stable, finite results.
func TestViolinStable(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values plotter.Values
		bw     float64
	}{
		{name: "repeated", values: plotter.Values{2, 2, 2, 2, 2}, bw: 1},
		{name: "repeated auto bandwidth", values: plotter.Values{2, 2, 2, 2, 2}, bw: 0},
		{name: "single", values: plotter.Values{3}, bw: 1},
		{name: "single auto bandwidth", values: plotter.Values{3}, bw: 0},
		{name: "negative", values: plotter.Values{-5, -3, -4}, bw: 1},
		{name: "narrow bandwidth", values: plotter.Values{0, 1e6}, bw: 1e-9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := plotter.NewViolinPlot(vg.Points(20), 0, tc.values, tc.bw)
			if err != nil {
				t.Fatalf("could not create violin plot: %v", err)
			}
			if v.Bandwidth <= 0 || math.IsNaN(v.Bandwidth) || math.IsInf(v.Bandwidth, 0) {
				t.Fatalf("invalid bandwidth: %v", v.Bandwidth)
			}
			prof := v.Profile()
			if len(prof) == 0 {
				t.Fatal("empty profile")
			}
			for _, p := range prof {
				if math.IsNaN(p.X) || math.IsNaN(p.Y) ||
					math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) {
					t.Fatalf("non-finite profile point: %v", p)
				}
				if p.Y < 0 {
					t.Fatalf("negative density: %v", p)
				}
			}
		})
	}
}

// TestViolinEmpty checks the behavior with empty groups.
func TestViolinEmpty(t *testing.T) {
	if _, err := plotter.NewViolinPlot(vg.Points(20), 0, plotter.Values{}, 1); err == nil {
		t.Error("expected an error for a violin with no values")
	}

	// Empty groups are skipped when making several violins.
	vs, err := plotter.NewViolinPlots(vg.Points(20), 1,
		plotter.Values{1, 2, 3}, plotter.Values{}, plotter.Values{4, 5, 6})
	if err != nil {
		t.Fatalf("could not create violin plots: %v", err)
	}
	if len(vs) != 2 {
		t.Fatalf("unexpected number of violins: got %d want 2", len(vs))
	}
	if vs[0].Location != 0 || vs[1].Location != 2 {
		t.Errorf("unexpected locations: got %v and %v want 0 and 2",
			vs[0].Location, vs[1].Location)
	}
}

// TestViolinErrors checks that invalid widths and bandwidths
// are rejected.
func TestViolinErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		w    vg.Length
		bw   float64
	}{
		{name: "negative width", w: -1, bw: 1},
		{name: "negative bandwidth", w: 1, bw: -1},
		{name: "NaN bandwidth", w: 1, bw: math.NaN()},
		{name: "infinite bandwidth", w: 1, bw: math.Inf(1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := plotter.NewViolinPlot(tc.w, 0, plotter.Values{1, 2}, tc.bw)
			if err == nil {
				t.Error("expected an error")
			}
		})
	}
}
