// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter_test

import (
	"log"
	"math"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/palette"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

// ExampleCalendarHeatMap draws the daily number of players of a game
// over the calendar year 2024.
// The output can be found at https://github.com/gonum/plot/blob/master/plotter/testdata/calendarHeatMap_golden.png.
func ExampleCalendarHeatMap() {
	// dailyPlayers returns a year of daily player counts with a
	// weekly rhythm and a seasonal trend.
	dailyPlayers := func(year int) plotter.DateValues {
		const day = 24 * time.Hour
		start := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(year+1, time.January, 1, 0, 0, 0, 0, time.UTC)
		var data plotter.DateValues
		for date := start; date.Before(end); date = date.Add(day) {
			season := math.Sin(2 * math.Pi * float64(date.YearDay()) / 365)
			weekend := 0.0
			switch date.Weekday() {
			case time.Saturday, time.Sunday:
				weekend = 400
			}
			data = append(data, plotter.DateValue{
				Date:  date,
				Value: 1000 + 500*season + weekend,
			})
		}
		return data
	}

	h, err := plotter.NewCalendarHeatMap(dailyPlayers(2024), 2024, palette.Heat(12, 1))
	if err != nil {
		log.Fatalf("could not create calendar heat map: %+v", err)
	}

	p := plot.New()
	p.Title.Text = "Daily players, 2024"
	p.HideAxes()
	p.Add(h)

	err = p.Save(10*vg.Inch, 3*vg.Inch, "testdata/calendarHeatMap.png")
	if err != nil {
		log.Fatalf("could not save plot: %+v", err)
	}
	err = p.Save(10*vg.Inch, 3*vg.Inch, "testdata/calendarHeatMap.svg")
	if err != nil {
		log.Fatalf("could not save plot: %+v", err)
	}
}
