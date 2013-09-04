package textframe

import (
	"code.google.com/p/freetype-go/freetype"
	"code.google.com/p/freetype-go/freetype/raster"
	"code.google.com/p/freetype-go/freetype/truetype"
	"fmt"
	"github.com/skelterjohn/go.wde"
	"image"
	"image/draw"
	"math"
	"runtime"
	"time"
	"unicode"
	"yacco/util"
)

type Redrawable interface {
	Redraw(flush bool)
}

type ColorRune struct {
	C uint8
	R rune
}

// Callback when the frame needs to scroll its text
// If scrollDir is 0 then n is the absolute position to move to
// If scrollDir is -1 the text should be scrolled back by n lines
// If scrollDir is +1 the text should be scrolled forward by n lines
type FrameScrollFn func(scrollDir int, n int)

const (
	HF_BOLSPACES uint32 = 1 << iota
	HF_MARKSOFTWRAP
	HF_QUOTEHACK
	HF_TRUNCATE // truncates instead of softwrapping
	HF_ELASTICTABS
	HF_COLUMNIZE
)

type Frame struct {
	Font        *util.Font
	Hackflags   uint32
	B           draw.Image      // the image the text will be drawn upon
	R           image.Rectangle // the rectangle occupied by the frame
	VisibleTick bool
	Colors      [][]image.Uniform
	TabWidth    int
	Wnd         wde.Window
	Scroll      FrameScrollFn
	Top         int

	Limit image.Point

	margin raster.Fix32

	Sels []util.Sel

	cs       []*freetype.Context
	glyphs   []glyph
	ins      raster.Point
	lastFull int
}

/*
ABOUT SELECTIONS AND COLORS
A frame can have any number (greater than 1) of selections.
Every selection is defined by a start point and an end point. Empty selections have start == end.
The first selection is mandatory and if it is empty a tick (a vertical bar) is displayed at its start (and end) point, to disable the tick set visible to false.

The color matrix must have as many rows as there are selections (empty or otherwise) in the frame plus one. In each row there must be at least two colors: the color at index 0 is the background color, the color at index 1 is the default foreground color. All other colors are foreground colors used as specified when using InsertColor.

The very first row of the color matrix are the colors used for unselected text.
*/

type glyph struct {
	r         rune
	fontIndex int
	index     truetype.Index
	width     raster.Fix32
	kerning   raster.Fix32
	widthy    raster.Fix32
	p         raster.Point
	color     uint8
}

// Initializes frame
func (fr *Frame) Init(margin int) error {
	fr.margin = raster.Fix32(margin << 8)
	fr.Sels = make([]util.Sel, len(fr.Colors))
	fr.glyphs = []glyph{}

	if fr.TabWidth == 0 {
		fr.TabWidth = 8
	}

	// sanity checks

	if len(fr.Colors) < 2 {
		return fmt.Errorf("Not enough color lines")
	}

	for i, _ := range fr.Colors {
		if len(fr.Colors[i]) < 2 {
			return fmt.Errorf("Not enough colors in line %d", i)
		}
	}

	// create font contexts

	fr.cs = fr.Font.CreateContexts(fr.R)

	fr.Clear()

	return nil
}

func (fr *Frame) ReinitFont() {
	fr.cs = fr.Font.CreateContexts(fr.R)
}

func (fr *Frame) lineHeight() raster.Fix32 {
	lh := fr.Font.LineHeight()
	return fr.cs[0].PointToFix32(float64(lh) * fr.Font.Spacing)
}

func (fr *Frame) initialInsPoint() raster.Point {
	gb := fr.Font.Bounds()
	return raster.Point{raster.Fix32(fr.R.Min.X<<8) + fr.margin, raster.Fix32(fr.R.Min.Y<<8) + raster.Fix32(int32(float64(gb.YMax)*fr.Font.Spacing)<<8)}
}

func (fr *Frame) Clear() {
	/*println("Frame", fr, "cleared")
	_, file, line, _ := runtime.Caller(1)
	println("\t", file, ":", line, "")
	_, file, line, _ = runtime.Caller(2)
	println("\t", file, ":", line, "\n")*/

	fr.ins = fr.initialInsPoint()
	fr.glyphs = fr.glyphs[:0]
	fr.lastFull = 0
}

