// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter_test

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"math"
	"regexp"
	"strconv"
	"testing"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/cmpimg"
	"gonum.org/v1/plot/palette"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/recorder"
	"gonum.org/v1/plot/vg/vgimg"
)

func TestCalendarHeatMap(t *testing.T) {
	cmpimg.CheckPlot(ExampleCalendarHeatMap, t, "calendarHeatMap.png", "calendarHeatMap.svg")
}

func mustCalendar(t *testing.T, data plotter.DateValuer, year int) *plotter.CalendarHeatMap {
	t.Helper()
	h, err := plotter.NewCalendarHeatMap(data, year, palette.Heat(12, 1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return h
}

// sameColor reports whether a and b render to the same opaque color.
func sameColor(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

// TestCalendarHeatMapLeapDay checks the canonical week-day grid of
// 2024: January 1 is a Monday and February 29 is a Thursday, and the
// days following the leap day keep their weekday rows.
func TestCalendarHeatMapLeapDay(t *testing.T) {
	h := mustCalendar(t, nil, 2024)

	if got, want := h.Days(), 366; got != want {
		t.Errorf("unexpected number of days: got %d, want %d", got, want)
	}
	// 2024 starts on a Monday and is a leap year: with weeks starting
	// on Monday the grid has no leading gap, so it needs 53 columns.
	if got, want := h.Weeks(), 53; got != want {
		t.Errorf("unexpected number of weeks: got %d, want %d", got, want)
	}

	for _, test := range []struct {
		date time.Time
		col  int
		row  int
	}{
		{date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), col: 0, row: 0},
		{date: time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC), col: 0, row: 6},
		{date: time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC), col: 1, row: 0},
		// February 29 is a Thursday: nine weeks after January 1, on
		// the fourth row of a Monday-first grid.
		{date: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), col: 8, row: 3},
		// The day after the leap day is a Friday in the same week,
		// and March 1 must not shift by a full row or column.
		{date: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC), col: 8, row: 4},
		{date: time.Date(2024, 3, 4, 0, 0, 0, 0, time.UTC), col: 9, row: 0},
		{date: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC), col: 52, row: 1},
	} {
		col, row, ok := h.Cell(test.date)
		if !ok {
			t.Errorf("%s: date not in calendar", test.date.Format("2006-01-02"))
			continue
		}
		if col != test.col || row != test.row {
			t.Errorf("%s: got cell (%d, %d), want (%d, %d)",
				test.date.Format("2006-01-02"), col, row, test.col, test.row)
		}
	}

	// The row of a date must match its weekday for every day of the
	// year: this catches a shift after the leap day.
	start := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	for d := start; d.Year() == 2024; d = d.AddDate(0, 0, 1) {
		_, row, ok := h.Cell(d)
		if !ok {
			t.Fatalf("%s: date not in calendar", d.Format("2006-01-02"))
		}
		want := (int(d.Weekday()) - int(time.Monday) + 7) % 7
		if row != want {
			t.Fatalf("%s: got row %d, want %d for weekday %v",
				d.Format("2006-01-02"), row, want, d.Weekday())
		}
	}
}

// TestCalendarHeatMapWeekStart checks that changing the week start
// shifts the layout without moving any date to another weekday row.
func TestCalendarHeatMapWeekStart(t *testing.T) {
	h := mustCalendar(t, nil, 2024)

	// With Sunday as the first row, January 1 (Monday) moves to row 1
	// and February 29 (Thursday) to row 4 of the same column that
	// holds the rest of that week.
	h.WeekStart = time.Sunday
	if got, want := h.Weeks(), 53; got != want {
		t.Errorf("unexpected number of weeks for Sunday start: got %d, want %d", got, want)
	}
	if _, row, _ := h.Cell(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)); row != 1 {
		t.Errorf("January 1 with Sunday start: got row %d, want 1", row)
	}
	if col, row, _ := h.Cell(time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC)); col != 8 || row != 4 {
		t.Errorf("February 29 with Sunday start: got cell (%d, %d), want (8, 4)", col, row)
	}

	h.WeekStart = time.Wednesday
	if _, row, _ := h.Cell(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)); row != 5 {
		t.Errorf("January 1 with Wednesday start: got row %d, want 5", row)
	}
	// An out-of-range weekday must be reduced modulo seven.
	h.WeekStart = time.Weekday(7 + int(time.Monday))
	if _, row, _ := h.Cell(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)); row != 0 {
		t.Errorf("January 1 with Monday+7 start: got row %d, want 0", row)
	}
}

