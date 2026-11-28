// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/palette"
	"gonum.org/v1/plot/text"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

var (
	// ErrCalendarYear is returned by NewCalendarHeatMap when the
	// requested year is outside the range supported by time.Time.
	ErrCalendarYear = errors.New("plotter: calendar year out of range")

	// ErrCalendarPalette is returned by NewCalendarHeatMap when the
	// provided palette is non-nil but contains no colors.
	ErrCalendarPalette = errors.New("plotter: calendar heat map has an empty palette")

	// ErrCalendarInf is returned by NewCalendarHeatMap when a value
	// belonging to the calendar year is infinite. NaN values are
	// accepted and mark missing measurements.
	ErrCalendarInf = errors.New("plotter: calendar heat map contains an infinite value")
)

const (
	// calendarRows is the number of weekday rows in a calendar grid.
	calendarRows = 7

	// calendarWeekdayBand is the width, in cell units, reserved to
	// the left of the grid for weekday labels.
	calendarWeekdayBand = 3.0

	// calendarMonthBand is the height, in cell units, reserved above
	// the grid for month labels.
	calendarMonthBand = 1.0
)

// defaultCalendarPalette is used when a nil palette is passed to
// NewCalendarHeatMap.
var defaultCalendarPalette = palette.Heat(12, 1)

// A DateValue associates a calendar date with a measured value.
type DateValue struct {
	// Date is the date of the measurement. Only the year, month and
	// day are significant; the time of day and location are ignored
	// when grouping values into calendar cells.
	Date time.Time

	// Value is the measured value. NaN marks a missing measurement.
	// Infinite values are rejected by NewCalendarHeatMap.
	Value float64
}

// DateValuer wraps the Len and DateValue methods.
type DateValuer interface {
	// Len returns the number of date/value pairs.
	Len() int

	// DateValue returns the i-th date and value.
	DateValue(i int) (date time.Time, value float64)
}

// DateValues implements the DateValuer interface using a slice.
type DateValues []DateValue

// Len implements the Len method of the DateValuer interface.
func (d DateValues) Len() int { return len(d) }

// DateValue implements the DateValue method of the DateValuer interface.
func (d DateValues) DateValue(i int) (date time.Time, value float64) {
	return d[i].Date, d[i].Value
}

// CalendarHeatMap implements the plot.Plotter interface, drawing one
// cell per day of a calendar year. Days are arranged with the weeks of
// the year as columns and the days of the week as rows, so that every
// day of every month falls in the column of the week it belongs to and
// the row of its weekday. The leading and trailing cells of a year
// that does not start or end on a week boundary are left empty.
//
// By default weeks start on Monday and the first week is the one that
// contains January 1. The week start is configurable through WeekStart;
// the calendar layout is recomputed from it on every use, so it may be
// changed after construction.
//
// Values whose calendar date is not in the year passed to
// NewCalendarHeatMap are ignored and counted by Skipped. When several
// values share the same date, the last one in input order is used.
// NaN values mark missing measurements and are drawn with the NaN
// color, falling back to the Missing color when NaN is nil. Infinite
// values are rejected by NewCalendarHeatMap.
type CalendarHeatMap struct {
	// WeekStart is the weekday of the first row of the calendar.
	// It defaults to time.Monday. Values outside the range of
	// time.Weekday are reduced modulo seven.
	WeekStart time.Weekday

	// Palette is the color scale used to render measured days.
	// Colors are ordered from the lowest to the highest value.
	// Palette must not be nil or return a zero length []color.Color.
	Palette palette.Palette

	// Missing is the color used to fill days without a measurement.
	// If Missing is nil, those cells are left blank. It defaults to a
	// light gray.
	Missing color.Color

	// NaN is the color used to fill days whose value is NaN. If NaN
	// is nil, NaN days are drawn like days without a measurement, using
	// Missing.
	NaN color.Color

	// Underflow and Overflow are colors used to fill measured days
	// whose value falls outside the dynamic range defined by Min and
	// Max. If either is nil, the corresponding cells are left blank.
	Underflow color.Color
	Overflow  color.Color

	// Min and Max define the dynamic range of the color scale. They
	// are initialized to the smallest and largest finite measurement
	// and may be overridden to use the same scale for several
	// calendars. When Min equals Max every measured day is drawn with
	// the middle color of Palette.
	Min, Max float64

	// MonthLabels determines whether the name of each month is drawn
	// above the first week of that month.
	MonthLabels bool

	// MonthLabelFormat is the time.Format layout used for month
	// labels. It defaults to "Jan".
	MonthLabelFormat string

	// MonthLabelStyle is the style of the month labels.
	MonthLabelStyle text.Style

	// WeekdayLabels determines whether the name of each weekday is
	// drawn to the left of its row.
	WeekdayLabels bool

	// WeekdayLabelFormat is the time.Format layout used for weekday
	// labels. It defaults to "Mon".
	WeekdayLabelFormat string

	// WeekdayLabelStyle is the style of the weekday labels.
	WeekdayLabelStyle text.Style

	// LineStyle is the style used to outline each day cell. A zero
	// width disables the outlines, which is the default.
	LineStyle draw.LineStyle

	// year is the calendar year described by the plotter.
	year int

	// days is the number of days in year.
	days int

	// values holds the resolved measurement of each day, keyed by
	// the day's date. Days without a measurement are absent. NaN
	// values are present and mark missing measurements.
	values map[dateKey]float64

	// skipped counts input values outside year.
	skipped int
}

