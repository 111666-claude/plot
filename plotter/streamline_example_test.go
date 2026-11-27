// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter_test

import (
	"image/color"
	"log"
	"os"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"
)

// ExampleStreamline draws streamlines of a rotational vector field.
func ExampleStreamline() {
	f, err := plotter.NewStreamline(field{
		r: 17, c: 19,
		fn: func(x, y float64) plotter.XY {
			return plotter.XY{
				X: y,
				Y: -x,
			}
		},
	}, plotter.XYs{
		{X: 1, Y: 0},
		{X: 2, Y: 0},
		{X: 3, Y: 0},
		{X: 4, Y: 0},
	})
	if err != nil {
		log.Panic(err)
	}
	f.StepSize = 0.1
	f.LineStyle.Color = color.RGBA{R: 0x08, G: 0x48, B: 0x8e, A: 0xff}
	f.LineStyle.Width = vg.Points(0.5)

	p := plot.New()
	p.Title.Text = "Streamlines"
	p.X.Tick.Marker = integerTicks{}
	p.Y.Tick.Marker = integerTicks{}

	p.Add(f)
	p.Legend.Add("flow", f)
	p.Legend.Top = true

	img := vgimg.New(250, 175)
	dc := draw.New(img)

	p.Draw(dc)
	w, err := os.Create("testdata/streamline.png")
	if err != nil {
		log.Panic(err)
	}
	defer w.Close()
	png := vgimg.PngCanvas{Canvas: img}
	if _, err = png.WriteTo(w); err != nil {
		log.Panic(err)
	}
}