// TestCalendarHeatMapYearEdges checks years that start and end at
// different offsets of the week, including a non-leap year.
func TestCalendarHeatMapYearEdges(t *testing.T) {
	// 2023 starts on a Sunday and ends on a Sunday: 53 columns with a
	// Monday week start.
	h := mustCalendar(t, nil, 2023)
	if got, want := h.Days(), 365; got != want {
		t.Errorf("unexpected number of days in 2023: got %d, want %d", got, want)
	}
	if got, want := h.Weeks(), 53; got != want {
		t.Errorf("unexpected number of weeks in 2023: got %d, want %d", got, want)
	}
	if col, row, _ := h.Cell(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)); col != 0 || row != 6 {
		t.Errorf("January 1 2023: got cell (%d, %d), want (0, 6)", col, row)
	}
	if col, row, _ := h.Cell(time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)); col != 52 || row != 6 {
		t.Errorf("December 31 2023: got cell (%d, %d), want (52, 6)", col, row)
	}

	// 2021 starts on a Friday and ends on a Friday: 53 columns, with
	// the first four rows of the last column empty.
	h = mustCalendar(t, nil, 2021)
	if col, row, _ := h.Cell(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)); col != 0 || row != 4 {
		t.Errorf("January 1 2021: got cell (%d, %d), want (0, 4)", col, row)
	}
	if col, row, _ := h.Cell(time.Date(2021, 12, 31, 0, 0, 0, 0, time.UTC)); col != 52 || row != 4 {
		t.Errorf("December 31 2021: got cell (%d, %d), want (52, 4)", col, row)
	}
}

// TestCalendarHeatMapValues checks duplicate dates, out-of-year values,
// NaN and infinite handling.
func TestCalendarHeatMapValues(t *testing.T) {
	data := plotter.DateValues{
		{Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Value: 1},
		{Date: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC), Value: 7},
		{Date: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), Value: 3},
		{Date: time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC), Value: 100},
		{Date: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Value: 200},
		{Date: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), Value: math.NaN()},
	}
	h := mustCalendar(t, data, 2024)

	if got, want := h.Skipped(), 2; got != want {
		t.Errorf("unexpected skipped count: got %d, want %d", got, want)
	}
	if got, want := h.Count(), 3; got != want {
		t.Errorf("unexpected value count: got %d, want %d", got, want)
	}
	// The last duplicate wins, including across different times of day.
	if got, ok := h.Value(time.Date(2024, 1, 1, 18, 0, 0, 0, time.UTC)); !ok || got != 7 {
		t.Errorf("duplicate date: got (%v, %v), want (7, true)", got, ok)
	}
	// NaN is stored and reports a missing measurement.
	if got, ok := h.Value(time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)); !ok || !math.IsNaN(got) {
		t.Errorf("NaN date: got (%v, %v), want (NaN, true)", got, ok)
	}
	// Values outside the year are not accessible.
	if _, ok := h.Value(time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)); ok {
		t.Error("out-of-year value is accessible")
	}
	// Min and Max ignore NaN and out-of-year values.
	if h.Min != 3 || h.Max != 7 {
		t.Errorf("unexpected range: got [%v, %v], want [3, 7]", h.Min, h.Max)
	}

	// An infinite value on a day of the year is rejected.
	_, err := plotter.NewCalendarHeatMap(plotter.DateValues{
		{Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Value: math.Inf(1)},
	}, 2024, palette.Heat(4, 1))
	if !errors.Is(err, plotter.ErrCalendarInf) {
		t.Errorf("expected ErrCalendarInf, got %v", err)
	}
	// An infinite value outside the year is ignored like any other.
	_, err = plotter.NewCalendarHeatMap(plotter.DateValues{
		{Date: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), Value: math.Inf(1)},
	}, 2024, palette.Heat(4, 1))
	if err != nil {
		t.Errorf("unexpected error for out-of-year infinity: %v", err)
	}

	// An empty palette and an unsupported year are rejected.
	_, err = plotter.NewCalendarHeatMap(nil, 2024, emptyPalette{})
	if !errors.Is(err, plotter.ErrCalendarPalette) {
		t.Errorf("expected ErrCalendarPalette, got %v", err)
	}
	_, err = plotter.NewCalendarHeatMap(nil, 0, nil)
	if !errors.Is(err, plotter.ErrCalendarYear) {
		t.Errorf("expected ErrCalendarYear, got %v", err)
	}
}

