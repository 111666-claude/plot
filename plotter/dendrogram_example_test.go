// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter_test

import (
	"log"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
)

func ExampleDendrogram() {
	// Leaves of the clustering, and the binary merge tree over them.
	// Leaf i is referred to as node i, and the i-th merge creates
	// node len(labels)+i.
	labels := []string{"A", "B", "C", "D", "E", "F"}
	merges := []plotter.Merge{
		{Left: 0, Right: 1, Height: 1},   // A and B merge at height 1.
		{Left: 3, Right: 4, Height: 1.5}, // D and E merge at height 1.5.
		{Left: 2, Right: 6, Height: 2.5}, // C joins {A, B} at height 2.5.
		{Left: 7, Right: 5, Height: 3},   // F joins {D, E} at height 3.
		{Left: 8, Right: 9, Height: 5},   // The two groups merge at height 5.
	}

	// Vertical dendrogram: leaves along the X axis, heights along Y.
	d, err := plotter.NewDendrogram(labels, merges)
	if err != nil {
		log.Panic(err)
	}
	p := plot.New()
	p.Title.Text = "Dendrogram"
	p.Y.Label.Text = "Height"
	p.Add(d)
	p.X.Tick.Marker = plot.ConstantTicks(d.Ticks())
	p.Add(plotter.NewGlyphBoxes())
	if err := p.Save(200, 200, "testdata/dendrogram.png"); err != nil {
		log.Panic(err)
	}

	// Horizontal dendrogram with a chosen leaf order and wider
	// spacing between the leaves.
	h, err := plotter.NewDendrogram(labels, merges)
	if err != nil {
		log.Panic(err)
	}
	h.Horizontal = true
	h.Spacing = 2
	if err := h.SetLeafOrder([]int{0, 1, 2, 3, 4, 5}); err != nil {
		log.Panic(err)
	}
	p2 := plot.New()
	p2.Title.Text = "Dendrogram"
	p2.X.Label.Text = "Height"
	p2.Add(h)
	p2.Y.Tick.Marker = plot.ConstantTicks(h.Ticks())
	p2.Add(plotter.NewGlyphBoxes())
	if err := p2.Save(200, 200, "testdata/dendrogram_horizontal.png"); err != nil {
		log.Panic(err)
	}
}