// Inserts text into the frame, returns the maximum X and Y used
func (fr *Frame) Insert(runes []rune) (limit image.Point) {
	cr := make([]ColorRune, len(runes))
	for i, _ := range runes {
		cr[i].C = 1
		cr[i].R = runes[i]
	}
	return fr.InsertColor(cr)
}

// Inserts text into the frame, returns the maximum X and Y used
func (fr *Frame) InsertColor(runes []ColorRune) (limit image.Point) {
	lh := fr.lineHeight()

	_, spaceIndex := fr.getIndex(' ')

	prev, prevFontIdx, hasPrev := truetype.Index(0), 0, false

	rightMargin := raster.Fix32(fr.R.Max.X<<8) - fr.margin
	leftMargin := raster.Fix32(fr.R.Min.X<<8) + fr.margin
	bottom := raster.Fix32(fr.R.Max.Y<<8) + lh

	_, xIndex := fr.getIndex('x')
	spaceWidth := raster.Fix32(fr.Font.Fonts[0].HMetric(fr.cs[0].Scale, spaceIndex).AdvanceWidth) << 2
	bigSpaceWidth := raster.Fix32(fr.Font.Fonts[0].HMetric(fr.cs[0].Scale, xIndex).AdvanceWidth) << 2
	tabWidth := spaceWidth * raster.Fix32(fr.TabWidth)

	if fr.Hackflags&HF_ELASTICTABS != 0 {
		defer func() {
			limit = fr.elasticTabs(bigSpaceWidth, tabWidth, bottom, leftMargin, rightMargin, lh)
		}()
	}

	limit.X = int(fr.ins.X >> 8)
	limit.Y = int(fr.ins.Y >> 8)

	for i, crune := range runes {
		if fr.ins.Y > bottom {
			fr.Limit = limit
			return
		}

		if fr.ins.Y < raster.Fix32(fr.R.Max.Y<<8) {
			//println("lastFull is: ", len(fr.glyphs), fr.ins.Y + lh, raster.Fix32(fr.R.Max.Y << 8))
			fr.lastFull = len(fr.glyphs)
		}

		if crune.R == '\n' {
			g := glyph{
				r:         crune.R,
				fontIndex: 0,
				index:     spaceIndex,
				p:         fr.ins,
				color:     crune.C & 0x0f,
				width:     raster.Fix32(fr.R.Max.X<<8) - fr.ins.X - fr.margin,
				widthy:    lh}

			fr.glyphs = append(fr.glyphs, g)

			fr.ins.X = leftMargin
			fr.ins.Y += lh
			prev, prevFontIdx, hasPrev = spaceIndex, 0, true
		} else if crune.R == '\t' {
			toNextCell := tabWidth - ((fr.ins.X - leftMargin) % tabWidth)
			if toNextCell <= spaceWidth/2 {
				toNextCell += tabWidth
			}

			g := glyph{
				r:         crune.R,
				fontIndex: 0,
				index:     spaceIndex,
				p:         fr.ins,
				color:     crune.C & 0x0f,
				width:     toNextCell}

			fr.glyphs = append(fr.glyphs, g)

			fr.ins.X += toNextCell
			prev, prevFontIdx, hasPrev = spaceIndex, 0, true
		} else if (crune.R == ' ') && (fr.Hackflags&HF_BOLSPACES != 0) {
			width := raster.Fix32(0)
			if i == 0 {
				if len(fr.glyphs) == 0 {
					width = bigSpaceWidth
				} else {
					width = spaceWidth
				}
			} else {
				switch fr.glyphs[len(fr.glyphs)-1].r {
				case '\t':
					fallthrough
				case '\n':
					width = bigSpaceWidth
				case ' ':
					width = fr.glyphs[len(fr.glyphs)-1].width
				default:
					width = spaceWidth
				}
			}

			g := glyph{
				r:         crune.R,
				fontIndex: 0,
				index:     spaceIndex,
				p:         fr.ins,
				color:     crune.C & 0x0f,
				width:     width}

			fr.glyphs = append(fr.glyphs, g)
			fr.ins.X += width
			prev, prevFontIdx, hasPrev = spaceIndex, 0, true
		} else {
			lur := crune.R

			if (fr.Hackflags & HF_QUOTEHACK) != 0 {
				if lur == '`' {
					lur = 0x2018
				} else if lur == '\'' {
					lur = 0x2019
				}
			}

			fontIdx, index := fr.getIndex(lur)
			width := raster.Fix32(fr.Font.Fonts[fontIdx].HMetric(fr.cs[fontIdx].Scale, index).AdvanceWidth) << 2
			kerning := raster.Fix32(0)
			if hasPrev && (fontIdx == prevFontIdx) {
				kerning = raster.Fix32(fr.Font.Fonts[fontIdx].Kerning(fr.cs[fontIdx].Scale, prev, index)) << 2
				fr.ins.X += kerning
			}

			if fr.Hackflags&HF_TRUNCATE == 0 {
				if fr.ins.X+width > rightMargin {
					fr.ins.X = leftMargin
					fr.ins.Y += lh
				}
			}

			g := glyph{
				r:         crune.R,
				fontIndex: fontIdx,
				index:     index,
				p:         fr.ins,
				color:     crune.C & 0x0f,
				kerning:   kerning,
				width:     width}

			fr.glyphs = append(fr.glyphs, g)

			fr.ins.X += width
			prev, prevFontIdx, hasPrev = index, fontIdx, true
		}

		if int(fr.ins.X>>8) > limit.X {
			limit.X = int(fr.ins.X >> 8)
		}

		if int(fr.ins.Y>>8) > limit.Y {
			limit.Y = int(fr.ins.Y >> 8)
		}
	}
	if fr.ins.Y < raster.Fix32(fr.R.Max.Y<<8) {
		//println("lastFull is: ", len(fr.glyphs), fr.ins.Y + lh, raster.Fix32(fr.R.Max.Y << 8))
		fr.lastFull = len(fr.glyphs)
	}
	fr.Limit = limit
	return
}