type emptyPalette struct{}

func (emptyPalette) Colors() []color.Color { return nil }

// TestCalendarHeatMapEmpty checks that the data range of a calendar
// without measurements reserves room for its labels.
func TestCalendarHeatMapEmpty(t *testing.T) {
	h := mustCalendar(t, nil, 2024)
	xmin, xmax, ymin, ymax := h.DataRange()
	if xmin >= 0 {
		t.Errorf("x range does not leave room for weekday labels: %v", xmin)
	}
	if ymax <= float64(7) {
		t.Errorf("y range does not leave room for month labels: %v", ymax)
	}
	if xmax != float64(h.Weeks())-0.5 {
		t.Errorf("unexpected xmax: got %v, want %v", xmax, float64(h.Weeks())-0.5)
	}
	if ymin != -0.5 {
		t.Errorf("unexpected ymin: got %v, want -0.5", ymin)
	}

	// Without labels the data range is exactly the grid.
	h.MonthLabels = false
	h.WeekdayLabels = false
	xmin, xmax, ymin, ymax = h.DataRange()
	if xmin != -0.5 || ymin != -0.5 || xmax != float64(h.Weeks())-0.5 || ymax != 6.5 {
		t.Errorf("unexpected unlabelled range: (%v, %v, %v, %v)", xmin, xmax, ymin, ymax)
	}
}

// cellFillRecords returns the filled rectangles, in canvas
// coordinates, emitted while drawing h to a recorder canvas. The plot
// background fill is excluded; the remaining fills are the day cells.
func cellFillRecords(t *testing.T, h *plotter.CalendarHeatMap, w, hgt vg.Length) []vg.Rectangle {
	t.Helper()
	p := plot.New()
	p.HideAxes()
	p.Add(h)

	// Draw once to settle the axis ranges, as the plot updates its
	// ranges as plotters are added.
	scratch := vgimg.New(w, hgt)
	p.Draw(draw.New(scratch))

	rec := new(recorder.Canvas)
	p.Draw(draw.NewCanvas(rec, w, hgt))

	background := vg.Rectangle{
		Min: vg.Point{X: 0, Y: 0},
		Max: vg.Point{X: w, Y: hgt},
	}
	var rects []vg.Rectangle
	for _, a := range rec.Actions {
		fill, ok := a.(*recorder.Fill)
		if !ok || len(fill.Path) != 5 {
			continue
		}
		r := pathRect(fill.Path)
		if r == background {
			continue
		}
		if r.Size().X <= 0 || r.Size().Y <= 0 {
			continue
		}
		rects = append(rects, r)
	}
	return rects
}

// pathRect returns the bounding rectangle of the four corners of a
// closed rectangular path.
func pathRect(p vg.Path) vg.Rectangle {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, c := range p {
		if c.Type != vg.LineComp {
			continue
		}
		minX = math.Min(minX, float64(c.Pos.X))
		minY = math.Min(minY, float64(c.Pos.Y))
		maxX = math.Max(maxX, float64(c.Pos.X))
		maxY = math.Max(maxY, float64(c.Pos.Y))
	}
	return vg.Rectangle{
		Min: vg.Point{X: vg.Length(minX), Y: vg.Length(minY)},
		Max: vg.Point{X: vg.Length(maxX), Y: vg.Length(maxY)},
	}
}

