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
	"yacco/util"
)

type Redrawable interface {
	Redraw(flush bool)
}

type ColorRune struct {
	C uint16
	R rune
}

const debugRedraw = false

// Callback when the frame needs to scroll its text
// If scrollDir is 0 then n is the absolute position to move to
// If scrollDir is -1 the text should be scrolled back by n lines
// If scrollDir is +1 the text should be scrolled forward by n lines
type FrameScrollFn func(scrollDir int, n int)
type ExpandSelectionFn func(kind, start, end int) (int, int)

const (
	HF_BOLSPACES uint32 = 1 << iota
	HF_MARKSOFTWRAP
	HF_QUOTEHACK
	HF_TRUNCATE   // truncates instead of softwrapping
	HF_NOVERTSTOP // Insert and InsertColor don't stop after they are past the bottom of the visible area
)

type Frame struct {
	Font            *util.Font
	Hackflags       uint32
	B               draw.Image      // the image the text will be drawn upon
	R               image.Rectangle // the rectangle occupied by the frame
	VisibleTick     bool
	Colors          [][]image.Uniform
	TabWidth        int
	Wnd             wde.Window
	Scroll          FrameScrollFn
	ExpandSelection ExpandSelectionFn
	Top             int
	Tabs            []int

	Limit image.Point

	margin raster.Fix32
	Offset int

	Sels []util.Sel

	glyphs   []glyph
	ins      raster.Point
	lastFull int

	redrawOpt struct {
		drawnVisibleTick bool
		drawnSels        []util.Sel
		reloaded         bool
		scrollStart      int
		scrollEnd        int
	}

	scrubGlyph image.Alpha

	debugRedraw bool
}

const COLORMASK = 0x0f

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
	fr.Offset = 0

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

	fr.Clear()

	return nil
}

func (fr *Frame) ReinitFont() {
	fr.Invalidate()
}

func (fr *Frame) initialInsPoint() raster.Point {
	gb := fr.Font.Bounds()
	return raster.Point{raster.Fix32(fr.R.Min.X<<8) + raster.Fix32(fr.Offset<<8) + fr.margin, raster.Fix32(fr.R.Min.Y<<8) + raster.Fix32(int32(fr.Font.SpacingFix(gb.YMax))<<8)}
}

func (fr *Frame) Clear() {
	fr.ins = fr.initialInsPoint()
	fr.glyphs = fr.glyphs[:0]
	fr.lastFull = 0
	fr.redrawOpt.reloaded = true
	fr.redrawOpt.scrollStart = -1
	fr.redrawOpt.scrollEnd = -1
}