// Tracks the mouse position, selecting text, the events channel is from go.wde
// kind is 1 for character by character selection, 2 for word by word selection, 3 for line by line selection
func (fr *Frame) Select(idx, kind int, button wde.Button, events <-chan interface{}) *wde.MouseUpEvent {
	if (idx < 0) || (idx >= len(fr.Sels)) {
		for ei := range events {
			switch e := ei.(type) {
			case wde.MouseUpEvent:
				return &e
			}
		}
	}

	fix := fr.Sels[idx].S
	var autoscrollTicker *time.Ticker
	var autoscrollChan <-chan time.Time

	stopAutoscroll := func() {
		if autoscrollTicker != nil {
			autoscrollTicker.Stop()
			autoscrollTicker = nil
		}
	}

	var lastPos image.Point

	for {
		runtime.Gosched()
		select {
		case ei := <-events:
			switch e := ei.(type) {
			case wde.MouseDraggedEvent:
				lastPos = e.Where
				if e.Where.In(fr.R) {
					stopAutoscroll()

					p := fr.CoordToPoint(e.Where)
					fr.SetSelect(idx, kind, fix, p)
					fr.Redraw(true)
				} else {
					if autoscrollTicker == nil {
						autoscrollTicker = time.NewTicker(100 * time.Millisecond)
						autoscrollChan = autoscrollTicker.C
					}
				}

			/*case wde.MouseEnteredEvent:
			stopAutoscroll()
			return nil*/

			case wde.MouseUpEvent:
				stopAutoscroll()
				return &e
			}

		case <-autoscrollChan:
			sd := fr.scrollDir(lastPos)
			if sd != 0 {
				fr.Scroll(sd, 1)
				if sd < 0 {
					fr.SetSelect(idx, kind, fr.Top, fix)
				} else if sd > 0 {
					fr.SetSelect(idx, kind, len(fr.glyphs)+fr.Top, fix)
				}
				fr.Redraw(true)
			}
		}
	}
}

// Sets extremes of the selection, pass start == end if you want an empty selection
// idx is the index of the selection
func (fr *Frame) SetSelect(idx, kind, start, end int) {
	fr.setSelectEx(idx, kind, start, end, true)
}