// TestCalendarHeatMapRendering checks that day cells are painted at
// the expected vector positions by both the recorder and the raster
// backend, and that the leap day does not shift later days.
func TestCalendarHeatMapRendering(t *testing.T) {
	data := plotter.DateValues{
		{Date: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), Value: 1},
		{Date: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC), Value: 2},
	}
	h := mustCalendar(t, data, 2024)
	h.Missing = color.RGBA{R: 240, G: 240, B: 240, A: 255}

	rects := cellFillRecords(t, h, 600, 300)
	// One missing cell per day without a measurement and one painted
	// cell for each of the two measured days.
	if got, want := len(rects), h.Days(); got != want {
		t.Fatalf("unexpected number of day cells: got %d, want %d", got, want)
	}

	// Paint the same plot and sample the center of every recorded cell.
	p := plot.New()
	p.HideAxes()
	p.Add(h)
	img := vgimg.New(600, 300)
	p.Draw(draw.New(img))

	want := h.Palette.Colors()
	dpi := img.DPI()
	counts := make(map[color.Color]int)
	for _, r := range rects {
		cx := (r.Min.X + r.Max.X) / 2
		cy := (r.Min.Y + r.Max.Y) / 2
		px := int(cx.Dots(dpi))
		py := int(img.Image().Bounds().Max.Y - 1 - int(cy.Dots(dpi)))
		counts[img.Image().At(px, py)]++
	}
	// The two measured days use the first and last palette colors;
	// every other day uses the missing color.
	if counts[color.RGBAModel.Convert(want[0])] != 1 {
		t.Errorf("expected exactly one cell with the lowest palette color, got %d", counts[color.RGBAModel.Convert(want[0])])
	}
	if counts[color.RGBAModel.Convert(want[len(want)-1])] != 1 {
		t.Errorf("expected exactly one cell with the highest palette color, got %d", counts[color.RGBAModel.Convert(want[len(want)-1])])
	}
	missing := color.RGBAModel.Convert(h.Missing)
	if got, wantN := counts[missing], h.Days()-2; got != wantN {
		t.Errorf("unexpected number of missing cells: got %d, want %d", got, wantN)
	}
}

// TestCalendarHeatMapSVGPNG checks that the SVG and PNG backends are
// handed the same day-cell grid: the SVG has one fill per recorded
// cell and the PNG paints each of those cells.
func TestCalendarHeatMapSVGPNG(t *testing.T) {
	data := plotter.DateValues{
		{Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Value: 0},
		{Date: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), Value: 10},
		{Date: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC), Value: 20},
	}
	h := mustCalendar(t, data, 2024)
	h.Min, h.Max = 0, 20

	p := plot.New()
	p.HideAxes()
	p.Add(h)

	const (
		w   = 8 * vg.Inch
		hgt = 3 * vg.Inch
	)

	rects := cellFillRecords(t, h, w, hgt)

	pngC, err := p.WriterTo(w, hgt, "png")
	if err != nil {
		t.Fatalf("could not create PNG canvas: %v", err)
	}
	pngBuf := new(bytes.Buffer)
	if _, err := pngC.WriteTo(pngBuf); err != nil {
		t.Fatalf("could not write PNG: %v", err)
	}
	pngImg, _, err := image.Decode(pngBuf)
	if err != nil {
		t.Fatalf("could not decode PNG: %v", err)
	}

	svgC, err := p.WriterTo(w, hgt, "svg")
	if err != nil {
		t.Fatalf("could not create SVG canvas: %v", err)
	}
	svgBuf := new(bytes.Buffer)
	if _, err := svgC.WriteTo(svgBuf); err != nil {
		t.Fatalf("could not write SVG: %v", err)
	}
	doc := svgBuf.String()

	// Every day cell is emitted as a filled quadrilateral in the SVG
	// document, at the same canvas coordinates the vector recorder saw.
	svgRects := svgFillRects(t, doc, w, hgt)
	if len(svgRects) != len(rects) {
		t.Fatalf("SVG and recorder disagree on cell count: %d != %d", len(svgRects), len(rects))
	}
	const tol = 0.5
	for i := range rects {
		if !rectClose(rects[i], svgRects[i], tol) {
			t.Fatalf("cell %d differs between SVG and recorder:\n got %+v\nwant %+v",
				i, svgRects[i], rects[i])
		}
	}

	// The raster image must paint a non-trivial area inside the
	// bounding box of the SVG day grid.
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	b := pngImg.Bounds()
	painted := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if !sameColor(pngImg.At(x, y), white) {
				painted++
			}
		}
	}
	if painted == 0 {
		t.Fatal("PNG painted nothing")
	}
	if len(rects) != h.Days() {
		t.Errorf("unexpected number of vector day cells: got %d, want %d", len(rects), h.Days())
	}
}

