// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter_test

import (
	"image/color"
	"log"
	"math/rand/v2"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

// ExampleQQ draws a two-sample quantile-quantile plot comparing the
// frame latency distributions of two renderers. Points near the dashed
// y = x line indicate similar distributions; deviations in the tails
// show where the renderers differ.
func ExampleQQ() {
	rnd := rand.New(rand.NewPCG(1, 2))

	// Two samples of different lengths, drawn from distributions whose
	// means differ by a constant shift.
	const nx, ny = 100, 150
	x := make(plotter.Values, nx)
	for i := range x {
		x[i] = rnd.NormFloat64()*2 + 10
	}
	y := make(plotter.Values, ny)
	for i := range y {
		y[i] = rnd.NormFloat64()*2 + 12
	}

	q, err := plotter.NewQQ(x, y)
	if err != nil {
		log.Panic(err)
	}
	q.GlyphStyle.Color = color.RGBA{R: 255, A: 255}
	q.GlyphStyle.Radius = vg.Points(2)

	// A standalone line provides the y = x legend thumbnail; the QQ
	// plotter already draws the reference line on the data area.
	refLine, err := plotter.NewLine(plotter.XYs{{X: 0, Y: 0}, {X: 1, Y: 1}})
	if err != nil {
		log.Panic(err)
	}
	refLine.LineStyle = plotter.DefaultQQLineStyle

	p := plot.New()
	p.Title.Text = "Frame latency Q-Q plot"
	p.X.Label.Text = "renderer A (ms)"
	p.Y.Label.Text = "renderer B (ms)"
	p.Add(q)
	p.Legend.Add("paired quantiles", q)
	p.Legend.Add("y = x", refLine)

	// Save the plot in both vector and raster form so the output can
	// be checked at both the SVG and PNG coordinate levels.
	err = p.Save(4*vg.Inch, 4*vg.Inch, "testdata/qq.png")
	if err != nil {
		log.Panic(err)
	}
	err = p.Save(4*vg.Inch, 4*vg.Inch, "testdata/qq.svg")
	if err != nil {
		log.Panic(err)
	}
}