func (fr *Frame) Invalidate() {
	fr.redrawOpt.reloaded = true
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

func (fr *Frame) toNextCell(spaceWidth, tabWidth, leftMargin raster.Fix32) raster.Fix32 {
	if fr.Tabs != nil {
		for i := range fr.Tabs {
			t := raster.Fix32(fr.Tabs[i]<<8) + leftMargin
			if fr.ins.X+spaceWidth/2 < t {
				return t - fr.ins.X
			}
		}
	}
	toNextCell := tabWidth - ((fr.ins.X - leftMargin) % tabWidth)
	if toNextCell <= spaceWidth/2 {
		toNextCell += tabWidth
	}
	return toNextCell
}

// Inserts text into the frame, returns the maximum X and Y used
func (fr *Frame) InsertColor(runes []ColorRune) (limit image.Point) {
	fr.redrawOpt.reloaded = true
	lh := fr.Font.LineHeightRaster()

	_, spaceIndex := fr.Font.Index(' ')

	prev, prevFontIdx, hasPrev := truetype.Index(0), 0, false

	rightMargin := raster.Fix32(fr.R.Max.X<<8) - fr.margin
	leftMargin := raster.Fix32(fr.R.Min.X<<8) + fr.margin
	bottom := raster.Fix32(fr.R.Max.Y<<8) + lh

	_, xIndex := fr.Font.Index('x')
	spaceWidth := fr.Font.GlyphWidth(0, spaceIndex)
	bigSpaceWidth := fr.Font.GlyphWidth(0, xIndex)
	tabWidth := spaceWidth * raster.Fix32(fr.TabWidth)

	limit.X = int(fr.ins.X >> 8)
	limit.Y = int(fr.ins.Y >> 8)

	for i, crune := range runes {
		if fr.ins.Y > bottom && (fr.Hackflags&HF_NOVERTSTOP == 0) {
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
				color:     uint8(crune.C & COLORMASK),
				width:     raster.Fix32(fr.R.Max.X<<8) - fr.ins.X - fr.margin,
				widthy:    lh}

			fr.glyphs = append(fr.glyphs, g)

			fr.ins.X = leftMargin
			fr.ins.Y += lh
			prev, prevFontIdx, hasPrev = spaceIndex, 0, true
		} else if crune.R == '\t' {
			toNextCell := fr.toNextCell(spaceWidth, tabWidth, leftMargin)

			g := glyph{
				r:         crune.R,
				fontIndex: 0,
				index:     spaceIndex,
				p:         fr.ins,
				color:     uint8(crune.C & COLORMASK),
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
				color:     uint8(crune.C & COLORMASK),
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

			fontIdx, index := fr.Font.Index(lur)
			width := fr.Font.GlyphWidth(fontIdx, index)
			kerning := raster.Fix32(0)
			if hasPrev && (fontIdx == prevFontIdx) {
				kerning = fr.Font.GlyphKerning(fontIdx, prev, index)
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
				color:     uint8(crune.C & COLORMASK),
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

func (fr *Frame) RefreshColors(a, b []ColorRune) {
	for i := range fr.glyphs {
		var crune ColorRune
		if i < len(a) {
			crune = a[i]
		} else {
			crune = b[i-len(a)]
		}
		fr.glyphs[i].r = crune.R
		fr.glyphs[i].color = uint8(crune.C & COLORMASK)
	}
}

// Mesures the length of the string
func (fr *Frame) Measure(rs []rune) int {
	ret := raster.Fix32(0)

	_, spaceIndex := fr.Font.Index(' ')
	spaceWidth := fr.Font.GlyphWidth(0, spaceIndex)
	tabWidth := spaceWidth * raster.Fix32(fr.TabWidth)

	prev, prevFontIdx, hasPrev := truetype.Index(0), 0, false

	for _, r := range rs {
		if r == '\t' {
			ret += tabWidth
		}

		lur := r

		if (fr.Hackflags & HF_QUOTEHACK) != 0 {
			if lur == '`' {
				lur = 0x2018
			} else if lur == '\'' {
				lur = 0x2019
			}
		}

		fontIdx, index := fr.Font.Index(lur)
		width := fr.Font.GlyphWidth(fontIdx, index)
		if hasPrev && (fontIdx == prevFontIdx) {
			width += fr.Font.GlyphKerning(fontIdx, prev, index)
		}
		ret += width
	}

	return int(ret >> 8)
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
					fr.Redraw(true, nil)
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
				fr.Redraw(true, nil)
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

	if fr.ExpandSelection != nil {
		nstart, nend := fr.ExpandSelection(kind, start, end)
		fr.Sels[idx].S = nstart
		fr.Sels[idx].E = nend
	} else {
		fr.Sels[idx].S = start
		fr.Sels[idx].E = end
	}
}

// Converts a graphical coordinate to a rune index
func (fr *Frame) CoordToPoint(coord image.Point) int {
	if !coord.In(fr.R) {
		return -1
	}

	ftcoord := freetype.Pt(coord.X, coord.Y)
	lh := fr.Font.LineHeightRaster()
	glyphBounds := fr.Font.Bounds()

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

func (fr *Frame) redrawSelection(s, e int, color *image.Uniform, invalid *[]image.Rectangle) {
	if s < 0 {
		s = 0
	}
	glyphBounds := fr.Font.Bounds()
	glyphBounds.YMax = int32(fr.Font.SpacingFix(glyphBounds.YMax))
	glyphBounds.YMin = int32(fr.Font.SpacingFix(glyphBounds.YMin))
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
		r = fr.R.Intersect(r)
		if invalid != nil {
			*invalid = append(*invalid, r)
		}
		drawingFuncs.DrawFillSrc(fr.B, r, color)
	} else {
		rs := fr.R.Intersect(image.Rectangle{sp, image.Point{int(rightMargin >> 8), int((ss.p.Y)>>8) - int(glyphBounds.YMin)}})
		re := fr.R.Intersect(image.Rectangle{sep, ep})
		rb := fr.R.Intersect(image.Rectangle{
			image.Point{sep.X, int((ss.p.Y)>>8) - int(glyphBounds.YMin)},
			image.Point{int(rightMargin >> 8), sep.Y},
		})
		if invalid != nil {
			*invalid = append(*invalid, rs, re, rb)
		}
		drawingFuncs.DrawFillSrc(fr.B, rs, color)
		drawingFuncs.DrawFillSrc(fr.B, re, color)
		drawingFuncs.DrawFillSrc(fr.B, rb, color)
	}
}

func (fr *Frame) reallyVisibleTick() bool {
	if !fr.VisibleTick || (fr.Sels[0].S != fr.Sels[0].E) {
		return false
	}
	if (fr.Sels[0].S-fr.Top < 0) || (fr.Sels[0].S-fr.Top > len(fr.glyphs)) {
		return false
	}

	return true
}

func (fr *Frame) drawTick(glyphBounds truetype.Bounds, drawingFuncs DrawingFuncs, idx int) image.Rectangle {
	if !fr.reallyVisibleTick() {
		return image.Rectangle{fr.R.Min, fr.R.Min}
	}

	var x, y int
	if len(fr.glyphs) == 0 {
		p := fr.initialInsPoint()
		x = int(p.X >> 8)
		y = int(p.Y >> 8)
	} else if fr.Sels[0].S-fr.Top < len(fr.glyphs) {
		p := fr.glyphs[fr.Sels[0].S-fr.Top].p
		x = int(p.X >> 8)
		y = int(p.Y >> 8)
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

	h := int(fr.Font.SpacingFix(glyphBounds.YMax))

	r := image.Rectangle{
		Min: image.Point{x, y - h},
		Max: image.Point{x + 1, y - int(glyphBounds.YMin)}}

	drawingFuncs.DrawFillSrc(fr.B, fr.R.Intersect(r), &fr.Colors[0][idx])

	r1 := r
	r1.Min.X -= 1
	r1.Max.X += 1
	r1.Max.Y = r1.Min.Y + 3
	drawingFuncs.DrawFillSrc(fr.B, fr.R.Intersect(r1), &fr.Colors[0][idx])

	r2 := r
	r2.Min.X -= 1
	r2.Max.X += 1
	r2.Min.Y = r2.Max.Y - 3
	drawingFuncs.DrawFillSrc(fr.B, fr.R.Intersect(r2), &fr.Colors[0][idx])

	rr := r
	rr.Min.X -= 1
	rr.Max.X += 1
	return rr
}

func (fr *Frame) deleteTick(glyphBounds truetype.Bounds, drawingFuncs DrawingFuncs) image.Rectangle {
	saved := fr.Sels[0]
	fr.Sels[0] = fr.redrawOpt.drawnSels[0]
	vt := fr.VisibleTick
	fr.VisibleTick = true
	fr.drawTick(glyphBounds, drawingFuncs, 0)
	fr.VisibleTick = vt

	if len(fr.glyphs) == 0 {
		fr.Sels[0] = saved
		return image.Rectangle{fr.R.Min, fr.R.Min}
	}
	var y int
	if fr.Sels[0].S == fr.Sels[0].E {
		if (fr.Sels[0].S-fr.Top >= 0) && (fr.Sels[0].S-fr.Top < len(fr.glyphs)) {
			fr.drawSingleGlyph(&fr.glyphs[fr.Sels[0].S-fr.Top], 0, drawingFuncs)
			if fr.Sels[0].S-fr.Top-1 >= 0 {
				fr.drawSingleGlyph(&fr.glyphs[fr.Sels[0].S-fr.Top-1], 0, drawingFuncs)
			}
			y = int(fr.glyphs[fr.Sels[0].S-fr.Top].p.Y >> 8)
		} else if (fr.Sels[0].S-fr.Top-1 >= 0) && (fr.Sels[0].S-fr.Top-1 < len(fr.glyphs)) {
			fr.drawSingleGlyph(&fr.glyphs[fr.Sels[0].S-fr.Top-1], 0, drawingFuncs)
			y = int((fr.glyphs[fr.Sels[0].S-fr.Top-1].p.Y + fr.glyphs[fr.Sels[0].S-fr.Top-1].widthy) >> 8)
		}
	}
	fr.Sels[0] = saved

	r := fr.R
	r.Min.Y = y - int(fr.Font.SpacingFix(glyphBounds.YMax))
	r.Max.Y = y - int(glyphBounds.YMin)
	return r
}

func (fr *Frame) updateRedrawOpt() {
	fr.redrawOpt.drawnVisibleTick = fr.reallyVisibleTick()
	if fr.redrawOpt.drawnSels == nil {
		fr.redrawOpt.drawnSels = make([]util.Sel, len(fr.Sels))
	}
	copy(fr.redrawOpt.drawnSels, fr.Sels)
	fr.redrawOpt.reloaded = false
	fr.redrawOpt.scrollStart = -1
	fr.redrawOpt.scrollEnd = -1
}

func (fr *Frame) redrawOptSelectionMoved(glyphBounds truetype.Bounds, drawingFuncs DrawingFuncs, leftMargin, rightMargin raster.Fix32) (bool, []image.Rectangle) {
	invalid := make([]image.Rectangle, 0, 3)

	if debugRedraw && fr.debugRedraw {
		fmt.Printf("%p Attempting to run optimized redraw\n", fr)
	}

	idx := -1
	failed := false
	for i := range fr.redrawOpt.drawnSels {
		changed := (fr.redrawOpt.drawnSels[i].S != fr.Sels[i].S) || (fr.redrawOpt.drawnSels[i].E != fr.Sels[i].E)
		if changed {
			if idx >= 0 {
				// failed, we want only one selection changed
				idx = -1
				failed = true
				break
			} else {
				idx = i
			}
		}
	}

	if debugRedraw {
		fmt.Printf("%v -> %v\n", fr.redrawOpt.drawnSels, fr.Sels)
	}

	if failed {
		if debugRedraw && fr.debugRedraw {
			fmt.Printf("\tFailed: multiple changed\n")
		}
		return false, nil
	}

	if idx < 0 {
		idx = 0
	}

	cs, ce := -1, -1

	fromnil := fr.redrawOpt.drawnSels[idx].S == fr.redrawOpt.drawnSels[idx].E
	tonil := fr.Sels[idx].S == fr.Sels[idx].E

	if fromnil && tonil {
		if idx == 0 {
			if debugRedraw && fr.debugRedraw {
				fmt.Printf("%p Redrawing tick move\n", fr)
			}
			if fr.redrawOpt.drawnVisibleTick {
				invalid = append(invalid, fr.deleteTick(glyphBounds, drawingFuncs))
			}

			invalid = append(invalid, fr.drawTick(glyphBounds, drawingFuncs, 1))
		}
		return true, invalid
	} else if fromnil && !tonil {
		if idx == 0 && fr.redrawOpt.drawnVisibleTick {
			invalid = append(invalid, fr.deleteTick(glyphBounds, drawingFuncs))
		}
		cs = fr.Sels[idx].S
		ce = fr.Sels[idx].E
	} else if !fromnil && tonil {
		cs = fr.redrawOpt.drawnSels[idx].S
		ce = fr.redrawOpt.drawnSels[idx].E
	} else if !fromnil && !tonil {
		// attempt to extend selection in one of two possible directions
		if fr.redrawOpt.drawnSels[idx].S == fr.Sels[idx].S {
			cs = fr.redrawOpt.drawnSels[idx].E
			ce = fr.Sels[idx].E
		} else if fr.redrawOpt.drawnSels[idx].E == fr.Sels[idx].E {
			cs = fr.redrawOpt.drawnSels[idx].S
			ce = fr.Sels[idx].S
		}

		if cs > ce {
			t := cs
			cs = ce
			ce = t
		}
	}

	if cs < 0 {
		if debugRedraw && fr.debugRedraw {
			fmt.Printf("\tFailed: noncontiguous selection\n")
		}
		return false, nil
	}

	rs := cs - fr.Top
	if rs < 0 {
		rs = 0
	} else if rs > len(fr.glyphs) {
		rs = len(fr.glyphs)
	}

	re := ce - fr.Top
	if re < 0 {
		re = 0
	} else if re > len(fr.glyphs) {
		re = len(fr.glyphs)
	}

	if rs != re {
		ssel := idx + 1
		if found, nssel := fr.getSsel(cs); found {
			ssel = nssel
		} else {
			ssel = 0
		}

		if debugRedraw && fr.debugRedraw {
			fmt.Printf("%p Redrawing selection %d change (%d %d) %d\n", fr, idx, cs, ce, ssel)
		}

		fr.redrawSelection(rs, re, &fr.Colors[ssel][0], &invalid)
		fr.redrawIntl(fr.glyphs[rs:re], false, rs, glyphBounds, rightMargin, leftMargin, drawingFuncs)
	}

	if tonil {
		invalid = append(invalid, fr.drawTick(glyphBounds, drawingFuncs, 1))
	}

	return true, invalid
}

func (fr *Frame) allSelectionsEmpty() bool {
	for i := range fr.Sels {
		if fr.Sels[i].S != fr.Sels[i].E {
			return false
		}
	}

	return true
}

func (fr *Frame) Redraw(flush bool, predrawRects *[]image.Rectangle) {
	glyphBounds := fr.Font.Bounds()
	rightMargin := raster.Fix32(fr.R.Max.X<<8) - fr.margin
	leftMargin := raster.Fix32(fr.R.Min.X<<8) + fr.margin

	drawingFuncs := GetOptimizedDrawing(fr.B)

	// FAST PATH 1
	// Followed only if:
	// - the frame wasn't reloaded (Clear, InsertColor weren't called) since last draw
	// - at most the tick changed position
	if !fr.redrawOpt.reloaded {
		if success, invalid := fr.redrawOptSelectionMoved(glyphBounds, drawingFuncs, leftMargin, rightMargin); success {
			fr.updateRedrawOpt()
			if flush && (fr.Wnd != nil) {
				fr.Wnd.FlushImage(invalid...)
			}
			if predrawRects != nil {
				*predrawRects = append(*predrawRects, invalid...)
			}
			return
		}
	}

	// FAST PATH 2
	// Followed only after a scroll operation and there are no active selections
	// Bitmaps are copied directly
	if fr.redrawOpt.scrollStart >= 0 {
		if debugRedraw && fr.debugRedraw {
			fmt.Printf("%p Redrawing (scroll) scrollStart: %d\n", fr, fr.redrawOpt.scrollStart)
		}
		if fr.redrawOpt.scrollEnd < 0 {
			fr.redrawOpt.scrollEnd = len(fr.glyphs)
		}
		fr.redrawIntl(fr.glyphs[fr.redrawOpt.scrollStart:fr.redrawOpt.scrollEnd], true, fr.redrawOpt.scrollStart, glyphBounds, rightMargin, leftMargin, drawingFuncs)
		tp := fr.Sels[0].S - fr.Top
		if tp >= fr.redrawOpt.scrollStart && tp <= fr.redrawOpt.scrollEnd {
			fr.drawTick(glyphBounds, drawingFuncs, 1)
		}
		fr.updateRedrawOpt()
		if flush && (fr.Wnd != nil) {
			fr.Wnd.FlushImage(fr.R)
		}
		if predrawRects != nil {
			*predrawRects = append(*predrawRects, fr.R)
		}
		return
	}

	fr.updateRedrawOpt()

	// NORMAL PATH HERE
	if debugRedraw && fr.debugRedraw {
		fmt.Printf("%p Redrawing (FULL)\n", fr)
	}

	// background
	drawingFuncs.DrawFillSrc(fr.B, fr.R, &fr.Colors[0][0])

	fr.redrawIntl(fr.glyphs, true, 0, glyphBounds, rightMargin, leftMargin, drawingFuncs)

	// Tick drawing
	fr.drawTick(glyphBounds, drawingFuncs, 1)

	if flush && (fr.Wnd != nil) {
		fr.Wnd.FlushImage(fr.R)
	}

	if predrawRects != nil {
		*predrawRects = append(*predrawRects, fr.R)
	}
}

func (fr *Frame) getSsel(i int) (bool, int) {
	for j := len(fr.Colors) - 2; j >= 0; j-- {
		if (i >= fr.Sels[j].S) && (i < fr.Sels[j].E) {
			return true, j + 1
		}
	}
	return false, -1
}

func (fr *Frame) redrawIntl(glyphs []glyph, drawSels bool, n int, glyphBounds truetype.Bounds, rightMargin, leftMargin raster.Fix32, drawingFuncs DrawingFuncs) {
	ssel := 0
	var cury raster.Fix32 = 0
	if len(fr.glyphs) > 0 {
		cury = fr.glyphs[0].p.Y
	}
	newline := true

	for i, g := range glyphs {
		// Selection drawing
		if ssel != 0 {
			if i+fr.Top+n >= fr.Sels[ssel-1].E {
				ssel = 0
			}
		} else {
			if found, nssel := fr.getSsel(i + fr.Top + n); found {
				ssel = nssel
				if drawSels {
					fr.redrawSelection(fr.Sels[ssel-1].S-fr.Top, fr.Sels[ssel-1].E-fr.Top, &fr.Colors[ssel][0], nil)
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
		mask, glyphRect, err := fr.Font.Glyph(g.fontIndex, g.index, g.p)
		if err != nil {
			panic(err)
		}
		dr := fr.R.Intersect(glyphRect)
		if !dr.Empty() {
			//println("Glyph", string([]rune{ g.r }), "pos", (g.p.X>>8), "rect X", dr.Min.X, dr.Max.X, glyphRect.Min.X)
			mp := image.Point{dr.Min.X - glyphRect.Min.X, dr.Min.Y - glyphRect.Min.Y}
			color := &fr.Colors[1][1]
			if (ssel >= 0) && (ssel < len(fr.Colors)) && (g.color >= 0) && (int(g.color) < len(fr.Colors[ssel])) {
				color = &fr.Colors[ssel][g.color]
			}
			drawingFuncs.DrawGlyphOver(fr.B, dr, color, mask, mp)
		}
	}
}

func (fr *Frame) drawSingleGlyph(g *glyph, ssel int, drawingFuncs DrawingFuncs) {
	// Glyph drawing
	mask, glyphRect, err := fr.Font.Glyph(g.fontIndex, g.index, g.p)
	if err != nil {
		panic(err)
	}
	dr := fr.R.Intersect(glyphRect)
	if !dr.Empty() {
		//println("Glyph", string([]rune{ g.r }), "pos", (g.p.X>>8), "rect X", dr.Min.X, dr.Max.X, glyphRect.Min.X)
		mp := image.Point{dr.Min.X - glyphRect.Min.X, dr.Min.Y - glyphRect.Min.Y}
		color := &fr.Colors[1][1]
		bgcolor := &fr.Colors[1][0]
		if (ssel >= 0) && (ssel < len(fr.Colors)) && (g.color >= 0) && (int(g.color) < len(fr.Colors[ssel])) {
			color = &fr.Colors[ssel][g.color]
			bgcolor = &fr.Colors[ssel][0]
		}

		// Clear old glyph
		if fr.scrubGlyph.Pix == nil || len(mask.Pix) > cap(fr.scrubGlyph.Pix) {
			fr.scrubGlyph.Pix = make([]uint8, len(mask.Pix))
		} else {
			fr.scrubGlyph.Pix = fr.scrubGlyph.Pix[:len(mask.Pix)]
		}
		fr.scrubGlyph.Stride = mask.Stride
		fr.scrubGlyph.Rect = mask.Rect
		for i := range mask.Pix {
			if mask.Pix[i] > 0 {
				fr.scrubGlyph.Pix[i] = 0xff
			} else {
				fr.scrubGlyph.Pix[i] = 0x00
			}
		}
		drawingFuncs.DrawGlyphOver(fr.B, dr, bgcolor, &fr.scrubGlyph, mp)

		// Redraw glyph
		drawingFuncs.DrawGlyphOver(fr.B, dr, color, mask, mp)
	}
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
		return nil
	}

	if e.Which == wde.WheelDownButton {
		f.Scroll(+1, 1)
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
		f.Redraw(true, nil)
		ee := f.Select(sel, e.Count, e.Which, events)
		f.Redraw(true, nil)
		return ee
	}

	return nil
}

func (fr *Frame) LineNo() int {
	return int(float32(fr.R.Max.Y-fr.R.Min.Y) / float32(fr.Font.LineHeightRaster()>>8))
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
func (fr *Frame) PhisicalLines() []int {
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
func (fr *Frame) PushUp(ln int, drawOpt bool) (newsize int) {
	fr.ins = fr.initialInsPoint()

	lh := fr.Font.LineHeightRaster()

	fr.Limit.X = int(fr.ins.X >> 8)
	fr.Limit.Y = int(fr.ins.Y >> 8)

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
		g := fr.glyphs[len(fr.glyphs)-1]
		copy(fr.glyphs, fr.glyphs[off:])
		fr.glyphs = fr.glyphs[:len(fr.glyphs)-off-1]
		fr.ins.X = g.p.X
		fr.ins.Y = g.p.Y
		fr.InsertColor([]ColorRune{ColorRune{uint16(g.color), g.r}})
	} else {
		fr.Top += len(fr.glyphs)
		fr.glyphs = []glyph{}
	}

	fr.lastFull = len(fr.glyphs)

	if fr.allSelectionsEmpty() && drawOpt {
		drawingFuncs := GetOptimizedDrawing(fr.B)

		h := ln * int(lh>>8)

		for fr.redrawOpt.scrollStart = len(fr.glyphs) - 1; fr.redrawOpt.scrollStart > 0; fr.redrawOpt.scrollStart-- {
			g := fr.glyphs[fr.redrawOpt.scrollStart]
			if int((g.p.Y+lh)>>8) < (fr.R.Max.Y - h) {
				break
			}
		}
		fr.redrawOpt.scrollEnd = -1

		p := fr.R.Min
		p.Y += h
		r := fr.R
		r.Max.Y -= h
		drawingFuncs.DrawCopy(fr.B, r, fr.B, p)

		r = fr.R
		if (fr.redrawOpt.scrollStart >= 0) && (fr.redrawOpt.scrollStart < len(fr.glyphs)) {
			bounds := fr.Font.Bounds()
			r.Min.Y = int(fr.glyphs[fr.redrawOpt.scrollStart].p.Y>>8) - int(bounds.YMin)
		} else {
			r.Min.Y = fr.R.Max.Y - h
		}
		drawingFuncs.DrawFillSrc(fr.B, r, &fr.Colors[0][0])
	}

	return len(fr.glyphs)
}

func (fr *Frame) PushDown(ln int, a, b []ColorRune) (limit image.Point) {
	oldglyphs := make([]glyph, len(fr.glyphs))
	copy(oldglyphs, fr.glyphs)

	fr.Top -= len(a) + len(b)
	fr.Clear()

	for {
		ng := len(fr.glyphs)

		if len(a) > 0 {
			fr.InsertColor(a)
		}
		if len(b) > 0 {
			fr.InsertColor(b)
		}

		limit = fr.Limit

		pl := fr.PhisicalLines()
		if len(pl) <= ln {
			break
		}

		added := len(fr.glyphs) - ng

		fr.PushUp(len(pl)-ln, false)

		if added <= 0 {
			break
		}

		if len(a) > added {
			a = a[added:]
		} else {
			added -= len(a)
			a = []ColorRune{}
			if len(b) > added {
				b = b[added:]
			} else {
				b = []ColorRune{}
			}
		}
	}

	lh := fr.Font.LineHeightRaster()

	if fr.allSelectionsEmpty() {
		drawingFuncs := GetOptimizedDrawing(fr.B)

		fr.redrawOpt.scrollStart = 0
		fr.redrawOpt.scrollEnd = len(fr.glyphs)

		h := len(fr.PhisicalLines()) * int(lh>>8)
		r := fr.R
		r.Min.Y += h
		drawingFuncs.DrawCopy(fr.B, r, fr.B, fr.R.Min)

		r = fr.R
		r.Max.Y = r.Min.Y + h
		r = r.Intersect(fr.R)
		drawingFuncs.DrawFillSrc(fr.B, r, &fr.Colors[0][0])
	}

	leftMargin := raster.Fix32(fr.R.Min.X<<8) + fr.margin
	bottom := raster.Fix32(fr.R.Max.Y<<8) + lh

	if fr.ins.X != leftMargin {
		fr.ins.X = leftMargin
		fr.ins.Y += lh
	}

	oldY := raster.Fix32(0)
	if len(oldglyphs) > 0 {
		oldY = oldglyphs[0].p.Y
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

		if oldglyphs[i].p.Y != oldY {
			fr.ins.Y += lh
			oldY = oldglyphs[i].p.Y
		}

		oldglyphs[i].p.Y = fr.ins.Y
		fr.ins.X = oldglyphs[i].p.X

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

func (fr *Frame) Size() int {
	return len(fr.glyphs)
}