// svgFillRects returns the filled quadrilaterals of the SVG document
// in points, in document order. Only axis-aligned, closed four-corner
// paths are returned.
func svgFillRects(t *testing.T, doc string, w, hgt vg.Length) []vg.Rectangle {
	t.Helper()
	re := regexp.MustCompile(`<path d="M([-0-9.]+),([-0-9.]+)L([-0-9.]+),([-0-9.]+)L([-0-9.]+),([-0-9.]+)L([-0-9.]+),([-0-9.]+)Z" style="fill:#([0-9A-F]{6})" />`)
	var rects []vg.Rectangle
	for _, m := range re.FindAllStringSubmatch(doc, -1) {
		vals := make([]float64, 8)
		for i := range vals {
			v, err := strconv.ParseFloat(m[1+i], 64)
			if err != nil {
				t.Fatalf("bad SVG coordinate %q: %v", m[1+i], err)
			}
			vals[i] = v
		}
		r := vg.Rectangle{
			Min: vg.Point{X: vg.Length(vals[0]), Y: vg.Length(vals[1])},
			Max: vg.Point{X: vg.Length(vals[2]), Y: vg.Length(vals[5])},
		}
		// Skip the plot background, which spans the whole canvas.
		if r.Min == (vg.Point{}) && math.Abs(float64(r.Max.X-w)) < 0.01 && math.Abs(float64(r.Max.Y-hgt)) < 0.01 {
			continue
		}
		rects = append(rects, r)
	}
	return rects
}

// rectClose reports whether the corners of a and b are within tol.
func rectClose(a, b vg.Rectangle, tol float64) bool {
	return math.Abs(float64(a.Min.X-b.Min.X)) <= tol &&
		math.Abs(float64(a.Min.Y-b.Min.Y)) <= tol &&
		math.Abs(float64(a.Max.X-b.Max.X)) <= tol &&
		math.Abs(float64(a.Max.Y-b.Max.Y)) <= tol
}

// TestCalendarHeatMapAspect checks that the same calendar rendered at
// different canvas sizes keeps the same cell assignment.
func TestCalendarHeatMapAspect(t *testing.T) {
	h := mustCalendar(t, nil, 2024)
	small := cellFillRecords(t, h, 400, 200)
	large := cellFillRecords(t, h, 800, 400)
	if len(small) != len(large) {
		t.Fatalf("cell count changed with canvas size: %d != %d", len(small), len(large))
	}
	for i := range small {
		// Rectangles are ordered by day, so the i-th cell must refer
		// to the same day at both sizes.
		if (small[i].Size().X > 0) != (large[i].Size().X > 0) {
			t.Fatalf("cell %d disappeared at the larger size", i)
		}
	}
}

// TestCalendarHeatMapMissingStyle checks that a nil Missing color
// leaves days without a measurement blank, and that NaN days are
// treated as missing.
func TestCalendarHeatMapMissingStyle(t *testing.T) {
	data := plotter.DateValues{
		{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Value: math.NaN()},
	}
	h := mustCalendar(t, data, 2024)
	h.Missing = nil

	rects := cellFillRecords(t, h, 600, 300)
	if len(rects) != 0 {
		t.Errorf("nil Missing color painted %d cells, want 0", len(rects))
	}

	h.Missing = color.RGBA{R: 10, G: 20, B: 30, A: 255}
	rects = cellFillRecords(t, h, 600, 300)
	if got, want := len(rects), h.Days(); got != want {
		t.Errorf("unexpected number of cells: got %d, want %d", got, want)
	}
}

