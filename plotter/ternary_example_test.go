// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter_test

import (
	"image/color"
	"log"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// ExampleTernaryScatter draws two groups of ternary mixtures inside a
// labeled ternary frame. Components are normalized by their per-point
// total, so the two groups use raw (non-normalized) component values.
func ExampleTernaryScatter() {
	groupA := plotter.ABCs{
		{10, 2, 2}, // A-rich, near the A vertex
		{6, 3, 3},  // two-thirds A
		{1, 1, 1},  // equal mixture, at the centroid
		{2, 7, 1},  // B-rich
		{1, 3, 6},  // C-rich
		{4, 4, 2},  // mid mixture
	}
	groupB := plotter.ABCs{
		{2, 10, 2}, // strongly B-rich
		{3, 6, 3},  // two-thirds B
		{1, 1, 8},  // strongly C-rich
		{3, 3, 4},  // slightly C-rich
		{5, 1, 4},  // A- and C-leaning
	}

	p := plot.New()
	p.Title.Text = "Ternary Scatter"
	// The ternary frame supplies its own axes; hide the Cartesian ones.
	p.HideAxes()

	frame := plotter.NewTernary("A", "B", "C")
	p.Add(frame)

	s1, err := plotter.NewTernaryScatter(groupA)
	if err != nil {
		log.Panic(err)
	}
	s1.GlyphStyle.Shape = draw.CircleGlyph{}
	s1.GlyphStyle.Color = color.RGBA{R: 196, G: 40, B: 40, A: 255}
	s1.GlyphStyle.Radius = vg.Points(4)

	s2, err := plotter.NewTernaryScatter(groupB)
	if err != nil {
		log.Panic(err)
	}
	s2.GlyphStyle.Shape = draw.SquareGlyph{}
	s2.GlyphStyle.Color = color.RGBA{B: 196, A: 255}
	s2.GlyphStyle.Radius = vg.Points(4)

	p.Add(s1, s2)
	p.Legend.Top = true
	p.Legend.Add("group 1", s1)
	p.Legend.Add("group 2", s2)

	if err := p.Save(12*vg.Centimeter, 12*vg.Centimeter, "testdata/ternaryScatter.png"); err != nil {
		log.Panic(err)
	}
	if err := p.Save(12*vg.Centimeter, 12*vg.Centimeter, "testdata/ternaryScatter.svg"); err != nil {
		log.Panic(err)
	}
}