// dateKey is the calendar date of a measurement, ignoring the time
// of day and the location.
type dateKey struct {
	year  int
	month time.Month
	day   int
}

func keyOf(t time.Time) dateKey {
	y, m, d := t.Date()
	return dateKey{year: y, month: m, day: d}
}

// NewCalendarHeatMap returns a CalendarHeatMap for the given year and
// data, using the provided palette. If pal is nil, a default heat
// palette is used.
//
// Values whose date is not in year are ignored. When several values
// share a date, the last one in input order is used. NaN values mark
// missing days and do not contribute to Min and Max. An infinite value
// on a date of year is rejected with ErrCalendarInf. A year outside
// the range supported by time.Time is rejected with ErrCalendarYear,
// and a non-nil palette without colors is rejected with
// ErrCalendarPalette.
func NewCalendarHeatMap(data DateValuer, year int, pal palette.Palette) (*CalendarHeatMap, error) {
	if year < 1 || year > 9999 {
		return nil, fmt.Errorf("%w: %d", ErrCalendarYear, year)
	}
	if pal == nil {
		pal = defaultCalendarPalette
	}
	if len(pal.Colors()) == 0 {
		return nil, ErrCalendarPalette
	}

	days := 365
	if isLeapYear(year) {
		days = 366
	}

	values := make(map[dateKey]float64)
	skipped := 0
	if data != nil {
		for i := range data.Len() {
			date, value := data.DateValue(i)
			key := keyOf(date)
			if key.year != year {
				skipped++
				continue
			}
			if math.IsInf(value, 0) {
				return nil, fmt.Errorf("%w: %v on %s",
					ErrCalendarInf, value, date.Format("2006-01-02"))
			}
			// Later values replace earlier ones, including NaN.
			values[key] = value
		}
	}

	min, max := math.Inf(1), math.Inf(-1)
	for _, v := range values {
		if math.IsNaN(v) {
			continue
		}
		min = math.Min(min, v)
		max = math.Max(max, v)
	}
	if math.IsInf(min, 1) {
		min, max = 0, 0
	}

	h := &CalendarHeatMap{
		WeekStart:          time.Monday,
		Palette:            pal,
		Missing:            color.Gray{Y: 0xe6},
		Min:                min,
		Max:                max,
		MonthLabels:        true,
		MonthLabelFormat:   "Jan",
		WeekdayLabels:      true,
		WeekdayLabelFormat: "Mon",
		year:               year,
		days:               days,
		values:             values,
		skipped:            skipped,
	}

	h.MonthLabelStyle = text.Style{
		Color:   color.Black,
		Font:    font.From(plot.DefaultFont, vg.Points(9)),
		XAlign:  draw.XCenter,
		YAlign:  draw.YBottom,
		Handler: plot.DefaultTextHandler,
	}
	h.WeekdayLabelStyle = text.Style{
		Color:   color.Black,
		Font:    font.From(plot.DefaultFont, vg.Points(9)),
		XAlign:  draw.XRight,
		YAlign:  draw.YCenter,
		Handler: plot.DefaultTextHandler,
	}

	return h, nil
}