func (fr *Frame) DisableOtherSelections(idx int) {
	for i := range fr.Sels {
		if i > 3 {
			break
		}
		if i != idx {
			fr.Sels[i].E = fr.Sels[i].S
		}
	}
}

func (fr *Frame) setSelectEx(idx, kind, start, end int, disableOther bool) {
	if disableOther || (kind != 1) {
		for i := range fr.Sels {
			if (i != idx) && (i != 0) {
				fr.Sels[i].E = fr.Sels[i].S
			}
		}
	}

	if (idx < 0) || (idx >= len(fr.Sels)) {
		return
	}

	if start >= end {
		temp := start
		start = end
		end = temp
	}

	switch kind {
	case 1:
		// do nothing

	case 2:
		var s, e int
		s = start - fr.Top
		if (s <= len(fr.glyphs)) && (s >= 0) {
			first := true
			if s == len(fr.glyphs) {
				s = s - 1
			}
			for ; s >= 0; s-- {
				g := fr.glyphs[s].r
				if !(unicode.IsLetter(g) || unicode.IsDigit(g) || (g == '_')) {
					if !first {
						s++
					}
					break
				}
				first = false
			}
			if s < 0 {
				s = 0
			}
		}

		e = end - fr.Top
		if (e < len(fr.glyphs)) && (e >= 0) {
			first := true
			for ; e < len(fr.glyphs); e++ {
				g := fr.glyphs[e].r
				if g == '\n' { // don't select end of line newline
					break
				}
				if !(unicode.IsLetter(g) || unicode.IsDigit(g) || (g == '_')) {
					if first {
						e++
					}
					break
				}
				first = false
			}
		}

		start = s + fr.Top
		end = e + fr.Top

	case 3:
		var s, e int
		s = start - fr.Top
		if (s <= len(fr.glyphs)) && (s >= 0) {
			s = s - 1
			for ; s > 0; s-- {
				if fr.glyphs[s].r == '\n' {
					s++
					break
				}
			}

			if s < 0 {
				s = 0
			}
		}

		e = end - fr.Top
		if (e < len(fr.glyphs)) && (e >= 0) {
			for ; e < len(fr.glyphs); e++ {
				if fr.glyphs[e].r == '\n' {
					e++
					break
				}
			}
		}
		start = s + fr.Top
		end = e + fr.Top
	}

	fr.Sels[idx].S = start
	fr.Sels[idx].E = end
}

// Converts a graphical coordinate to a rune index
func (fr *Frame) CoordToPoint(coord image.Point) int {
	if !coord.In(fr.R) {
		return -1
	}

	ftcoord := freetype.Pt(coord.X, coord.Y)
	lh := fr.lineHeight()
	glyphBounds := fr.Font.Fonts[0].Bounds(int32(fr.Font.Size))

	for i, g := range fr.glyphs {
		if g.p.Y-raster.Fix32(glyphBounds.YMin<<8) < ftcoord.Y {
			continue
		} else if (g.p.Y - lh) > ftcoord.Y {
			return i + fr.Top
		} else if ftcoord.X < g.p.X {
			return i + fr.Top
		} else if g.r == '\n' {
			return i + fr.Top
		} else if (ftcoord.X >= g.p.X) && (ftcoord.X <= g.p.X+g.width) {
			return i + fr.Top
		}
	}

	return fr.Top + len(fr.glyphs)
}

// Converts rune index into a graphical coordinate
func (fr *Frame) PointToCoord(p int) image.Point {
	pp := p - fr.Top
	if pp < 0 {
		var r raster.Point
		if len(fr.glyphs) == 0 {
			r = fr.ins
		} else {
			r = fr.glyphs[0].p
		}
		return image.Point{int(r.X >> 8), int(r.Y >> 8)}
	} else if pp < len(fr.glyphs) {
		r := fr.glyphs[pp].p
		return image.Point{int(r.X >> 8), int(r.Y >> 8)}
	} else if (pp == len(fr.glyphs)) && (len(fr.glyphs) > 0) {
		r := fr.glyphs[pp-1].p
		return image.Point{int((r.X + fr.glyphs[pp-1].width) >> 8), int(r.Y >> 8)}
	} else {
		return image.Point{fr.R.Min.X + 2, fr.R.Min.Y + 2}
	}
}