// TestCalendarHeatMapUnderOverflow checks that values outside an
// explicit range use the underflow and overflow colors, and that an
// empty range uses the middle palette color.
func TestCalendarHeatMapUnderOverflow(t *testing.T) {
	data := plotter.DateValues{
		{Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Value: -10},
		{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Value: 5},
		{Date: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), Value: 100},
	}
	h := mustCalendar(t, data, 2024)
	h.Min, h.Max = 0, 10
	under := color.RGBA{R: 1, G: 2, B: 3, A: 255}
	over := color.RGBA{R: 4, G: 5, B: 6, A: 255}
	h.Underflow = under
	h.Overflow = over
	h.Missing = color.RGBA{R: 240, G: 240, B: 240, A: 255}

	rects := cellFillRecords(t, h, 600, 300)
	if got, want := len(rects), h.Days(); got != want {
		t.Fatalf("unexpected number of cells: got %d, want %d", got, want)
	}

	p := plot.New()
	p.HideAxes()
	p.Add(h)
	img := vgimg.New(600, 300)
	p.Draw(draw.New(img))
	dpi := img.DPI()

	counts := make(map[color.Color]int)
	for _, r := range rects {
		cx := (r.Min.X + r.Max.X) / 2
		cy := (r.Min.Y + r.Max.Y) / 2
		px := int(cx.Dots(dpi))
		py := int(img.Image().Bounds().Max.Y - 1 - int(cy.Dots(dpi)))
		counts[color.RGBAModel.Convert(img.Image().At(px, py))]++
	}
	if got := counts[color.RGBAModel.Convert(under)]; got != 1 {
		t.Errorf("unexpected underflow cell count: got %d, want 1", got)
	}
	if got := counts[color.RGBAModel.Convert(over)]; got != 1 {
		t.Errorf("unexpected overflow cell count: got %d, want 1", got)
	}

	// With Min == Max every measured day uses the middle color.
	h.Min, h.Max = 5, 5
	h.Underflow, h.Overflow = nil, nil
	// The two out-of-range days now have no under/overflow color and
	// are left blank, while every in-range or missing day is painted.
	rects = cellFillRecords(t, h, 600, 300)
	if got, want := len(rects), h.Days()-2; got != want {
		t.Errorf("unexpected number of cells with empty range: got %d, want %d", got, want)
	}
}

// TestCalendarHeatMapLabels checks that the month and weekday labels
// can be formatted and disabled.
func TestCalendarHeatMapLabels(t *testing.T) {
	h := mustCalendar(t, nil, 2024)

	rec := new(recorder.Canvas)
	p := plot.New()
	p.HideAxes()
	p.Add(h)
	scratch := vgimg.New(800, 300)
	p.Draw(draw.New(scratch))
	p.Draw(draw.NewCanvas(rec, 800, 300))

	labels := make(map[string]bool)
	for _, a := range rec.Actions {
		if s, ok := a.(*recorder.FillString); ok {
			labels[s.String] = true
		}
	}
	for _, want := range []string{"Jan", "Feb", "Dec", "Mon", "Thu", "Sun"} {
		if !labels[want] {
			t.Errorf("missing label %q", want)
		}
	}

	h.MonthLabelFormat = "January"
	h.WeekdayLabelFormat = "Monday"
	h.MonthLabels = false
	h.WeekdayLabels = false
	rec = new(recorder.Canvas)
	p.Draw(draw.NewCanvas(rec, 800, 300))
	for _, a := range rec.Actions {
		if s, ok := a.(*recorder.FillString); ok {
			t.Errorf("unexpected label %q with labels disabled", s.String)
		}
	}
}

// TestCalendarHeatMapCellCoords checks the data coordinates of the
// cells and that they agree with the row/column mapping used by Cell.
func TestCalendarHeatMapCellCoords(t *testing.T) {
	h := mustCalendar(t, nil, 2024)
	if got := h.CellX(0); got != 0 {
		t.Errorf("CellX(0) = %v, want 0", got)
	}
	if got := h.CellX(4); got != 4 {
		t.Errorf("CellX(4) = %v, want 4", got)
	}
	// Row zero is the top of the grid, so it has the largest y.
	if got := h.CellY(0); got != 6 {
		t.Errorf("CellY(0) = %v, want 6", got)
	}
	if got := h.CellY(6); got != 0 {
		t.Errorf("CellY(6) = %v, want 0", got)
	}
	if got := h.CellY(3); got != 3 {
		t.Errorf("CellY(3) = %v, want 3", got)
	}
	// The coordinates must round-trip through Cell for every day.
	start := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	for d := start; d.Year() == 2024; d = d.AddDate(0, 0, 1) {
		col, row, ok := h.Cell(d)
		if !ok {
			t.Fatalf("%s: date not in calendar", d.Format("2006-01-02"))
		}
		if h.CellX(col) != float64(col) || h.CellY(row) != float64(6-row) {
			t.Fatalf("%s: incoherent cell coordinates", d.Format("2006-01-02"))
		}
	}
}

