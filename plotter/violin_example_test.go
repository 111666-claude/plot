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

func ExampleViolinPlot() {
	rnd := rand.New(rand.NewPCG(1, 1))

	// Create the sample data.  The first group is bimodal:
	// a box plot of it would hide the two clusters.
	const n = 100
	bimodal := make(plotter.Values, n)
	normal := make(plotter.Values, n)
	uniform := make(plotter.Values, n)
	for i := range n {
		c := 2.0
		if i%2 == 0 {
			c = -2
		}
		bimodal[i] = c + rnd.NormFloat64()
		normal[i] = rnd.NormFloat64()
		uniform[i] = rnd.Float64()
	}

	// Make violins for the three groups with a common
	// width scale, and add them to the plot.
	vs, err := plotter.NewViolinPlots(vg.Points(40), 0, bimodal, normal, uniform)
	if err != nil {
		log.Panic(err)
	}
	vs[0].FillColor = color.RGBA{127, 188, 165, 255}
	vs[1].FillColor = color.RGBA{255, 214, 96, 255}
	vs[2].FillColor = color.RGBA{140, 160, 220, 255}

	p := plot.New()
	p.Title.Text = "Violin Plot"
	p.Y.Label.Text = "plotter.Values"
	for i, v := range vs {
		p.Add(v)
		p.Legend.Add([]string{"Bimodal", "Normal", "Uniform"}[i], v)
	}
	p.NominalX("Bimodal\nDistribution", "Normal\nDistribution",
		"Uniform\nDistribution")

	err = p.Save(200, 200, "testdata/violinPlot.png")
	if err != nil {
		log.Panic(err)
	}

	// Now, make the same plot but horizontal.
	for _, v := range vs {
		v.Horizontal = true
	}

	p2 := plot.New()
	p2.Title.Text = "Horizontal Violin Plot"
	p2.X.Label.Text = "plotter.Values"
	for _, v := range vs {
		p2.Add(v)
	}
	p2.NominalY("Bimodal\nDistribution", "Normal\nDistribution",
		"Uniform\nDistribution")

	err = p2.Save(200, 200, "testdata/horizontalViolinPlot.png")
	if err != nil {
		log.Panic(err)
	}
}