func (fr *Frame) redrawSelection(s, e int, color *image.Uniform) {
	if s < 0 {
		s = 0
	}
	glyphBounds := fr.Font.Fonts[0].Bounds(int32(fr.Font.Size))
	rightMargin := raster.Fix32(fr.R.Max.X<<8) - fr.margin
	leftMargin := raster.Fix32(fr.R.Min.X<<8) + fr.margin
	drawingFuncs := GetOptimizedDrawing(fr.B)

	var sp, ep, sep image.Point

	ss := fr.glyphs[s]
	sp = image.Point{int(ss.p.X >> 8), int((ss.p.Y)>>8) - int(glyphBounds.YMax)}

	var se glyph

	if e < len(fr.glyphs) {
		se = fr.glyphs[e]
		sep = image.Point{int(leftMargin >> 8), int((se.p.Y)>>8) - int(glyphBounds.YMax)}
		ep = image.Point{int(se.p.X >> 8), int((se.p.Y)>>8) - int(glyphBounds.YMin)}
	} else if e == len(fr.glyphs) {
		se = fr.glyphs[len(fr.glyphs)-1]
		sep = image.Point{int(leftMargin >> 8), int((se.p.Y)>>8) - int(glyphBounds.YMax)}
		ep = image.Point{int((se.p.X + se.width) >> 8), int((se.p.Y)>>8) - int(glyphBounds.YMin)}
	} else {
		se = fr.glyphs[len(fr.glyphs)-1]
		sep = image.Point{int(leftMargin >> 8), int((se.p.Y)>>8) - int(glyphBounds.YMax)}
		ep = image.Point{int(rightMargin >> 8), fr.R.Max.Y}
	}

	if ss.p.Y == se.p.Y {
		r := image.Rectangle{sp, ep}
		drawingFuncs.DrawFillSrc(fr.B, fr.R.Intersect(r), color)
	} else {
		rs := image.Rectangle{sp, image.Point{int(rightMargin >> 8), int((ss.p.Y)>>8) - int(glyphBounds.YMin)}}
		re := image.Rectangle{sep, ep}
		rb := image.Rectangle{
			image.Point{sep.X, int((ss.p.Y)>>8) - int(glyphBounds.YMin)},
			image.Point{int(rightMargin >> 8), sep.Y},
		}
		drawingFuncs.DrawFillSrc(fr.B, fr.R.Intersect(rs), color)
		drawingFuncs.DrawFillSrc(fr.B, fr.R.Intersect(re), color)
		drawingFuncs.DrawFillSrc(fr.B, fr.R.Intersect(rb), color)
	}
}