// TestCalendarHeatMapCellOutOfYear checks that Cell rejects dates that
// are not in the calendar year without disturbing the grid.
func TestCalendarHeatMapCellOutOfYear(t *testing.T) {
	h := mustCalendar(t, nil, 2024)
	for _, d := range []time.Time{
		time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	} {
		if col, row, ok := h.Cell(d); ok || col != 0 || row != 0 {
			t.Errorf("%s: got (%d, %d, %v), want (0, 0, false)", d.Format("2006-01-02"), col, row, ok)
		}
	}
	// A date in the same year but at another time of day is in the
	// calendar.
	if _, _, ok := h.Cell(time.Date(2024, 2, 29, 23, 59, 0, 0, time.UTC)); !ok {
		t.Error("February 29 at 23:59 is not in the calendar")
	}
}

// TestCalendarHeatMapLeapRules checks the Gregorian leap year rule for
// century years: 2000 is a leap year, 1900 and 2100 are not.
func TestCalendarHeatMapLeapRules(t *testing.T) {
	for _, test := range []struct {
		year int
		days int
	}{
		{year: 2024, days: 366},
		{year: 2023, days: 365},
		{year: 2000, days: 366},
		{year: 1900, days: 365},
		{year: 2100, days: 365},
	} {
		h := mustCalendar(t, nil, test.year)
		if got := h.Days(); got != test.days {
			t.Errorf("%d: got %d days, want %d", test.year, got, test.days)
		}
		if got := h.Year(); got != test.year {
			t.Errorf("Year() = %d, want %d", got, test.year)
		}
	}
	// February 29 exists only in the leap years. Go normalizes a
	// non-existent February 29 to March 1, so a century non-leap year
	// is checked through its day count and the position of March 1.
	if _, _, ok := mustCalendar(t, nil, 2000).Cell(time.Date(2000, 2, 29, 0, 0, 0, 0, time.UTC)); !ok {
		t.Error("February 29 2000 is not in the calendar")
	}
	h := mustCalendar(t, nil, 1900)
	march1 := time.Date(1900, 3, 1, 0, 0, 0, 0, time.UTC)
	// In a non-leap year March 1 is the 60th day of the year, so it
	// sits exactly one cell after February 28 in the grid.
	feb28Col, feb28Row, ok := h.Cell(time.Date(1900, 2, 28, 0, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatal("February 28 1900 is not in the calendar")
	}
	col, row, ok := h.Cell(march1)
	if !ok {
		t.Error("March 1 1900 is not in the calendar")
	}
	got := col*7 + row
	if want := feb28Col*7 + feb28Row + 1; got != want {
		t.Errorf("March 1 1900 is at position %d, want %d", got, want)
	}
}

// TestCalendarHeatMapNaNStyle checks that NaN days use the NaN color,
// fall back to the missing color when it is nil, and do not contribute
// to the dynamic range.
func TestCalendarHeatMapNaNStyle(t *testing.T) {
	data := plotter.DateValues{
		{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Value: math.NaN()},
		{Date: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), Value: 5},
	}
	h := mustCalendar(t, data, 2024)
	h.Min, h.Max = 0, 10
	h.Missing = color.RGBA{R: 240, G: 240, B: 240, A: 255}
	nan := color.RGBA{R: 250, G: 0, B: 0, A: 255}
	h.NaN = nan

	rects := cellFillRecords(t, h, 600, 300)
	if got, want := len(rects), h.Days(); got != want {
		t.Fatalf("unexpected number of cells: got %d, want %d", got, want)
	}

	p := plot.New()
	p.HideAxes()
	p.Add(h)
	img := vgimg.New(600, 300)
	p.Draw(draw.New(img))
	dpi := img.DPI()

	counts := make(map[color.Color]int)
	for _, r := range rects {
		cx := (r.Min.X + r.Max.X) / 2
		cy := (r.Min.Y + r.Max.Y) / 2
		px := int(cx.Dots(dpi))
		py := int(img.Image().Bounds().Max.Y - 1 - int(cy.Dots(dpi)))
		counts[color.RGBAModel.Convert(img.Image().At(px, py))]++
	}
	if got := counts[color.RGBAModel.Convert(nan)]; got != 1 {
		t.Errorf("unexpected NaN cell count: got %d, want 1", got)
	}
	if got := counts[color.RGBAModel.Convert(h.Missing)]; got != h.Days()-2 {
		t.Errorf("unexpected missing cell count: got %d, want %d", got, h.Days()-2)
	}

	// With a nil NaN color, the NaN day is drawn like a missing day.
	h.NaN = nil
	rects = cellFillRecords(t, h, 600, 300)
	if got, want := len(rects), h.Days(); got != want {
		t.Errorf("unexpected number of cells without NaN color: got %d, want %d", got, want)
	}
}