// isLeapYear reports whether year is a leap year in the Gregorian
// calendar.
func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// Year returns the calendar year described by the plotter.
func (h *CalendarHeatMap) Year() int { return h.year }

// Days returns the number of days in the calendar year.
func (h *CalendarHeatMap) Days() int { return h.days }

// Count returns the number of days that have a measurement, including
// days whose value is NaN.
func (h *CalendarHeatMap) Count() int { return len(h.values) }

// Skipped returns the number of input values that were ignored because
// their date is not in the calendar year.
func (h *CalendarHeatMap) Skipped() int { return h.skipped }

// Value returns the measurement for date and whether a measurement is
// present. A NaN value is present and is reported as NaN. Dates outside
// the calendar year have no measurement.
func (h *CalendarHeatMap) Value(date time.Time) (value float64, ok bool) {
	key := keyOf(date)
	if key.year != h.year {
		return 0, false
	}
	value, ok = h.values[key]
	return value, ok
}

// weekStart returns WeekStart reduced to a valid time.Weekday.
func (h *CalendarHeatMap) weekStart() time.Weekday {
	return time.Weekday((int(h.WeekStart)%7 + 7) % 7)
}

// lead returns the number of empty cells before January 1 in the first
// week column.
func (h *CalendarHeatMap) lead() int {
	jan1 := time.Date(h.year, time.January, 1, 0, 0, 0, 0, time.UTC)
	return (int(jan1.Weekday()) - int(h.weekStart()) + 7) % 7
}

// Weeks returns the number of week columns needed to lay out the year.
func (h *CalendarHeatMap) Weeks() int {
	return (h.lead() + h.days + 6) / 7
}

// Cell returns the zero-based column and row of date in the calendar
// grid, and whether the date belongs to the calendar year. Column zero
// is the first week of the year and row zero is the week start weekday.
func (h *CalendarHeatMap) Cell(date time.Time) (col, row int, ok bool) {
	key := keyOf(date)
	if key.year != h.year {
		return 0, 0, false
	}
	day := time.Date(key.year, key.month, key.day, 0, 0, 0, 0, time.UTC).YearDay() - 1
	pos := h.lead() + day
	return pos / 7, pos % 7, true
}

// CellX returns the x data coordinate of the center of column col.
func (h *CalendarHeatMap) CellX(col int) float64 { return float64(col) }

// CellY returns the y data coordinate of the center of row row. Row
// zero is drawn at the top of the grid.
func (h *CalendarHeatMap) CellY(row int) float64 {
	return float64(calendarRows - 1 - row)
}

// Plot implements the Plot method of the plot.Plotter interface.
func (h *CalendarHeatMap) Plot(c draw.Canvas, plt *plot.Plot) {
	if h.Palette == nil {
		panic("plotter: calendar heat map has no palette")
	}
	pal := h.Palette.Colors()
	if len(pal) == 0 {
		panic("plotter: calendar heat map has an empty palette")
	}
	if h.Min > h.Max {
		panic("plotter: calendar heat map: invalid range: min greater than max")
	}

	trX, trY := plt.Transforms(&c)
	lead := h.lead()
	start := time.Date(h.year, time.January, 1, 0, 0, 0, 0, time.UTC)

	for day := range h.days {
		pos := lead + day
		col, row := pos/7, pos%7
		date := start.AddDate(0, 0, day)

		var fill color.Color
		value, has := h.values[keyOf(date)]
		switch {
		case !has:
			fill = h.Missing
		case math.IsNaN(value):
			fill = h.NaN
			if fill == nil {
				fill = h.Missing
			}
		case value < h.Min:
			fill = h.Underflow
		case value > h.Max:
			fill = h.Overflow
		default:
			fill = pal[h.paletteIndex(value, len(pal))]
		}
		if fill == nil {
			continue
		}

		h.plotCell(c, trX, trY, col, row, fill)
	}

	if h.LineStyle.Width > 0 {
		for day := range h.days {
			pos := lead + day
			h.plotCellOutline(c, trX, trY, pos/7, pos%7)
		}
	}

	if h.MonthLabels {
		h.plotMonthLabels(c, trX, trY)
	}
	if h.WeekdayLabels {
		h.plotWeekdayLabels(c, trX, trY)
	}
}

