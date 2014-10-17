package textframe

import (
	"fmt"
	"github.com/skelterjohn/go.wde"
	"image"
	"image/draw"
	"runtime"
	"time"
	"yacco/util"
)

type ScrollFrame struct {
	b     draw.Image      // where the scrollbar and textframe will be displayed
	r     image.Rectangle // the rectangle that should be occupied by the scrollbar and textframe
	Wnd   wde.Window
	Color image.Uniform // color for the scrollbar
	Width int           // horizontal width of the scrollbar
	Fr    Frame         // text frame

	bodyLen int
}

// Initializes the ScrollFrame, will set B, R and Wnd of the underlying Frame itself
func (sfr *ScrollFrame) Init(margin int) error {
	sfr.Fr.B = sfr.b
	sfr.Fr.R = sfr.r
	sfr.Fr.R.Min.X += sfr.Width
	sfr.Fr.Wnd = sfr.Wnd
	if sfr.Width < 2 {
		return fmt.Errorf("ScrollFrame not wide enough")
	}
	return sfr.Fr.Init(margin)
}

func (sfr *ScrollFrame) Set(top, bodyLen int) {
	sfr.Fr.Top = top
	sfr.bodyLen = bodyLen
}

func (sfr *ScrollFrame) scale(x int) int {
	scrollsz := (sfr.r.Max.Y - sfr.r.Min.Y)
	return int(float32(x) / float32(sfr.bodyLen) * float32(scrollsz))
}

func (sfr *ScrollFrame) SetRects(b draw.Image, r image.Rectangle) {
	sfr.b = b
	sfr.r = r
	sfr.Fr.B = sfr.b
	sfr.Fr.R = sfr.r
	sfr.Fr.R.Min.X += sfr.Width
}

func (sfr *ScrollFrame) Redraw(flush bool) {
	drawingFuncs := GetOptimizedDrawing(sfr.b)

	// scrollbar background
	bgr := sfr.r
	bgr.Max.X = bgr.Min.X + sfr.Width
	drawingFuncs.DrawFillSrc(sfr.b, sfr.r.Intersect(bgr), &sfr.Color)

	// position indicator
	posr := bgr
	posr.Max.X = posr.Max.X - 1
	posr.Min.Y = sfr.scale(sfr.Fr.Top) + sfr.r.Min.Y
	posz := sfr.scale(len(sfr.Fr.glyphs))
	if posz < 5 {
		posz = 5
	}
	posr.Max.Y = posz + posr.Min.Y
	drawingFuncs.DrawFillSrc(sfr.b, sfr.r.Intersect(posr), &sfr.Fr.Colors[0][0])

	sfr.Fr.Redraw(false)

	if flush && (sfr.Wnd != nil) {
		sfr.Wnd.FlushImage(sfr.r)
	}
}

func (sfr *ScrollFrame) scrollSetClick(event util.MouseDownEvent, events <-chan interface{}) {
	scrollr := sfr.r
	scrollr.Max.X = scrollr.Min.X + sfr.Width

	set := func(where image.Point) {
		p := int(float32(where.Y-sfr.r.Min.Y) / float32(sfr.r.Max.Y-sfr.r.Min.Y) * float32(sfr.bodyLen))
		sfr.Fr.Scroll(0, p)
		sfr.Redraw(true)
	}

	set(event.Where)

	for ei := range events {
		runtime.Gosched()
		switch e := ei.(type) {
		case wde.MouseUpEvent:
			return
		case wde.MouseDraggedEvent:
			set(e.Where)
		}
	}
}

func (sfr *ScrollFrame) Under(p image.Point) bool {
	return p.In(sfr.r)
}

// If the click wasn't in the scrollbar area returns false
// Otherwise handles the click, until mouse-up is received, then returns true
func (sfr *ScrollFrame) ScrollClick(e util.MouseDownEvent, events <-chan interface{}) bool {
	scrollr := sfr.r
	scrollr.Max.X = scrollr.Min.X + sfr.Width

	if !e.Where.In(scrollr) {
		return false
	}

	if e.Which == wde.MiddleButton {
		sfr.scrollSetClick(e, events)
		return true
	}

	where := e.Where
	which := e.Which
	autoscrollTicker := time.NewTicker(100 * time.Millisecond)
	inertia := 0

	scroll := func() {
		c := int(float32(where.Y-sfr.r.Min.Y) / float32(sfr.Fr.Font.LineHeightRaster()>>8))

		switch which {
		case wde.LeftButton:
			sfr.Fr.Scroll(-1, c)
		case wde.RightButton:
			sfr.Fr.Scroll(1, c)
		}
		sfr.Redraw(true)
	}

	scroll()

loop:
	for {
		runtime.Gosched()
		select {
		case ei := <-events:
			switch e := ei.(type) {
			case wde.MouseUpEvent:
				break loop
			case wde.MouseDraggedEvent:
				where = e.Where
			}

		case <-autoscrollTicker.C:
			if inertia > 5 {
				scroll()
			} else {
				inertia++
			}
		}
	}

	return true
}

func (sfr *ScrollFrame) OnClick(e util.MouseDownEvent, events <-chan interface{}) (bool, *wde.MouseUpEvent) {
	if sfr.ScrollClick(e, events) {
		return false, nil
	}

	ee := sfr.Fr.OnClick(e, events)
	sfr.Redraw(true)
	return true, ee
}