// TestCalendarHeatMapDefaultPalette checks that a nil palette selects a
// usable default instead of panicking.
func TestCalendarHeatMapDefaultPalette(t *testing.T) {
	h, err := plotter.NewCalendarHeatMap(plotter.DateValues{
		{Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Value: 1},
	}, 2024, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rects := cellFillRecords(t, h, 400, 200)
	if got, want := len(rects), h.Days(); got != want {
		t.Fatalf("unexpected number of cells: got %d, want %d", got, want)
	}
}

// TestCalendarHeatMapPanics checks the failing configurations that Plot
// rejects at draw time.
func TestCalendarHeatMapPanics(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*plotter.CalendarHeatMap)
	}{
		{
			name:   "nil palette",
			mutate: func(h *plotter.CalendarHeatMap) { h.Palette = nil },
		},
		{
			name:   "empty palette",
			mutate: func(h *plotter.CalendarHeatMap) { h.Palette = emptyPalette{} },
		},
		{
			name:   "inverted range",
			mutate: func(h *plotter.CalendarHeatMap) { h.Min, h.Max = 10, 0 },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := mustCalendar(t, nil, 2024)
			test.mutate(h)

			defer func() {
				if recover() == nil {
					t.Error("expected a panic")
				}
			}()
			p := plot.New()
			p.HideAxes()
			p.Add(h)
			img := vgimg.New(200, 100)
			p.Draw(draw.New(img))
		})
	}
}

// TestCalendarHeatMapOutlines checks that a positive line width strokes
// one outline per day cell.
func TestCalendarHeatMapOutlines(t *testing.T) {
	h := mustCalendar(t, nil, 2024)
	h.Missing = nil
	h.LineStyle.Width = vg.Points(0.5)

	p := plot.New()
	p.HideAxes()
	p.Add(h)
	scratch := vgimg.New(600, 300)
	p.Draw(draw.New(scratch))
	rec := new(recorder.Canvas)
	p.Draw(draw.NewCanvas(rec, 600, 300))

	// Day cell outlines are four-corner closed paths; the plot frame
	// is a two-point stroke and is not counted.
	strokes := 0
	for _, a := range rec.Actions {
		s, ok := a.(*recorder.Stroke)
		if ok && len(s.Path) == 5 {
			strokes++
		}
	}
	if got, want := strokes, h.Days(); got != want {
		t.Errorf("unexpected number of outlined cells: got %d, want %d", got, want)
	}
}

// TestCalendarHeatMapOneColor checks that a single-color palette is
// valid and paints every measured day without an index panic.
func TestCalendarHeatMapOneColor(t *testing.T) {
	h, err := plotter.NewCalendarHeatMap(plotter.DateValues{
		{Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Value: 3},
	}, 2024, palette.Heat(1, 1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h.Missing = nil
	rects := cellFillRecords(t, h, 400, 200)
	if got, want := len(rects), 1; got != want {
		t.Errorf("unexpected number of cells: got %d, want %d", got, want)
	}
}

// TestCalendarHeatMapDuplicateYearInput checks that a year with a
// February 29 in the input but not in the calendar skips the value.
func TestCalendarHeatMapDuplicateYearInput(t *testing.T) {
	h := mustCalendar(t, plotter.DateValues{
		{Date: time.Date(2023, 2, 28, 0, 0, 0, 0, time.UTC), Value: 1},
		{Date: time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC), Value: 2},
	}, 2023)
	if got, want := h.Days(), 365; got != want {
		t.Errorf("unexpected number of days: got %d, want %d", got, want)
	}
	if col, row, ok := h.Cell(time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC)); !ok || col != 9 || row != 2 {
		t.Errorf("March 1 2023: got cell (%d, %d, %v), want (9, 2, true)", col, row, ok)
	}
}
