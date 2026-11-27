// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"image/color"
	"math"
	"testing"

	"gonum.org/v1/plot/palette"
)

func TestNewHorizonErrors(t *testing.T) {
	for _, test := range []struct {
		name      string
		data      XYs
		baseline  float64
		bandwidth float64
		err       error
	}{
		{
			name:      "empty",
			data:      nil,
			baseline:  0,
			bandwidth: 1,
			err:       ErrHorizonEmpty,
		},
		{
			name:      "zero bandwidth",
			data:      XYs{{X: 0, Y: 1}},
			baseline:  0,
			bandwidth: 0,
			err:       ErrHorizonBandwidth,
		},
		{
			name:      "negative bandwidth",
			data:      XYs{{X: 0, Y: 1}},
			baseline:  0,
			bandwidth: -2,
			err:       ErrHorizonBandwidth,
		},
		{
			name:      "NaN bandwidth",
			data:      XYs{{X: 0, Y: 1}},
			baseline:  0,
			bandwidth: math.NaN(),
			err:       ErrHorizonBandwidth,
		},
		{
			name:      "infinite bandwidth",
			data:      XYs{{X: 0, Y: 1}},
			baseline:  0,
			bandwidth: math.Inf(1),
			err:       ErrHorizonBandwidth,
		},
		{
			name:      "infinite baseline",
			data:      XYs{{X: 0, Y: 1}},
			baseline:  math.Inf(-1),
			bandwidth: 1,
			err:       ErrHorizonInf,
		},
		{
			name:      "infinite value",
			data:      XYs{{X: 0, Y: math.Inf(1)}},
			baseline:  0,
			bandwidth: 1,
			err:       ErrHorizonInf,
		},
		{
			name:      "infinite x",
			data:      XYs{{X: math.Inf(1), Y: 1}},
			baseline:  0,
			bandwidth: 1,
			err:       ErrHorizonInf,
		},
		{
			name:      "unordered",
			data:      XYs{{X: 0, Y: 1}, {X: 0, Y: 2}},
			baseline:  0,
			bandwidth: 1,
			err:       ErrHorizonUnordered,
		},
		{
			name:      "decreasing",
			data:      XYs{{X: 1, Y: 1}, {X: 0, Y: 2}},
			baseline:  0,
			bandwidth: 1,
			err:       ErrHorizonUnordered,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewHorizon(test.data, test.baseline, test.bandwidth)
			if err != test.err {
				t.Fatalf("unexpected error: got %v, want %v", err, test.err)
			}
		})
	}
}