func (fr *Frame) Redraw(flush bool) {
	glyphBounds := fr.Font.Fonts[0].Bounds(int32(fr.Font.Size))
	rightMargin := raster.Fix32(fr.R.Max.X<<8) - fr.margin
	leftMargin := raster.Fix32(fr.R.Min.X<<8) + fr.margin

	drawingFuncs := GetOptimizedDrawing(fr.B)

	// background
	drawingFuncs.DrawFillSrc(fr.B, fr.R, &fr.Colors[0][0])

	ssel := 0
	var cury raster.Fix32 = 0
	if len(fr.glyphs) > 0 {
		cury = fr.glyphs[0].p.Y
	}
	newline := true

	for i, g := range fr.glyphs {
		// Selection drawing
		if ssel != 0 {
			if i+fr.Top >= fr.Sels[ssel-1].E {
				ssel = 0
			}
		} else {
			for j := range fr.Sels {
				if (j+1 >= len(fr.Colors)) {
					break
				}
				if /*((fr.Colors[j+1][0] != fr.Colors[0][0]) || (fr.Colors[j+1][1] != fr.Colors[0][1])) && */(i+fr.Top >= fr.Sels[j].S) && (i+fr.Top < fr.Sels[j].E) {
					ssel = j + 1
					fr.redrawSelection(fr.Sels[j].S-fr.Top, fr.Sels[j].E-fr.Top, &fr.Colors[ssel][0])
				}
			}
		}

		// Softwrap mark drawing
		if (g.p.Y != cury) && ((fr.Hackflags & HF_MARKSOFTWRAP) != 0) {
			midline := int(cury>>8) - int((glyphBounds.YMax+glyphBounds.YMin)/2)
			if !newline {
				r := image.Rectangle{
					image.Point{int(rightMargin >> 8), midline},
					image.Point{int(rightMargin>>8) + int(fr.margin>>8), midline + 1}}
				drawingFuncs.DrawFillSrc(fr.B, fr.R.Intersect(r), &fr.Colors[0][1])
			}

			cury = g.p.Y

			midline = int(cury>>8) - int((glyphBounds.YMax+glyphBounds.YMin)/2)

			if !newline {
				r := image.Rectangle{
					image.Point{int(leftMargin>>8) - int(fr.margin>>8), midline},
					image.Point{int(leftMargin >> 8), midline + 1}}
				drawingFuncs.DrawFillSrc(fr.B, fr.R.Intersect(r), &fr.Colors[0][1])
			}
		}
		newline = (g.r == '\n')

		// Glyph drawing
		var color *image.Uniform
		if ssel >= len(fr.Colors) {
			color = &fr.Colors[0][g.color]
		} else {
			color = &fr.Colors[ssel][g.color]
		}
		fr.cs[g.fontIndex].SetSrc(color)
		mask, offset, err := fr.cs[g.fontIndex].Glyph(g.index, g.p)
		if err != nil {
			panic(err)
		}
		glyphRect := mask.Bounds().Add(offset)
		dr := fr.R.Intersect(glyphRect)
		if !dr.Empty() {
			mp := image.Point{0, dr.Min.Y - glyphRect.Min.Y}
			color := &fr.Colors[1][1]
			if (ssel >= 0) && (ssel < len(fr.Colors)) && (g.color >= 0) && (int(g.color) < len(fr.Colors[ssel])) {
				color = &fr.Colors[ssel][g.color]
			}
			drawingFuncs.DrawGlyphOver(fr.B, dr, color, mask, mp)
		}
	}

	// Tick drawing
	if (fr.Sels[0].S == fr.Sels[0].E) && fr.VisibleTick && (len(fr.glyphs) > 0) && (fr.Sels[0].S-fr.Top >= 0) && (fr.Sels[0].S-fr.Top <= len(fr.glyphs)) {
		var x, y int
		if fr.Sels[0].S-fr.Top < len(fr.glyphs) {
			g := fr.glyphs[fr.Sels[0].S-fr.Top]
			x = int(g.p.X >> 8)
			y = int(g.p.Y >> 8)
		} else {
			g := fr.glyphs[len(fr.glyphs)-1]

			if g.widthy > 0 {
				x = fr.R.Min.X + int(fr.margin>>8)
				y = int((g.p.Y + g.widthy) >> 8)
			} else {
				x = int((g.p.X+g.width)>>8) + 1
				y = int(g.p.Y >> 8)
			}

		}
		r := image.Rectangle{
			Min: image.Point{x, y - int(glyphBounds.YMax)},
			Max: image.Point{x + 1, y - int(glyphBounds.YMin)}}

		drawingFuncs.DrawFillSrc(fr.B, fr.R.Intersect(r), &fr.Colors[0][1])

		r1 := r
		r1.Min.X -= 1
		r1.Max.X += 1
		r1.Max.Y = r1.Min.Y + 3
		drawingFuncs.DrawFillSrc(fr.B, fr.R.Intersect(r1), &fr.Colors[0][1])

		r2 := r
		r2.Min.X -= 1
		r2.Max.X += 1
		r2.Min.Y = r2.Max.Y - 3
		drawingFuncs.DrawFillSrc(fr.B, fr.R.Intersect(r2), &fr.Colors[0][1])
	}

	if flush && (fr.Wnd != nil) {
		fr.Wnd.FlushImage(fr.R)
	}
}

