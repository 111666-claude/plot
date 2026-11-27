// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter_test

import (
	"log"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

// ExampleUpSet creates an UpSet plot of test-tag memberships
// across three test suites.
// The output can be found at https://github.com/gonum/plot/blob/master/plotter/testdata/upset_golden.png.
func ExampleUpSet() {
	sets := []plotter.Set{
		{Name: "Combat", Elements: []string{"t01", "t02", "t03", "t04", "t05", "t06"}},
		{Name: "Multiplayer", Elements: []string{"t02", "t04", "t07", "t08"}},
		{Name: "Mobile", Elements: []string{"t04", "t05", "t08", "t09"}},
	}

	up, err := plotter.NewUpSet(sets)
	if err != nil {
		log.Fatalf("could not create UpSet plotter: %+v", err)
	}

	p := plot.New()
	p.Title.Text = "Test tag combinations"
	p.Add(up)
	p.HideX()
	p.Y.Tick.Marker = plot.ConstantTicks(up.Ticks())

	err = p.Save(6*vg.Inch, 4*vg.Inch, "testdata/upset.png")
	if err != nil {
		log.Fatalf("could not save plot: %+v", err)
	}
	err = p.Save(6*vg.Inch, 4*vg.Inch, "testdata/upset.svg")
	if err != nil {
		log.Fatalf("could not save plot: %+v", err)
	}
}
