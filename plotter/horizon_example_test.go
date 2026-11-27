// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter_test

import (
	"log"
	"math"
	"math/rand/v2"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

// Example_horizon draws a horizon chart of GPU frame-time deviations from
// a baseline. Positive and negative deviations are folded into a single
// fixed-height band, with deeper layers using darker colors. A missing
// measurement (NaN) breaks the series into two segments.
func Example_horizon() {
	rnd := rand.New(rand.NewPCG(42, 1))

	const (
		n         = 120
		baseline  = 16.0 // milliseconds per frame
		bandwidth = 4.0  // milliseconds per band
	)

	pts := make(plotter.XYs, n)
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	for i := range n {
		pts[i].X = float64(start.Add(time.Duration(i) * time.Minute).Unix())

		// Mix small jitter with occasional positive and negative spikes.
		v := baseline + 1.5*rnd.NormFloat64()
		switch i {
		case 15, 16, 17:
			v += 7 + rnd.Float64()
		case 60, 61:
			v -= 8 + rnd.Float64()
		case 95:
			v += 11
		}
		pts[i].Y = v
	}
	// A missing measurement disconnects the line.
	pts[45].Y = math.NaN()

	hz, err := plotter.NewHorizon(pts, baseline, bandwidth)
	if err != nil {
		log.Panic(err)
	}

	p := plot.New()
	p.Title.Text = "GPU Frame-Time Deviation"
	p.X.Tick.Marker = plot.TimeTicks{Format: "15:04"}
	p.Y.Label.Text = "ms per frame"
	p.Y.Tick.Marker = horizonTicks{baseline: baseline, bandwidth: bandwidth}

	p.Add(plotter.NewGrid(), hz)

	// One legend entry per layer, labeled with the value range it covers.
	for _, entry := range hz.Legend() {
		p.Legend.Add(entry.Label, entry.Thumbnail)
	}
	p.Legend.Top = true
	p.Legend.ThumbnailWidth = 2 * vg.Millimeter
	p.Legend.TextStyle.Font.Size = 2 * vg.Millimeter

	err = p.Save(16*vg.Centimeter, 6*vg.Centimeter, "testdata/horizon.png")
	if err != nil {
		log.Panic(err)
	}
}

// horizonTicks labels the folded band with its baseline and band edges.
type horizonTicks struct {
	baseline  float64
	bandwidth float64
}

// Ticks implements the plot.Ticker interface.
func (t horizonTicks) Ticks(min, max float64) []plot.Tick {
	return []plot.Tick{
		{Value: t.baseline - t.bandwidth, Label: "−b"},
		{Value: t.baseline, Label: "baseline"},
		{Value: t.baseline + t.bandwidth, Label: "+b"},
	}
}
