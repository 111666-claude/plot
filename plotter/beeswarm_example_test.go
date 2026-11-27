// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter_test

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand/v2"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// ExampleBeeswarm draws a beeswarm plot of frame times per level.
// Frame times are quantized so that many samples share the same
// value; the beeswarm layout keeps every sample visible instead of
// overplotting them.
func ExampleBeeswarm() {
	rnd := rand.New(rand.NewPCG(1, 1))

	// frameTimes returns n quantized frame times in milliseconds
	// around the given mean. Quantizing to 0.5 ms creates many
	// duplicate values, like a frame timer with a fixed resolution.
	frameTimes := func(n int, mean float64) []float64 {
		ts := make([]float64, n)
		for i := range ts {
			t := mean + 2*rnd.NormFloat64()
			ts[i] = math.Round(t*2) / 2
		}
		return ts
	}

	const n = 60
	levels := []string{"Level 1", "Level 2", "Level 3"}
	means := []float64{16.5, 18, 21}

	pts := make(plotter.XYs, 0, n*len(levels))
	for lvl := range levels {
		for _, t := range frameTimes(n, means[lvl]) {
			pts = append(pts, plotter.XY{X: float64(lvl), Y: t})
		}
	}

	p := plot.New()
	p.Title.Text = "Frame time per level"
	p.Y.Label.Text = "Frame time (ms)"
	p.NominalX(levels...)
	p.Add(plotter.NewGrid())

	s, err := plotter.NewBeeswarm(pts)
	if err != nil {
		log.Panic(err)
	}
	s.GlyphStyle.Color = color.RGBA{R: 255, B: 128, A: 255}
	s.GlyphStyle.Radius = vg.Points(2)

	p.Add(s)
	p.Legend.Add("frame time", s)

	// Leave room for the horizontal spread of the swarms.
	p.X.Min = -0.5
	p.X.Max = float64(len(levels)) - 0.5

	err = p.Save(300, 240, "testdata/beeswarm.png")
	if err != nil {
		log.Panic(err)
	}

	err = p.Save(300, 240, "testdata/beeswarm.svg")
	if err != nil {
		log.Panic(err)
	}
}

// ExampleBeeswarm_colored draws a beeswarm plot with per-point
// glyph styles, reusing the existing GlyphStyleFunc mechanism.
func ExampleBeeswarm_colored() {
	rnd := rand.New(rand.NewPCG(2, 2))

	pts := make(plotter.XYs, 0, 90)
	for lvl := 0; lvl < 3; lvl++ {
		for i := 0; i < 30; i++ {
			t := 16 + 2*float64(lvl) + 3*rnd.Float64()
			pts = append(pts, plotter.XY{X: float64(lvl), Y: math.Round(t*2) / 2})
		}
	}

	p := plot.New()
	p.Title.Text = "Frame time per level"
	p.Y.Label.Text = "Frame time (ms)"
	p.NominalX("Level 1", "Level 2", "Level 3")

	s, err := plotter.NewBeeswarm(pts)
	if err != nil {
		log.Panic(err)
	}
	s.GlyphStyle.Radius = vg.Points(2)
	// Color points by whether the frame time exceeds a 20 ms budget.
	s.GlyphStyleFunc = func(i int) draw.GlyphStyle {
		sty := s.GlyphStyle
		if pts[i].Y > 20 {
			sty.Color = color.RGBA{R: 255, A: 255}
		} else {
			sty.Color = color.RGBA{G: 160, A: 255}
		}
		return sty
	}

	p.Add(s)
	p.X.Min = -0.5
	p.X.Max = 2.5

	err = p.Save(300, 240, "testdata/beeswarmColor.png")
	if err != nil {
		log.Panic(err)
	}

	fmt.Println("done")

	// Output: done
}