// paletteIndex returns the index of the palette color for value.
func (h *CalendarHeatMap) paletteIndex(value float64, n int) int {
	if n == 1 || h.Max == h.Min {
		return n / 2
	}
	i := int((value-h.Min)/(h.Max-h.Min)*float64(n-1) + 0.5)
	return min(max(i, 0), n-1)
}

// cellRect returns the corners of the cell at col, row in canvas
// coordinates.
func cellRect(trX, trY func(float64) vg.Length, col, row int) []vg.Point {
	x, y := float64(col), float64(calendarRows-1-row)
	x0, x1 := trX(x-0.5), trX(x+0.5)
	y0, y1 := trY(y-0.5), trY(y+0.5)
	return []vg.Point{
		{X: x0, Y: y0},
		{X: x1, Y: y0},
		{X: x1, Y: y1},
		{X: x0, Y: y1},
	}
}

// plotCell fills the cell at col, row.
func (h *CalendarHeatMap) plotCell(c draw.Canvas, trX, trY func(float64) vg.Length, col, row int, fill color.Color) {
	pts := c.ClipPolygonXY(cellRect(trX, trY, col, row))
	if len(pts) < 3 {
		return
	}
	c.FillPolygon(fill, pts)
}

// plotCellOutline strokes the outline of the cell at col, row.
func (h *CalendarHeatMap) plotCellOutline(c draw.Canvas, trX, trY func(float64) vg.Length, col, row int) {
	pts := cellRect(trX, trY, col, row)
	pts = append(pts, pts[0])
	c.StrokeLines(h.LineStyle, c.ClipLinesXY(pts)...)
}

// plotMonthLabels draws the month labels above the grid, centered on
// the columns that hold each month.
func (h *CalendarHeatMap) plotMonthLabels(c draw.Canvas, trX, trY func(float64) vg.Length) {
	y := trY(float64(calendarRows-1) + 0.5 + 0.15)
	for month := time.January; month <= time.December; month++ {
		first := time.Date(h.year, month, 1, 0, 0, 0, 0, time.UTC)
		last := first.AddDate(0, 1, -1)
		firstCol, _, _ := h.Cell(first)
		lastCol, _, _ := h.Cell(last)
		x := trX(float64(firstCol+lastCol) / 2)
		pt := vg.Point{X: x, Y: y}
		if !c.Contains(pt) {
			continue
		}
		label := first.Format(h.MonthLabelFormat)
		c.FillText(h.MonthLabelStyle, pt, label)
	}
}

// plotWeekdayLabels draws the weekday labels to the left of the grid.
func (h *CalendarHeatMap) plotWeekdayLabels(c draw.Canvas, trX, trY func(float64) vg.Length) {
	x := trX(-0.6)
	// Choose a reference date that starts on a Sunday so that the
	// weekday of each row can be formatted with any layout.
	sunday := time.Date(2000, time.January, 2, 0, 0, 0, 0, time.UTC)
	start := h.weekStart()
	for row := range calendarRows {
		day := time.Weekday((int(start) + row) % 7)
		date := sunday.AddDate(0, 0, int(day))
		pt := vg.Point{X: x, Y: trY(h.CellY(row))}
		if !c.Contains(pt) {
			continue
		}
		c.FillText(h.WeekdayLabelStyle, pt, date.Format(h.WeekdayLabelFormat))
	}
}

// DataRange implements the DataRange method of the plot.DataRanger
// interface. The range includes the space reserved for enabled labels.
func (h *CalendarHeatMap) DataRange() (xmin, xmax, ymin, ymax float64) {
	xmin = -0.5
	if h.WeekdayLabels {
		xmin -= calendarWeekdayBand
	}
	xmax = float64(h.Weeks()) - 0.5
	ymin = -0.5
	ymax = float64(calendarRows) - 0.5
	if h.MonthLabels {
		ymax += calendarMonthBand
	}
	return xmin, xmax, ymin, ymax
}
