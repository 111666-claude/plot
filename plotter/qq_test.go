// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/recorder"
)

func TestQuantile(t *testing.T) {
	// Hand-computed order-statistic probabilities for n = 4:
	// p = 0, 1/3, 2/3, 1.
	s := Values{0, 1, 2, 3}
	for _, tc := range []struct {
		p    float64
		want float64
	}{
		{0, 0},
		{1.0 / 3.0, 1},
		{0.5, 1.5},
		{2.0 / 3.0, 2},
		{1, 3},
	} {
		if got := quantile(s, tc.p); got != tc.want {
			t.Errorf("quantile(%v) = %v, want %v", tc.p, got, tc.want)
		}
	}

	// A single-value sample has the same quantile at every probability.
	one := Values{7}
	for _, p := range []float64{0, 0.25, 0.5, 0.75, 1} {
		if got := quantile(one, p); got != 7 {
			t.Errorf("quantile(single, %v) = %v, want 7", p, got)
		}
	}

	// Ties: any probability inside a run of equal values returns
	// that value.
	ties := Values{1, 1, 1, 2, 2}
	for _, p := range []float64{0, 0.1, 0.25, 0.49} {
		if got := quantile(ties, p); got != 1 {
			t.Errorf("quantile(ties, %v) = %v, want 1", p, got)
		}
	}
	for _, p := range []float64{0.75, 0.9, 1} {
		if got := quantile(ties, p); got != 2 {
			t.Errorf("quantile(ties, %v) = %v, want 2", p, got)
		}
	}

	// Negative values interpolate like any other finite values.
	neg := Values{-4, -2, 0}
	if got, want := quantile(neg, 0.5), -2.0; got != want {
		t.Errorf("quantile(negative, 0.5) = %v, want %v", got, want)
	}
}

func TestNewQQ(t *testing.T) {
	// Equal-length samples land exactly on the grid order statistics.
	// [0,1,2,3] vs [10,11,12,13]: every point satisfies y = x + 10.
	q, err := NewQQ(Values{0, 1, 2, 3}, Values{10, 11, 12, 13})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := XYs{
		{X: 0, Y: 10},
		{X: 1, Y: 11},
		{X: 2, Y: 12},
		{X: 3, Y: 13},
	}
	if !reflect.DeepEqual(q.XYs, want) {
		t.Errorf("unexpected paired quantiles:\ngot: %v\nwant:%v", q.XYs, want)
	}
	for _, p := range q.XYs {
		if p.Y != p.X+10 {
			t.Errorf("point (%v, %v) is not on y = x + 10", p.X, p.Y)
		}
	}

	// Input order must not change the result: the constructor sorts.
	q2, err := NewQQ(Values{3, 1, 0, 2}, Values{13, 10, 12, 11})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(q2.XYs, want) {
		t.Errorf("unsorted input changed result:\ngot: %v\nwant:%v", q2.XYs, want)
	}
}

func TestNewQQDifferentLengths(t *testing.T) {
	// The grid has the size of the longer sample (n = 4), and the
	// shorter sample is interpolated onto it.
	q, err := NewQQ(Values{0, 2}, Values{0, 1, 2, 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := XYs{
		{X: 0, Y: 0},
		{X: 2.0 / 3.0, Y: 1},
		{X: 4.0 / 3.0, Y: 2},
		{X: 2, Y: 3},
	}
	if len(q.XYs) != len(want) {
		t.Fatalf("got %d points, want %d", len(q.XYs), len(want))
	}
	for i, p := range want {
		if q.XYs[i].X != p.X || q.XYs[i].Y != p.Y {
			t.Errorf("point %d: got (%v, %v), want (%v, %v)",
				i, q.XYs[i].X, q.XYs[i].Y, p.X, p.Y)
		}
	}

	// Identical samples, whatever their lengths, produce points on y = x.
	q, err = NewQQ(Values{1, 1, 1}, Values{-5, 0, 5, 10, 15})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, p := range q.XYs {
		if p.X != 1 {
			t.Errorf("degenerate sample quantile = %v, want 1", p.X)
		}
		if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) {
			t.Errorf("non-finite point %v", p)
		}
	}
}

func TestNewQQSingleSample(t *testing.T) {
	q, err := NewQQ(Values{42}, Values{1, 2, 3, 4, 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.XYs) != 5 {
		t.Fatalf("got %d points, want 5", len(q.XYs))
	}
	for i, p := range q.XYs {
		if p.X != 42 {
			t.Errorf("point %d: single-sample quantile = %v, want 42", i, p.X)
		}
		if p.Y != float64(i+1) {
			t.Errorf("point %d: y = %v, want %d", i, p.Y, i+1)
		}
	}

	// Both samples hold a single value: one paired point.
	q, err = NewQQ(Values{-3}, Values{9})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.XYs) != 1 || q.XYs[0] != (XY{X: -3, Y: 9}) {
		t.Errorf("unexpected points for single-value samples: %v", q.XYs)
	}
}

func TestNewQQEdgeCases(t *testing.T) {
	// Ties, extreme-but-finite tail values and negative values in one
	// construction must all produce finite, monotonically ordered
	// points.
	huge := math.MaxFloat64 / 4
	x := Values{-huge, -1, -1, -1, 0, huge}
	y := Values{huge, huge, 1, 1, 1, -huge, -huge}
	q, err := NewQQ(x, y)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.XYs) != len(y) {
		t.Fatalf("got %d points, want %d", len(q.XYs), len(y))
	}
	for i, p := range q.XYs {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) {
			t.Errorf("point %d is not finite: %v", i, p)
		}
		if i > 0 && (p.X < q.XYs[i-1].X || p.Y < q.XYs[i-1].Y) {
			t.Errorf("point %d breaks monotonic order: %v < %v", i, p, q.XYs[i-1])
		}
	}
	// The endpoints coincide with the sample extrema, so the tails
	// are represented exactly rather than extrapolated.
	if q.XYs[0].X != x[0] || q.XYs[len(q.XYs)-1].X != huge {
		t.Errorf("x endpoints = (%v, %v), want (%v, %v)",
			q.XYs[0].X, q.XYs[len(q.XYs)-1].X, x[0], huge)
	}

	// The axis range must cover the extreme data and the reference
	// line without producing NaN or Inf bounds.
	xmin, xmax, ymin, ymax := q.DataRange()
	for _, v := range []float64{xmin, xmax, ymin, ymax} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("non-finite DataRange bound: %v", v)
		}
	}
	if xmin != -huge || xmax != huge || ymin != -huge || ymax != huge {
		t.Errorf("DataRange = (%v, %v, %v, %v), want (±%v) bounds",
			xmin, xmax, ymin, ymax, huge)
	}
}

func TestNewQQErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		x, y Valuer
		want error
	}{
		{"empty x", Values{}, Values{1}, ErrNoData},
		{"empty y", Values{1}, Values{}, ErrNoData},
		{"NaN x", Values{math.NaN()}, Values{1}, ErrNaN},
		{"NaN y", Values{1}, Values{1, math.NaN()}, ErrNaN},
		{"+Inf x", Values{math.Inf(1)}, Values{1}, ErrInfinity},
		{"-Inf y", Values{1}, Values{math.Inf(-1)}, ErrInfinity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewQQ(tc.x, tc.y)
			if !errors.Is(err, tc.want) {
				t.Errorf("got error %v, want %v", err, tc.want)
			}
		})
	}
}

func TestQQDataRange(t *testing.T) {
	// Data: x in [0, 3], y in [10, 13]. The reference line spans the
	// union, so it must extend the x range up to 13 and the y range
	// down to 0.
	q, err := NewQQ(Values{0, 1, 2, 3}, Values{10, 11, 12, 13})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	xmin, xmax, ymin, ymax := q.DataRange()
	if xmin != 0 || xmax != 13 || ymin != 0 || ymax != 13 {
		t.Errorf("DataRange = (%v, %v, %v, %v), want (0, 13, 0, 13)",
			xmin, xmax, ymin, ymax)
	}
	if q.refMin != 0 || q.refMax != 13 {
		t.Errorf("reference endpoints = (%v, %v), want (0, 13)", q.refMin, q.refMax)
	}
}

func TestQQPlotCoordinates(t *testing.T) {
	q, err := NewQQ(Values{0, 1, 2, 3}, Values{10, 11, 12, 13})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p := plot.New()
	p.X.Min, p.X.Max = 0, 13
	p.Y.Min, p.Y.Max = 0, 13
	p.X.Padding, p.Y.Padding = 0, 0
	p.Add(q)

	c := new(recorder.Canvas)
	dc := draw.NewCanvas(c, 100, 100)
	p.Draw(dc)

	// The plotter draws into the cropped data canvas, so expected
	// coordinates must be transformed through it as well.
	dataDC := p.DataCanvas(dc)
	trX, trY := p.Transforms(&dataDC)

	// The reference line runs from (0, 0) to (13, 13) in data space.
	wantLine := vg.Path{
		{Type: vg.MoveComp, Pos: vg.Point{X: trX(0), Y: trY(0)}},
		{Type: vg.LineComp, Pos: vg.Point{X: trX(13), Y: trY(13)}},
	}

	// The default glyph is a ring: each glyph center is recorded as
	// the center of an ArcComp.
	var glyphCenters []vg.Point
	var linePaths []vg.Path
	for _, a := range c.Actions {
		s, ok := a.(*recorder.Stroke)
		if !ok {
			continue
		}
		if reflect.DeepEqual(s.Path, wantLine) {
			linePaths = append(linePaths, s.Path)
			continue
		}
		for _, comp := range s.Path {
			if comp.Type == vg.ArcComp {
				glyphCenters = append(glyphCenters, comp.Pos)
			}
		}
	}

	if len(glyphCenters) != len(q.XYs) {
		t.Fatalf("got %d glyphs, want %d", len(glyphCenters), len(q.XYs))
	}
	// Glyphs are drawn in probability order, which is also the order
	// of the stored points.
	for i, pt := range q.XYs {
		want := vg.Point{X: trX(pt.X), Y: trY(pt.Y)}
		if glyphCenters[i] != want {
			t.Errorf("glyph %d: got %v, want %v", i, glyphCenters[i], want)
		}
	}

	if len(linePaths) != 1 {
		t.Errorf("got %d reference lines, want 1", len(linePaths))
	}
}

func TestQQReferenceLineDisabled(t *testing.T) {
	q, err := NewQQ(Values{0, 1}, Values{0, 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	q.LineStyle.Width = 0

	p := plot.New()
	p.Add(q)
	c := new(recorder.Canvas)
	p.Draw(draw.NewCanvas(c, vg.Centimeter, vg.Centimeter))

	// With the reference line disabled, every remaining stroke must be
	// a glyph ring (a path containing an arc), never a two-point line.
	for _, a := range c.Actions {
		s, ok := a.(*recorder.Stroke)
		if !ok {
			continue
		}
		if reflect.DeepEqual(s.Path, vg.Path{
			{Type: vg.MoveComp},
			{Type: vg.LineComp},
		}) {
			t.Errorf("unexpected straight stroke with reference line disabled: %v", s.Path)
		}
	}
}