func TestNewHorizonNaNBreaksSeries(t *testing.T) {
	// NaN values are accepted and mark missing measurements rather than
	// being rejected.
	h, err := NewHorizon(XYs{
		{X: 0, Y: 1},
		{X: 1, Y: math.NaN()},
		{X: 2, Y: -1},
		{X: math.NaN(), Y: 1},
		{X: 3, Y: 2},
	}, 0, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	segs := h.segments()
	if len(segs) != 3 {
		t.Fatalf("unexpected number of segments: got %d, want 3", len(segs))
	}
	wantLens := []int{1, 1, 1}
	for i, seg := range segs {
		if len(seg.runs) != wantLens[i] {
			t.Errorf("segment %d: got %d runs, want %d", i, len(seg.runs), wantLens[i])
		}
	}
}

func TestHorizonLayers(t *testing.T) {
	// Baseline 0, bandwidth 2: ±3 each occupy the full first layer and
	// half of the second layer, with different sign colors.
	h, err := NewHorizon(XYs{{X: 0, Y: -3}, {X: 1, Y: 3}}, 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pos, neg := h.Layers()
	if pos != 2 || neg != 2 {
		t.Fatalf("got layers pos=%d neg=%d, want 2 and 2", pos, neg)
	}

	// A deviation equal to an exact multiple of the bandwidth fills only
	// that many full bands.
	h, err = NewHorizon(XYs{{X: 0, Y: 4}}, 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pos, _ = h.Layers()
	if pos != 2 {
		t.Fatalf("got %d positive layers, want 2", pos)
	}

	// A series that stays on the baseline still has one layer of each
	// sign so the legend and default palettes remain well defined.
	h, err = NewHorizon(XYs{{X: 0, Y: 0}, {X: 1, Y: 0}}, 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pos, neg = h.Layers(); pos != 1 || neg != 1 {
		t.Fatalf("got layers pos=%d neg=%d, want 1 and 1", pos, neg)
	}
}

func TestFillFraction(t *testing.T) {
	const bandwidth = 2
	for _, test := range []struct {
		dev   float64
		layer int
		want  float64
	}{
		{dev: 3, layer: 0, want: 1},   // first band full
		{dev: 3, layer: 1, want: 0.5}, // second band half
		{dev: 3, layer: 2, want: 0},   // beyond the deviation
		{dev: -3, layer: 0, want: 1},
		{dev: -3, layer: 1, want: 0.5},
		{dev: 1, layer: 0, want: 0.5},
		{dev: 4, layer: 1, want: 1},
		{dev: 4, layer: 2, want: 0},
	} {
		if got := fillFraction(test.dev, bandwidth, test.layer); got != test.want {
			t.Errorf("fillFraction(%v, %v, %d) = %v, want %v",
				test.dev, bandwidth, test.layer, got, test.want)
		}
	}
}

func TestThresholdCuts(t *testing.T) {
	const bandwidth = 2
	for _, test := range []struct {
		name string
		a, b horizonPoint
		want []horizonPoint
	}{
		{
			name: "single band no cut",
			a:    horizonPoint{x: 0, dev: 0.5},
			b:    horizonPoint{x: 1, dev: 1.5},
			want: nil,
		},
		{
			name: "cross upper threshold",
			a:    horizonPoint{x: 0, dev: 1},
			b:    horizonPoint{x: 1, dev: 3},
			want: []horizonPoint{{x: 0.5, dev: 2}},
		},
		{
			name: "cross baseline",
			a:    horizonPoint{x: 0, dev: -1},
			b:    horizonPoint{x: 1, dev: 1},
			want: []horizonPoint{{x: 0.5, dev: 0}},
		},
		{
			name: "cross several thresholds",
			a:    horizonPoint{x: 0, dev: -3},
			b:    horizonPoint{x: 1, dev: 3},
			want: []horizonPoint{
				{x: 1.0 / 6, dev: -2},
				{x: 0.5, dev: 0},
				{x: 5.0 / 6, dev: 2},
			},
		},
		{
			name: "exact threshold not duplicated",
			a:    horizonPoint{x: 0, dev: 2},
			b:    horizonPoint{x: 1, dev: 3},
			want: nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := thresholdCuts(test.a, test.b, bandwidth)
			if len(got) != len(test.want) {
				t.Fatalf("got %d cuts %+v, want %d %+v", len(got), got, len(test.want), test.want)
			}
			for i := range got {
				if got[i] != test.want[i] {
					t.Errorf("cut %d: got %+v, want %+v", i, got[i], test.want[i])
				}
			}
		})
	}
}

func TestHorizonSegmentRuns(t *testing.T) {
	h, err := NewHorizon(XYs{
		{X: 0, Y: -3},
		{X: 1, Y: 3},
	}, 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	segs := h.segments()
	if len(segs) != 1 {
		t.Fatalf("got %d segments, want 1", len(segs))
	}
	runs := segs[0].runs
	// From -3 to +3 the edge crosses -2, 0 and +2, producing four
	// sign-homogeneous runs connected at the exact cut positions.
	if len(runs) != 4 {
		t.Fatalf("got %d runs, want 4", len(runs))
	}
	wantSigns := []bool{false, false, true, true}
	for i, r := range runs {
		if r.positive != wantSigns[i] {
			t.Errorf("run %d sign: got %v, want %v", i, r.positive, wantSigns[i])
		}
	}
	// Every cut is placed by linear interpolation.
	wantCuts := map[float64]bool{
		0: true, 1: true,
		1.0 / 6: true, 0.5: true, 5.0 / 6: true,
	}
	for i, r := range runs {
		for _, p := range r.points {
			if !wantCuts[p.x] {
				t.Errorf("run %d: unexpected cut position %v", i, p.x)
			}
		}
	}
}

func TestHorizonAllMissing(t *testing.T) {
	// A non-empty series where every measurement is missing must still
	// construct without error and must render without producing segments
	// or panicking. The x range stays valid because the timestamps remain
	// finite; only the values are NaN.
	h, err := NewHorizon(XYs{
		{X: 0, Y: math.NaN()},
		{X: 1, Y: math.NaN()},
	}, 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if segs := h.segments(); len(segs) != 0 {
		t.Fatalf("got %d segments, want 0", len(segs))
	}
	xmin, xmax, ymin, ymax := h.DataRange()
	if xmin != 0 || xmax != 1 {
		t.Fatalf("got x range [%v, %v], want [0, 1]", xmin, xmax)
	}
	// The folded band still spans one bandwidth around the baseline.
	if ymin != -2 || ymax != 2 {
		t.Fatalf("got y range [%v, %v], want [-2, 2]", ymin, ymax)
	}
}

func TestHorizonAllMissingCoordinates(t *testing.T) {
	// When the timestamps themselves are missing, the x range is empty
	// and represented by infinities; plot.Axis.sanitizeRange handles that
	// when drawing.
	h, err := NewHorizon(XYs{
		{X: math.NaN(), Y: 1},
		{X: math.NaN(), Y: 2},
	}, 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if segs := h.segments(); len(segs) != 0 {
		t.Fatalf("got %d segments, want 0", len(segs))
	}
	xmin, xmax, _, _ := h.DataRange()
	if !math.IsInf(xmin, 1) || !math.IsInf(xmax, -1) {
		t.Fatalf("got x range [%v, %v], want empty (inf, -inf)", xmin, xmax)
	}
}

func TestHorizonSinglePoint(t *testing.T) {
	h, err := NewHorizon(XYs{{X: 5, Y: 7}}, 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pos, neg := h.Layers()
	if pos != 4 || neg != 1 {
		t.Fatalf("got layers pos=%d neg=%d, want 4 and 1", pos, neg)
	}
	segs := h.segments()
	if len(segs) != 1 || len(segs[0].runs) != 1 {
		t.Fatalf("got %+v, want one segment with one run", segs)
	}
}

func TestHorizonCanonicalFolding(t *testing.T) {
	// Baseline 0, bandwidth 2: both -3 and +3 occupy the full height of
	// the first layer and half of the second layer, but with different
	// sign colors.
	h, err := NewHorizon(XYs{{X: 0, Y: -3}, {X: 1, Y: 3}}, 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, test := range []struct {
		name string
		dev  float64
	}{
		{name: "positive", dev: 3},
		{name: "negative", dev: -3},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := fillFraction(test.dev, 2, 0); got != 1 {
				t.Errorf("first layer fraction = %v, want 1 (full height)", got)
			}
			if got := fillFraction(test.dev, 2, 1); got != 0.5 {
				t.Errorf("second layer fraction = %v, want 0.5 (half height)", got)
			}
			if got := fillFraction(test.dev, 2, 2); got != 0 {
				t.Errorf("third layer fraction = %v, want 0", got)
			}
		})
	}

	posColors := h.Positive.Colors()
	negColors := h.Negative.Colors()
	if sameColor(posColors[0], negColors[0]) {
		t.Error("first-layer positive and negative colors must differ")
	}
	if sameColor(posColors[1], negColors[1]) {
		t.Error("second-layer positive and negative colors must differ")
	}
}

func TestHorizonDataRange(t *testing.T) {
	h, err := NewHorizon(XYs{
		{X: 2, Y: -7},
		{X: 5, Y: 9},
	}, 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	xmin, xmax, ymin, ymax := h.DataRange()
	if xmin != 2 || xmax != 5 {
		t.Errorf("x range got [%v, %v], want [2, 5]", xmin, xmax)
	}
	// The vertical range always spans exactly one band centered on the
	// baseline, independent of how many layers the deviations need.
	if ymin != -1 || ymax != 3 {
		t.Errorf("y range got [%v, %v], want [-1, 3]", ymin, ymax)
	}
}

func TestHorizonLegend(t *testing.T) {
	h, err := NewHorizon(XYs{{X: 0, Y: -3}, {X: 1, Y: 3}}, 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries := h.Legend()
	if len(entries) != 4 {
		t.Fatalf("got %d legend entries, want 4", len(entries))
	}
	wantLabels := []string{"[-2, 0)", "[-4, -2)", "[0, 2)", "[2, 4)"}
	for i, e := range entries {
		if e.Label != wantLabels[i] {
			t.Errorf("entry %d: got label %q, want %q", i, e.Label, wantLabels[i])
		}
		if e.Thumbnail == nil {
			t.Errorf("entry %d has nil thumbnail", i)
		}
	}

	// Negative and positive first-layer colors must differ.
	if sameColor(entries[0].Thumbnail.(HorizonThumbnail).Color,
		entries[2].Thumbnail.(HorizonThumbnail).Color) {
		t.Error("positive and negative layer colors must differ")
	}
}

func TestHorizonNonZeroBaselineLegend(t *testing.T) {
	h, err := NewHorizon(XYs{{X: 0, Y: 11}, {X: 1, Y: 9}}, 10, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries := h.Legend()
	wantLabels := []string{"[8, 10)", "[10, 12)"}
	if len(entries) != len(wantLabels) {
		t.Fatalf("got %d entries, want %d", len(entries), len(wantLabels))
	}
	for i, e := range entries {
		if e.Label != wantLabels[i] {
			t.Errorf("entry %d: got %q, want %q", i, e.Label, wantLabels[i])
		}
	}
}

func TestHorizonShortPalettePanics(t *testing.T) {
	h, err := NewHorizon(XYs{{X: 0, Y: 5}}, 0, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h.Positive = singleColorPalette{color.NRGBA{R: 1, A: 255}}
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for palette shorter than the layer count")
		}
	}()
	_, _ = h.paletteColors()
}

func TestHorizonEmptyPalettePanics(t *testing.T) {
	h, err := NewHorizon(XYs{{X: 0, Y: 1}}, 0, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h.Positive = singleColorPalette{}
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for an empty palette")
		}
	}()
	_, _ = h.paletteColors()
}

type singleColorPalette struct {
	c color.Color
}

func (p singleColorPalette) Colors() []color.Color {
	if p.c == nil {
		return nil
	}
	return []color.Color{p.c}
}

var _ palette.Palette = singleColorPalette{}

func sameColor(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
