package main

import (
	"image"
	"image/draw"
	"yacco/config"
)

type Cols struct {
	cols []*Col
	wnd  *Window
	b    draw.Image
	r    image.Rectangle
}

const COL_MIN_WIDTH = 40

func NewCols(wnd *Window, r image.Rectangle) *Cols {
	return &Cols{[]*Col{}, wnd, wnd.img, r}
}

func (cs *Cols) SetRects(wnd *Window, b draw.Image, r image.Rectangle, reset bool) {
	cs.wnd = wnd
	cs.r = r
	cs.b = b
	cs.RecalcRects(reset)
}

// Create a new column after the specified one, specify -1 to create a new column at the end
func (cs *Cols) AddAfter(c *Col, n int, f float64) *Col {
	//c := NewCol(cs.wnd, cs.r)
	if len(cs.cols) == 0 {
		cs.cols = append(cs.cols, c)
	} else {
		if n < 0 {
			n = len(cs.cols) - 1
		}

		c.frac = cs.cols[n].frac * f
		cs.cols[n].frac -= c.frac

		cs.cols = append(cs.cols, nil)
		copy(cs.cols[n+2:], cs.cols[n+1:])
		cs.cols[n+1] = c
	}

	cs.RecalcRects(false)
	cs.Redraw()

	return c
}

func (cs *Cols) RecalcRects(reset bool) {
	w := cs.r.Max.X - cs.r.Min.X

	minimizedw := 0
	lastNonminimized := -1
	totalFrac := 0.0001

	for i := range cs.cols {
		ew := int(cs.cols[i].frac / 10 * float64(w))
		if ew < COL_MIN_WIDTH {
			minimizedw += COL_MIN_WIDTH
		} else {
			lastNonminimized = i
			totalFrac += cs.cols[i].frac
		}
	}

	x := cs.r.Min.X
	w -= minimizedw
	remw := w

	for i := range cs.cols {
		var curw int
		ew := int(cs.cols[i].frac / totalFrac * float64(w))
		if ew < COL_MIN_WIDTH {
			curw = COL_MIN_WIDTH
		} else if i == lastNonminimized {
			curw = remw
			remw = 0
		} else {
			curw = ew
			remw -= curw
		}

		r := cs.r
		r.Min.X = x
		r.Max.X = x + curw
		x += curw
		cs.cols[i].SetRects(cs.wnd, cs.b, cs.r.Intersect(r), i == (len(cs.cols)-1), reset)
	}
}

func (cs *Cols) Redraw() {
	if len(cs.cols) <= 0 {
		draw.Draw(cs.b, cs.r, &config.TheColorScheme.WindowBG, cs.r.Min, draw.Src)
	}

	for _, c := range cs.cols {
		c.Redraw()
	}
}

func (cs *Cols) IndexOf(c *Col) int {
	for i := range cs.cols {
		if cs.cols[i] == c {
			return i
		}
	}

	return -1
}

func (cs *Cols) Remove(i int) {
	if i > 0 {
		cs.cols[i-1].frac += cs.cols[i].frac
	} else { // i == 0
		if len(cs.cols) > 0 {
			cs.cols[1].frac += cs.cols[0].frac
		}
	}
	copy(cs.cols[i:], cs.cols[i+1:])
	cs.cols = cs.cols[:len(cs.cols)-1]
	cs.RecalcRects(false)
	cs.Redraw()
}
