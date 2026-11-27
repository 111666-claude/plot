// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter_test

import (
	"errors"
	"image/color"
	"math"
	"testing"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/cmpimg"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

// survivalData implements plotter.SurvivalObservationer for testing.
type survivalData struct {
	time  []float64
	event []bool
}

func (s survivalData) Len() int { return len(s.time) }

func (s survivalData) Time(i int) float64 { return s.time[i] }

func (s survivalData) Event(i int) bool { return s.event[i] }

func TestKaplanMeierEstimate(t *testing.T) {
	// Four samples: event at 1, censored at 2, event at 3, censored at 4.
	data := survivalData{
		time:  []float64{1, 2, 3, 4},
		event: []bool{true, false, true, false},
	}
	km, err := plotter.NewKaplanMeier(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantXYs := plotter.XYs{
		{X: 0, Y: 1},
		{X: 1, Y: 0.75},  // after the event at 1: 3/4
		{X: 2, Y: 0.75},  // censoring at 2 leaves the estimate unchanged
		{X: 3, Y: 0.375}, // after the event at 3: 3/4 * 1/2 = 3/8
		{X: 4, Y: 0.375}, // censoring at 4 leaves the estimate unchanged
	}
	if !xyEqual(km.XYs, wantXYs) {
		t.Errorf("unexpected step curve:\ngot: %v\nwant: %v", km.XYs, wantXYs)
	}

	wantCensors := plotter.XYs{
		{X: 2, Y: 0.75},
		{X: 4, Y: 0.375},
	}
	if !xyEqual(km.CensorXYs, wantCensors) {
		t.Errorf("unexpected censor points:\ngot: %v\nwant: %v", km.CensorXYs, wantCensors)
	}

	xmin, xmax, ymin, ymax := km.DataRange()
	for _, tc := range []struct {
		name string
		got  float64
		want float64
	}{
		{"xmin", xmin, 0},
		{"xmax", xmax, 4},
		{"ymin", ymin, 0.375},
		{"ymax", ymax, 1},
	} {
		if tc.got != tc.want {
			t.Errorf("unexpected %s: got %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

func TestKaplanMeierTies(t *testing.T) {
	// Two events and one censoring share time t=2. The censored
	// observation remains in the risk set when the events are processed.
	// After the event at t=1 three players remain at risk, two of whom
	// leave at t=2, so the estimate drops by (3-2)/3 = 1/3 from 3/4 to
	// 1/4 and the censor mark sits on the new level.
	data := survivalData{
		time:  []float64{1, 2, 2, 2},
		event: []bool{true, true, true, false},
	}
	km, err := plotter.NewKaplanMeier(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantXYs := plotter.XYs{
		{X: 0, Y: 1},
		{X: 1, Y: 0.75},
		{X: 2, Y: 0.25},
	}
	if !xyEqual(km.XYs, wantXYs) {
		t.Errorf("unexpected step curve:\ngot: %v\nwant: %v", km.XYs, wantXYs)
	}

	wantCensors := plotter.XYs{{X: 2, Y: 0.25}}
	if !xyEqual(km.CensorXYs, wantCensors) {
		t.Errorf("unexpected censor points:\ngot: %v\nwant: %v", km.CensorXYs, wantCensors)
	}
}

func TestKaplanMeierAllCensored(t *testing.T) {
	// With no events the estimate stays at 1 for the whole observed
	// range and every distinct censor time gets a mark.
	data := survivalData{
		time:  []float64{2, 2, 5},
		event: []bool{false, false, false},
	}
	km, err := plotter.NewKaplanMeier(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantXYs := plotter.XYs{
		{X: 0, Y: 1},
		{X: 2, Y: 1},
		{X: 5, Y: 1},
	}
	if !xyEqual(km.XYs, wantXYs) {
		t.Errorf("unexpected step curve:\ngot: %v\nwant: %v", km.XYs, wantXYs)
	}

	wantCensors := plotter.XYs{
		{X: 2, Y: 1},
		{X: 5, Y: 1},
	}
	if !xyEqual(km.CensorXYs, wantCensors) {
		t.Errorf("unexpected censor points:\ngot: %v\nwant: %v", km.CensorXYs, wantCensors)
	}
}

func TestKaplanMeierZeroTime(t *testing.T) {
	// Time zero is a valid non-negative observed time.
	data := survivalData{
		time:  []float64{0, 1},
		event: []bool{true, true},
	}
	km, err := plotter.NewKaplanMeier(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantXYs := plotter.XYs{
		{X: 0, Y: 1},
		{X: 0, Y: 0.5},
		{X: 1, Y: 0},
	}
	if !xyEqual(km.XYs, wantXYs) {
		t.Errorf("unexpected step curve:\ngot: %v\nwant: %v", km.XYs, wantXYs)
	}
}

func TestKaplanMeierErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		data survivalData
		want error
	}{
		{
			name: "empty",
			data: survivalData{},
			want: plotter.ErrNoSurvivalData,
		},
		{
			name: "negative time",
			data: survivalData{
				time:  []float64{1, -0.5},
				event: []bool{true, false},
			},
			want: plotter.ErrNegativeTime,
		},
		{
			name: "NaN time",
			data: survivalData{
				time:  []float64{1, math.NaN()},
				event: []bool{true, false},
			},
			want: plotter.ErrNaN,
		},
		{
			name: "infinite time",
			data: survivalData{
				time:  []float64{1, math.Inf(1)},
				event: []bool{true, false},
			},
			want: plotter.ErrInfinity,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := plotter.NewKaplanMeier(tc.data)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got error %v, want %v", err, tc.want)
			}
		})
	}
}

func TestKaplanMeierInterfaces(t *testing.T) {
	km, err := plotter.NewKaplanMeier(survivalData{
		time:  []float64{1, 2},
		event: []bool{true, false},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var _ plot.Plotter = km
	var _ plot.DataRanger = km
	var _ plot.GlyphBoxer = km
	var _ plot.Thumbnailer = km
}

func TestKaplanMeierPlot(t *testing.T) {
	cmpimg.CheckPlot(ExampleKaplanMeier, t, "kaplanMeier.png")
}

func xyEqual(a, b plotter.XYs) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].X != b[i].X || a[i].Y != b[i].Y {
			return false
		}
	}
	return true
}

// ExampleKaplanMeier draws the Kaplan–Meier survival curves of two player
// groups after a game-server disconnect. Players that are still online at
// the end of the observation window are right-censored.
func ExampleKaplanMeier() {
	groupA := survivalData{
		time:  []float64{1, 2, 3, 4, 5, 6},
		event: []bool{true, false, true, true, false, true},
	}
	groupB := survivalData{
		time:  []float64{2, 3, 4, 5, 6, 7},
		event: []bool{false, true, false, true, true, false},
	}

	p := plot.New()
	p.Title.Text = "Player retention after disconnect"
	p.X.Label.Text = "time online"
	p.Y.Label.Text = "survival fraction"
	p.Add(plotter.NewGrid())
	// Place the legend in the bottom-right corner, clear of the curves.
	p.Legend.Top = false
	p.Legend.Left = false

	kmA, err := plotter.NewKaplanMeier(groupA)
	if err != nil {
		panic(err)
	}
	kmA.LineStyle.Color = color.RGBA{R: 255, A: 255}
	kmA.CensorGlyphStyle.Color = color.RGBA{R: 255, A: 255}

	kmB, err := plotter.NewKaplanMeier(groupB)
	if err != nil {
		panic(err)
	}
	kmB.LineStyle.Color = color.RGBA{B: 255, A: 255}
	kmB.CensorGlyphStyle.Color = color.RGBA{B: 255, A: 255}

	p.Add(kmA, kmB)
	p.Legend.Add("group A", kmA)
	p.Legend.Add("group B", kmB)
	p.Y.Min = 0
	p.Y.Max = 1

	err = p.Save(4*vg.Inch, 4*vg.Inch, "testdata/kaplanMeier.png")
	if err != nil {
		panic(err)
	}
}