func (fr *Frame) getIndex(x rune) (fontIdx int, index truetype.Index) {
	var font *truetype.Font
	for fontIdx, font = range fr.Font.Fonts {
		index = font.Index(x)
		if index != 0 {
			return
		}
	}
	fontIdx = 0
	index = 0
	return
}

func (fr *Frame) scrollDir(recalcPos image.Point) int {
	if (recalcPos.X < fr.R.Min.X) || (recalcPos.X > fr.R.Max.X) {
		return 0
	}

	if recalcPos.Y < fr.R.Min.Y {
		return -1
	}

	if recalcPos.Y > fr.R.Max.Y {
		return +1
	}

	return 0
}

func (f *Frame) OnClick(e util.MouseDownEvent, events <-chan interface{}) *wde.MouseUpEvent {
	if e.Which == wde.WheelUpButton {
		f.Scroll(-1, 1)
		f.Redraw(true)
		return nil
	}

	if e.Which == wde.WheelDownButton {
		f.Scroll(+1, 1)
		f.Redraw(true)
		return nil
	}

	p := f.CoordToPoint(e.Where)

	sel := int(math.Log2(float64(e.Which)))
	if sel >= len(f.Sels) {
		return nil
	}

	if p >= 0 {
		if (sel == 0) && (e.Count == 1) && (e.Modifiers == "shift+") {
			// shift-click extends selection, but only for the first selection
			if p < f.Sels[0].S {
				f.setSelectEx(sel, e.Count, p, f.Sels[0].E, false)
			} else {
				f.setSelectEx(sel, e.Count, f.Sels[0].S, p, false)
			}
		} else {
			f.setSelectEx(sel, e.Count, p, p, false)
		}
		if e.Count > 1 {
			f.Redraw(true)
		}
		ee := f.Select(sel, e.Count, e.Which, events)
		f.Redraw(true)
		return ee
	}

	return nil
}

func (fr *Frame) LineNo() int {
	return int(float32(fr.R.Max.Y-fr.R.Min.Y) / float32(fr.lineHeight()>>8))
}

func (fr *Frame) Inside(p int) bool {
	rp := p - fr.Top
	//println("Inside", p, rp, fr.lastFull)
	if rp < 0 {
		return false
	}
	if rp > fr.lastFull {
		return false
	}
	return true
}

// Returns a slice of addresses of the starting characters of each phisical line
// Phisical lines are distinct lines on the screen, ie a softwrap generates a new phisical line
func (fr *Frame) phisicalLines() []int {
	r := []int{}
	y := raster.Fix32(0)
	for i := range fr.glyphs {
		if fr.glyphs[i].p.Y != y {
			r = append(r, i)
			y = fr.glyphs[i].p.Y
		}
	}
	return r
}

// Pushes text graphically up by "ln" phisical lines
// Returns the number of glyphs left in the frame
func (fr *Frame) PushUp(ln int) (newsize int) {
	fr.ins = fr.initialInsPoint()

	lh := fr.lineHeight()
	_, spaceIndex := fr.getIndex(' ')

	rightMargin := raster.Fix32(fr.R.Max.X<<8) - fr.margin
	leftMargin := raster.Fix32(fr.R.Min.X<<8) + fr.margin
	bottom := raster.Fix32(fr.R.Max.Y<<8) + lh

	_, xIndex := fr.getIndex('x')
	spaceWidth := raster.Fix32(fr.Font.Fonts[0].HMetric(fr.cs[0].Scale, spaceIndex).AdvanceWidth) << 2
	bigSpaceWidth := raster.Fix32(fr.Font.Fonts[0].HMetric(fr.cs[0].Scale, xIndex).AdvanceWidth) << 2
	tabWidth := spaceWidth * raster.Fix32(fr.TabWidth)

	fr.Limit.X = int(fr.ins.X >> 8)
	fr.Limit.Y = int(fr.ins.Y >> 8)

	if fr.Hackflags&HF_ELASTICTABS != 0 {
		defer func() {
			fr.Limit = fr.elasticTabs(bigSpaceWidth, tabWidth, bottom, leftMargin, rightMargin, lh)
		}()
	}

	off := -1
	for i := range fr.glyphs {
		fr.glyphs[i].p.Y -= raster.Fix32(ln) * lh
		if (off < 0) && (fr.glyphs[i].p.Y >= fr.ins.Y) {
			off = i
		}

		if int(fr.glyphs[i].p.Y>>8) > fr.Limit.Y {
			fr.Limit.Y = int(fr.glyphs[i].p.Y >> 8)
		}
		if int(fr.glyphs[i].p.X>>8) > fr.Limit.X {
			fr.Limit.X = int(fr.glyphs[i].p.X >> 8)
		}
	}

	if off >= 0 {
		fr.Top += off
		copy(fr.glyphs, fr.glyphs[off:])
		fr.glyphs = fr.glyphs[:len(fr.glyphs)-off]
	} else {
		fr.glyphs = []glyph{}
	}

	if len(fr.glyphs) > 0 {
		g := &fr.glyphs[len(fr.glyphs)-1]
		fr.ins.X = g.p.X
		fr.ins.Y = g.p.Y
	}

	fr.lastFull = len(fr.glyphs)
	return len(fr.glyphs)
}

