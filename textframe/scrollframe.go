package textframe

import (
	"fmt"
	"image"
	"image/draw"
	"time"

	"github.com/aarzilli/yacco/util"
	"golang.org/x/mobile/event/mouse"
)

type ScrollFrame struct {
	b     draw.Image      // where the scrollbar and textframe will be displayed
	r     image.Rectangle // the rectangle that should be occupied by the scrollbar and textframe
	Flush func(...image.Rectangle)
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
	sfr.Flush = sfr.Flush
	if sfr.Width < 2 {
		return fmt.Errorf("ScrollFrame not wide enough")
	}
	sfr.Fr.debugRedraw = true
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

/*
Redraws frame.
rdir and rpar are optimization parameters for partial redrawing of the frame during scrolling
When rdir > 0, rpar is the number of glyphs that don't need to be redrawn
When rdir < 0, rpar is the first glyph that doesn't need to be redrawn
*/
func (sfr *ScrollFrame) Redraw(flush bool, predrawRects *[]image.Rectangle) {
	// scrollbar background
	bgr := sfr.r
	bgr.Max.X = bgr.Min.X + sfr.Width
	draw.Draw(sfr.b, sfr.r.Intersect(bgr), &sfr.Color, sfr.r.Intersect(bgr).Min, draw.Src)

	// position indicator
	posr := bgr
	posr.Max.X = posr.Max.X - 1
	posr.Min.Y = sfr.scale(sfr.Fr.Top) + sfr.r.Min.Y
	posz := sfr.scale(len(sfr.Fr.glyphs))
	if posz < 5 {
		posz = 5
	}
	posr.Max.Y = posz + posr.Min.Y
	draw.Draw(sfr.b, sfr.r.Intersect(posr), &sfr.Fr.Colors[0][0], sfr.r.Intersect(posr).Min, draw.Src)

	sfr.Fr.Redraw(false, predrawRects)

	if flush && (sfr.Flush != nil) {
		sfr.Flush(sfr.r)
	}

	if predrawRects != nil {
		*predrawRects = append(*predrawRects, bgr)
	}
}

func (sfr *ScrollFrame) scrollSetClick(event util.MouseDownEvent, events <-chan util.EventOrRunnable) {
	scrollr := sfr.r
	scrollr.Max.X = scrollr.Min.X + sfr.Width

	set := func(where image.Point) {
		p := int(float32(where.Y-sfr.r.Min.Y) / float32(sfr.r.Max.Y-sfr.r.Min.Y) * float32(sfr.bodyLen))
		sfr.Fr.Scroll(0, p)
		sfr.Redraw(true, nil)
	}

	set(event.Where)

	for ei := range events {
		switch e := ei.EventOrRun().(type) {
		case mouse.Event:
			switch e.Direction {
			case mouse.DirRelease:
				return
			default:
				set(image.Point{int(e.X), int(e.Y)})
			}
		}
	}
}

func (sfr *ScrollFrame) Under(p image.Point) bool {
	return p.In(sfr.r)
}

// If the click wasn't in the scrollbar area returns false
// Otherwise handles the click, until mouse-up is received, then returns true
func (sfr *ScrollFrame) ScrollClick(e util.MouseDownEvent, events <-chan util.EventOrRunnable) bool {
	scrollr := sfr.r
	scrollr.Max.X = scrollr.Min.X + sfr.Width

	if !e.Where.In(scrollr) {
		return false
	}

	if e.Which == mouse.ButtonMiddle {
		sfr.scrollSetClick(e, events)
		return true
	}

	where := e.Where
	which := e.Which
	autoscrollTicker := time.NewTicker(100 * time.Millisecond)
	inertia := 0

	scroll := func() {
		c := int(float32(where.Y-sfr.r.Min.Y) / float32(sfr.Fr.Font.Metrics().Height.Floor()))

		switch which {
		case mouse.ButtonLeft:
			sfr.Fr.Scroll(-1, c)
		case mouse.ButtonRight:
			sfr.Fr.Scroll(1, c)
		}
		//sfr.Scroll(true)
	}

	scroll()

loop:
	for {
		select {
		case ei := <-events:
			switch e := ei.EventOrRun().(type) {
			case mouse.Event:
				switch e.Direction {
				case mouse.DirRelease:
					break loop
				default:
					if e.Button != mouse.ButtonNone {
						where = image.Point{int(e.X), int(e.Y)}
					}
				}
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

func (sfr *ScrollFrame) OnClick(e util.MouseDownEvent, events <-chan util.EventOrRunnable) (bool, *mouse.Event) {
	if sfr.ScrollClick(e, events) {
		return false, nil
	}

	ee := sfr.Fr.OnClick(e, events)
	sfr.Redraw(true, nil)
	return true, ee
}