func (fr *Frame) PushDown(ln int, a, b []ColorRune) (limit image.Point) {
	oldglyphs := make([]glyph, len(fr.glyphs))
	copy(oldglyphs, fr.glyphs)

	fr.Top -= len(a) + len(b)
	fr.Clear()
	fr.InsertColor(a)
	fr.InsertColor(b)

	limit = fr.Limit

	pl := fr.phisicalLines()

	if len(pl) > ln {
		fr.PushUp(len(pl) - ln)
	}

	lh := fr.lineHeight()

	_, spaceIndex := fr.getIndex(' ')

	rightMargin := raster.Fix32(fr.R.Max.X<<8) - fr.margin
	leftMargin := raster.Fix32(fr.R.Min.X<<8) + fr.margin
	bottom := raster.Fix32(fr.R.Max.Y<<8) + lh

	_, xIndex := fr.getIndex('x')
	spaceWidth := raster.Fix32(fr.Font.Fonts[0].HMetric(fr.cs[0].Scale, spaceIndex).AdvanceWidth) << 2
	bigSpaceWidth := raster.Fix32(fr.Font.Fonts[0].HMetric(fr.cs[0].Scale, xIndex).AdvanceWidth) << 2
	tabWidth := spaceWidth * raster.Fix32(fr.TabWidth)

	if fr.ins.X != leftMargin {
		fr.ins.X = leftMargin
		fr.ins.Y += lh
	}

	if fr.Hackflags&HF_ELASTICTABS != 0 {
		defer func() {
			fr.Limit = fr.elasticTabs(bigSpaceWidth, tabWidth, bottom, leftMargin, rightMargin, lh)
		}()
	}

	for i := range oldglyphs {
		if fr.ins.Y > bottom {
			fr.Limit = limit
			return
		}

		if fr.ins.Y < raster.Fix32(fr.R.Max.Y<<8) {
			//println("lastFull is: ", len(fr.glyphs), fr.ins.Y + lh, raster.Fix32(fr.R.Max.Y << 8))
			fr.lastFull = len(fr.glyphs)
		}

		if oldglyphs[i].r == '\n' {
			fr.ins.Y += lh
		} else {
			oldglyphs[i].p.Y = fr.ins.Y
			fr.ins.X = oldglyphs[i].p.X
		}

		fr.glyphs = append(fr.glyphs, oldglyphs[i])

		if int(fr.glyphs[i].p.Y>>8) > fr.Limit.Y {
			fr.Limit.Y = int(fr.glyphs[i].p.Y >> 8)
		}
		if int(fr.glyphs[i].p.X>>8) > fr.Limit.X {
			fr.Limit.X = int(fr.glyphs[i].p.X >> 8)
		}
	}

	if fr.ins.Y < raster.Fix32(fr.R.Max.Y<<8) {
		//println("lastFull is: ", len(fr.glyphs), fr.ins.Y + lh, raster.Fix32(fr.R.Max.Y << 8))
		fr.lastFull = len(fr.glyphs)
	}
	fr.Limit = limit

	return
}
