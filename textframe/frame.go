package textframe

import (
	"fmt"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/mouse"
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
const optiStats = false

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
	Font2           *util.Font
	Hackflags       uint32
	B               draw.Image      // the image the text will be drawn upon
	R               image.Rectangle // the rectangle occupied by the frame
	VisibleTick     bool
	Colors          [][]image.Uniform
	TabWidth        int
	Flush           func(...image.Rectangle)
	Scroll          FrameScrollFn
	ExpandSelection ExpandSelectionFn
	Top             int
	Tabs            []int

	margin fixed.Int26_6
	Offset int

	Sel      util.Sel
	SelColor int
	PMatch   util.Sel

	glyphs   []glyph
	ins      fixed.Point26_6
	lastFull int

	redrawOpt struct {
		drawnVisibleTick bool
		drawnSel         util.Sel
		drawnPMatch      util.Sel
		selColor         int
		reloaded         bool
		scrollStart      int
		scrollEnd        int
	}

	scrubGlyph image.Alpha

	debugRedraw bool

	glyphBounds             fixed.Rectangle26_6
	leftMargin, rightMargin fixed.Int26_6
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

type subfont struct {
	idx     int
	fontIdx int
}

type glyph struct {
	r       rune
	subfont subfont
	index   truetype.Index
	width   fixed.Int26_6
	kerning fixed.Int26_6
	widthy  fixed.Int26_6
	p       fixed.Point26_6
	color   uint8
}

// Initializes frame
func (fr *Frame) Init(margin int) error {
	fr.margin = fixed.I(margin)
	fr.glyphs = []glyph{}
	fr.Offset = 0
	fr.SelColor = 0

	if fr.TabWidth == 0 {
		fr.TabWidth = 8
	}

	// sanity checks

	if len(fr.Colors) < 2 {
		return fmt.Errorf("Not enough color lines")
	}

	for i, _ := range fr.Colors {
		if len(fr.Colors[i]) < 2 {
			return fmt.Errorf("Not enough colors in line %d (%d)", i, len(fr.Colors[i]))
		}
	}

	fr.Clear()

	return nil
}

func (fr *Frame) ReinitFont() {
	fr.Invalidate()
}

func (fr *Frame) initialInsPoint() fixed.Point26_6 {
	gb := fr.Font.Bounds()
	//p := fixed.P(fr.R.Min.X + fr.Offset, fr.R.Min.Y + int(fr.Font.SpacingFix(int32(util.FixedToInt(gb.Max.Y)))))
	p := fixed.P(fr.R.Min.X+fr.Offset, fr.R.Min.Y+util.FixedToInt(gb.Max.Y))
	p.X += fr.margin
	return p
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

func (fr *Frame) toNextCell(spaceWidth, tabWidth, leftMargin fixed.Int26_6) fixed.Int26_6 {
	if fr.Tabs != nil {
		for i := range fr.Tabs {
			t := fixed.I(fr.Tabs[i]) + leftMargin
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

	fr.rightMargin = fixed.I(fr.R.Max.X) - fr.margin
	fr.leftMargin = fixed.I(fr.R.Min.X) + fr.margin
	bottom := fixed.I(fr.R.Max.Y) + lh

	_, xIndex := fr.Font.Index('x')
	spaceWidth, _, _, _ := fr.Font.Glyph(0, spaceIndex, fixed.P(0, 0))
	bigSpaceWidth, _, _, _ := fr.Font.Glyph(0, xIndex, fixed.P(0, 0))
	tabWidth := spaceWidth * fixed.Int26_6(fr.TabWidth)

	limit.X = util.FixedToInt(fr.ins.X)
	limit.Y = util.FixedToInt(fr.ins.Y)

	for i, crune := range runes {
		if fr.ins.Y > bottom && (fr.Hackflags&HF_NOVERTSTOP == 0) {
			return
		}

		if fr.ins.Y < fixed.I(fr.R.Max.Y) {
			fr.lastFull = len(fr.glyphs)
		}

		if crune.R == '\n' {
			g := glyph{
				r:       crune.R,
				subfont: subfont{0, 0},
				index:   spaceIndex,
				p:       fr.ins,
				color:   uint8(crune.C & COLORMASK),
				width:   fixed.I(fr.R.Max.X) - fr.ins.X - fr.margin,
				widthy:  lh,
			}

			fr.glyphs = append(fr.glyphs, g)

			fr.ins.X = fr.leftMargin
			fr.ins.Y += lh
			prev, prevFontIdx, hasPrev = spaceIndex, 0, true
		} else if crune.R == '\t' {
			toNextCell := fr.toNextCell(spaceWidth, tabWidth, fr.leftMargin)

			g := glyph{
				r:       crune.R,
				subfont: subfont{0, 0},
				index:   spaceIndex,
				p:       fr.ins,
				color:   uint8(crune.C & COLORMASK),
				width:   toNextCell,
			}

			fr.glyphs = append(fr.glyphs, g)

			fr.ins.X += toNextCell
			prev, prevFontIdx, hasPrev = spaceIndex, 0, true
		} else if (crune.R == ' ') && (fr.Hackflags&HF_BOLSPACES != 0) {
			width := fixed.I(0)
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
				r:       crune.R,
				subfont: subfont{0, 0},
				index:   spaceIndex,
				p:       fr.ins,
				color:   uint8(crune.C & COLORMASK),
				width:   width,
			}

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

			f := fr.Font
			fidx := 0
			if crune.C > 1 {
				fidx = 1
				f = fr.Font2
			}

			fontIdx, index := f.Index(lur)
			width, _, _, _ := f.Glyph(fontIdx, index, fr.ins)
			kerning := fixed.I(0)
			if hasPrev && (fontIdx == prevFontIdx) {
				kerning = f.GlyphKerning(fontIdx, prev, index)
				fr.ins.X += kerning
			}

			if fr.Hackflags&HF_TRUNCATE == 0 {
				if fr.ins.X+width > fr.rightMargin {
					fr.ins.X = fr.leftMargin
					fr.ins.Y += lh
				}
			}

			g := glyph{
				r:       crune.R,
				subfont: subfont{fidx, fontIdx},
				index:   index,
				p:       fr.ins,
				color:   uint8(crune.C & COLORMASK),
				kerning: kerning,
				width:   width,
			}

			fr.glyphs = append(fr.glyphs, g)

			fr.ins.X += width
			prev, prevFontIdx, hasPrev = index, fontIdx, true
		}

		if util.FixedToInt(fr.ins.X) > limit.X {
			limit.X = util.FixedToInt(fr.ins.X)
		}

		if util.FixedToInt(fr.ins.Y) > limit.Y {
			limit.Y = util.FixedToInt(fr.ins.Y)
		}
	}
	if fr.ins.Y < fixed.I(fr.R.Max.Y) {
		fr.lastFull = len(fr.glyphs)
	}
	return
}

// Mesures the length of the string
func (fr *Frame) Measure(rs []rune) int {
	ret := fixed.I(0)

	_, spaceIndex := fr.Font.Index(' ')
	spaceWidth, _, _, _ := fr.Font.Glyph(0, spaceIndex, fixed.P(0, 0))
	tabWidth := spaceWidth * fixed.Int26_6(fr.TabWidth)

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
		width, _, _, _ := fr.Font.Glyph(fontIdx, index, fixed.P(0, 0))
		if hasPrev && (fontIdx == prevFontIdx) {
			width += fr.Font.GlyphKerning(fontIdx, prev, index)
		}
		ret += width
	}

	return util.FixedToInt(ret)
}

// Tracks the mouse position, selecting text, the events channel is from go.wde
// kind is 1 for character by character selection, 2 for word by word selection, 3 for line by line selection
func (fr *Frame) Select(idx, kind int, button mouse.Button, events <-chan interface{}) *mouse.Event {
	if (idx < 0) || (idx >= len(fr.Colors)-1) {
		for ei := range events {
			switch e := ei.(type) {
			case mouse.Event:
				if e.Direction == mouse.DirRelease {
					return &e
				}
			}
		}
	}

	fr.PMatch.E = fr.PMatch.S
	fr.SelColor = idx
	fix := fr.Sel.S
	var autoscrollTicker *time.Ticker
	var autoscrollChan <-chan time.Time

	stopAutoscroll := func() {
		if autoscrollTicker != nil {
			autoscrollTicker.Stop()
			autoscrollTicker = nil
		}
	}

	var lastPos image.Point

	r := fr.R
	r.Max.Y -= 2

	for {
		runtime.Gosched()
		select {
		case ei := <-events:
			switch e := ei.(type) {
			case mouse.Event:
				if e.Direction == mouse.DirRelease {
					stopAutoscroll()
					return &e
				}

				where := image.Point{int(e.X), int(e.Y)}
				lastPos = where
				if where.In(r) {
					stopAutoscroll()

					p := fr.CoordToPoint(where)
					fr.SetSelect(idx, kind, fix, p)
					fr.Redraw(true, nil)
				} else {
					if autoscrollTicker == nil {
						autoscrollTicker = time.NewTicker(100 * time.Millisecond)
						autoscrollChan = autoscrollTicker.C
					}
				}
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
	fr.setSelectEx(idx, kind, start, end)
}

func (fr *Frame) setSelectEx(idx, kind, start, end int) {
	if (idx < 0) || (idx >= len(fr.Colors)) {
		idx = 0
	}

	fr.SelColor = idx

	if start >= end {
		temp := start
		start = end
		end = temp
	}

	if fr.ExpandSelection != nil {
		nstart, nend := fr.ExpandSelection(kind, start, end)
		fr.Sel.S = nstart
		fr.Sel.E = nend
	} else {
		fr.Sel.S = start
		fr.Sel.E = end
	}
}

// Converts a graphical coordinate to a rune index
func (fr *Frame) CoordToPoint(coord image.Point) int {
	if !coord.In(fr.R) {
		return -1
	}

	ftcoord := freetype.Pt(coord.X, coord.Y)
	lh := fr.Font.LineHeightRaster()
	fr.glyphBounds = fr.Font.Bounds()

	for i, g := range fr.glyphs {
		if g.p.Y-fr.glyphBounds.Min.Y < ftcoord.Y {
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
		var r fixed.Point26_6
		if len(fr.glyphs) == 0 {
			r = fr.ins
		} else {
			r = fr.glyphs[0].p
		}
		return image.Point{util.FixedToInt(r.X), util.FixedToInt(r.Y)}
	} else if pp < len(fr.glyphs) {
		r := fr.glyphs[pp].p
		return image.Point{util.FixedToInt(r.X), util.FixedToInt(r.Y)}
	} else if (pp == len(fr.glyphs)) && (len(fr.glyphs) > 0) {
		r := fr.glyphs[pp-1].p
		return image.Point{util.FixedToInt((r.X + fr.glyphs[pp-1].width)), util.FixedToInt(r.Y)}
	} else {
		return image.Point{fr.R.Min.X + 2, fr.R.Min.Y + 2}
	}
}

func (fr *Frame) redrawSelection(s, e int, color *image.Uniform, invalid *[]image.Rectangle) {
	if s < 0 {
		s = 0
	}
	glyphBounds := fr.Font.Bounds()

	var sp, ep, sep image.Point

	ss := fr.glyphs[s]
	sp = image.Point{util.FixedToInt(ss.p.X), util.FixedToInt(ss.p.Y - glyphBounds.Max.Y)}

	var se glyph

	if e < len(fr.glyphs) {
		se = fr.glyphs[e]
		sep = image.Point{util.FixedToInt(fr.leftMargin), util.FixedToInt(se.p.Y - glyphBounds.Max.Y)}
		ep = image.Point{util.FixedToInt(se.p.X), util.FixedToInt(se.p.Y - glyphBounds.Min.Y)}
	} else if e == len(fr.glyphs) {
		se = fr.glyphs[len(fr.glyphs)-1]
		sep = image.Point{util.FixedToInt(fr.leftMargin), util.FixedToInt(se.p.Y - glyphBounds.Max.Y)}
		ep = image.Point{util.FixedToInt(se.p.X + se.width), util.FixedToInt(se.p.Y - glyphBounds.Min.Y)}
	} else {
		se = fr.glyphs[len(fr.glyphs)-1]
		sep = image.Point{util.FixedToInt(fr.leftMargin), util.FixedToInt(se.p.Y - glyphBounds.Max.Y)}
		ep = image.Point{util.FixedToInt(fr.rightMargin), fr.R.Max.Y}
	}

	if ss.p.Y == se.p.Y {
		r := image.Rectangle{sp, ep}
		r = fr.R.Intersect(r)
		if invalid != nil {
			*invalid = append(*invalid, r)
		}
		draw.Draw(fr.B, r, color, r.Min, draw.Src)
	} else {
		rs := fr.R.Intersect(image.Rectangle{sp, image.Point{util.FixedToInt(fr.rightMargin), util.FixedToInt(ss.p.Y - glyphBounds.Min.Y)}})
		re := fr.R.Intersect(image.Rectangle{sep, ep})
		rb := fr.R.Intersect(image.Rectangle{
			image.Point{sep.X, util.FixedToInt(ss.p.Y - glyphBounds.Min.Y)},
			image.Point{util.FixedToInt(fr.rightMargin), sep.Y},
		})
		if invalid != nil {
			*invalid = append(*invalid, rs, re, rb)
		}
		draw.Draw(fr.B, rs, color, rs.Min, draw.Src)
		draw.Draw(fr.B, re, color, re.Min, draw.Src)
		draw.Draw(fr.B, rb, color, rb.Min, draw.Src)
	}
}

func (fr *Frame) reallyVisibleTick() bool {
	if !fr.VisibleTick || (fr.Sel.S != fr.Sel.E) {
		return false
	}
	if (fr.Sel.S-fr.Top < 0) || (fr.Sel.S-fr.Top > len(fr.glyphs)) {
		return false
	}

	return true
}

func (fr *Frame) drawTick(idx int) image.Rectangle {
	if !fr.reallyVisibleTick() {
		return image.Rectangle{fr.R.Min, fr.R.Min}
	}

	var x, y int
	if len(fr.glyphs) == 0 {
		p := fr.initialInsPoint()
		x = util.FixedToInt(p.X)
		y = util.FixedToInt(p.Y)
	} else if fr.Sel.S-fr.Top < len(fr.glyphs) {
		p := fr.glyphs[fr.Sel.S-fr.Top].p
		x = util.FixedToInt(p.X)
		y = util.FixedToInt(p.Y)
	} else {
		g := fr.glyphs[len(fr.glyphs)-1]

		if g.widthy > 0 {
			x = fr.R.Min.X + util.FixedToInt(fr.margin)
			y = util.FixedToInt(g.p.Y + g.widthy)
		} else {
			x = util.FixedToInt(g.p.X+g.width) + 1
			y = util.FixedToInt(g.p.Y)
		}
	}

	//h := int(fr.Font.SpacingFix(fr.glyphBounds.YMax))
	h := util.FixedToInt(fr.glyphBounds.Max.Y)

	basedx := int(math.Floor(fr.Font.Size/14 + .5))
	if basedx < 1 {
		basedx = 1
	}

	basedxl := basedx / 2
	basedxr := basedxl
	if basedxl+basedxr < basedx {
		basedxr++
	}

	r := image.Rectangle{
		Min: image.Point{x - basedxl, y - h},
		Max: image.Point{x + basedxr, y - util.FixedToInt(fr.glyphBounds.Min.Y)}}

	draw.Draw(fr.B, fr.R.Intersect(r), &fr.Colors[0][idx], fr.R.Intersect(r).Min, draw.Src)

	r1 := r
	r1.Min.X -= r.Dx()
	r1.Max.X += r.Dx()
	r1.Max.Y = r1.Min.Y + r.Dx()*3
	draw.Draw(fr.B, fr.R.Intersect(r1), &fr.Colors[0][idx], fr.R.Intersect(r1).Min, draw.Src)

	r2 := r
	r2.Min.X -= r.Dx()
	r2.Max.X += r.Dx()
	r2.Min.Y = r2.Max.Y - r.Dx()*3
	draw.Draw(fr.B, fr.R.Intersect(r2), &fr.Colors[0][idx], fr.R.Intersect(r2).Min, draw.Src)

	rr := r
	rr.Min.X -= r.Dx()
	rr.Max.X += r.Dx()
	return rr
}

func (fr *Frame) deleteTick() image.Rectangle {
	saved := fr.Sel
	fr.Sel = fr.redrawOpt.drawnSel
	vt := fr.VisibleTick
	fr.VisibleTick = true
	r := fr.drawTick(0)
	fr.VisibleTick = vt

	if len(fr.glyphs) == 0 {
		fr.Sel = saved
		return image.Rectangle{fr.R.Min, fr.R.Min}
	}
	var y int
	if fr.Sel.S == fr.Sel.E {
		if (fr.Sel.S-fr.Top >= 0) && (fr.Sel.S-fr.Top < len(fr.glyphs)) {
			fr.drawSingleGlyph(&fr.glyphs[fr.Sel.S-fr.Top], 0)
			if fr.Sel.S-fr.Top-1 >= 0 {
				fr.drawSingleGlyph(&fr.glyphs[fr.Sel.S-fr.Top-1], 0)
			}
			y = util.FixedToInt(fr.glyphs[fr.Sel.S-fr.Top].p.Y)
		} else if (fr.Sel.S-fr.Top-1 >= 0) && (fr.Sel.S-fr.Top-1 < len(fr.glyphs)) {
			fr.drawSingleGlyph(&fr.glyphs[fr.Sel.S-fr.Top-1], 0)
			y = util.FixedToInt(fr.glyphs[fr.Sel.S-fr.Top-1].p.Y + fr.glyphs[fr.Sel.S-fr.Top-1].widthy)
		}
	}
	fr.Sel = saved

	_ = y

	return r
}

func (fr *Frame) updateRedrawOpt() {
	fr.redrawOpt.drawnVisibleTick = fr.reallyVisibleTick()
	fr.redrawOpt.drawnSel = fr.Sel
	fr.redrawOpt.drawnPMatch = fr.PMatch
	fr.redrawOpt.selColor = fr.SelColor
	fr.redrawOpt.reloaded = false
	fr.redrawOpt.scrollStart = -1
	fr.redrawOpt.scrollEnd = -1
}

func (fr *Frame) redrawOptSelectionMoved() (bool, []image.Rectangle) {
	if fr.redrawOpt.selColor != fr.SelColor {
		return false, nil
	}
	invalid := make([]image.Rectangle, 0, 3)

	if debugRedraw && fr.debugRedraw {
		fmt.Printf("%p Attempting to run optimized redraw\n", fr)
	}

	if debugRedraw {
		fmt.Printf("%v -> %v\n", fr.redrawOpt.drawnSel, fr.Sel)
	}

	fromnil := fr.redrawOpt.drawnSel.S == fr.redrawOpt.drawnSel.E
	tonil := fr.Sel.S == fr.Sel.E

	if !fromnil || !tonil {
		return false, nil
	}

	if debugRedraw && fr.debugRedraw {
		fmt.Printf("%p Redrawing tick move\n", fr)
	}
	if fr.redrawOpt.drawnVisibleTick {
		invalid = append(invalid, fr.deleteTick())
	}

	invalid = append(invalid, fr.drawTick(1))

	if len(fr.Colors) > 4 {
		if debugRedraw && fr.debugRedraw {
			fmt.Printf("\tRedrawing parenthesis match (1): %v -> %v\n", fr.redrawOpt.drawnPMatch, fr.PMatch)
		}
		fr.redrawSelectionLogical(fr.redrawOpt.drawnPMatch, &invalid)
		fr.redrawSelectionLogical(fr.PMatch, &invalid)
	}

	return true, invalid
}

func (fr *Frame) redrawSelectionLogical(sel util.Sel, invalid *[]image.Rectangle) {
	if sel.S == sel.E {
		return
	}

	var color *image.Uniform
	if sel.S >= fr.PMatch.S && sel.E <= fr.PMatch.E {
		color = &fr.Colors[4][0]
	} else if sel.S >= fr.Sel.S && sel.E <= fr.Sel.E {
		color = &fr.Colors[fr.SelColor+1][0]
	} else {
		color = &fr.Colors[0][0]
	}

	rs := sel.S - fr.Top
	re := sel.E - fr.Top

	if re < 0 {
		return
	}

	if rs >= len(fr.glyphs) {
		return
	}

	fr.redrawSelection(rs, re, color, invalid)
	fr.redrawIntl(fr.glyphs[rs:re], false, rs)
}

func (fr *Frame) allSelectionsEmpty() bool {
	return (fr.Sel.S == fr.Sel.E) && (fr.PMatch.S == fr.PMatch.E)

}

func calcPixels(invalid []image.Rectangle) int {
	a := 0
	for i := range invalid {
		a += invalid[i].Dx() * invalid[i].Dy()
	}
	return a
}

func (fr *Frame) Redraw(flush bool, predrawRects *[]image.Rectangle) {
	fr.glyphBounds = fr.Font.Bounds()
	fr.rightMargin = fixed.I(fr.R.Max.X) - fr.margin
	fr.leftMargin = fixed.I(fr.R.Min.X) + fr.margin

	// FAST PATH 1
	// Followed only if:
	// - the frame wasn't reloaded (Clear, InsertColor weren't called) since last draw
	// - at most the tick changed position
	if !fr.redrawOpt.reloaded {
		if success, invalid := fr.redrawOptSelectionMoved(); success {
			fr.updateRedrawOpt()
			if flush && (fr.Flush != nil) {
				fr.Flush(invalid...)
			}
			if predrawRects != nil {
				*predrawRects = append(*predrawRects, invalid...)
			}
			if optiStats {
				fmt.Printf("%p Invalidating %d pixels\n", fr, calcPixels(invalid))
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
		fr.redrawIntl(fr.glyphs[fr.redrawOpt.scrollStart:fr.redrawOpt.scrollEnd], true, fr.redrawOpt.scrollStart)
		tp := fr.Sel.S - fr.Top
		if tp >= fr.redrawOpt.scrollStart && tp <= fr.redrawOpt.scrollEnd {
			fr.drawTick(1)
		}
		fr.updateRedrawOpt()
		if flush && (fr.Flush != nil) {
			fr.Flush(fr.R)
		}
		if predrawRects != nil {
			*predrawRects = append(*predrawRects, fr.R)
		}
		if optiStats {
			fmt.Printf("Full invalidation (scroll) %d\n", calcPixels([]image.Rectangle{fr.R}))
		}
		return
	}

	fr.updateRedrawOpt()

	// NORMAL PATH HERE
	if debugRedraw && fr.debugRedraw {
		fmt.Printf("%p Redrawing (FULL)\n", fr)
	}
	if optiStats && fr.debugRedraw {
		fmt.Printf("Full invalidation (full) %d\n", calcPixels([]image.Rectangle{fr.R}))
	}

	// background
	draw.Draw(fr.B, fr.R, &fr.Colors[0][0], fr.R.Min, draw.Src)

	fr.redrawIntl(fr.glyphs, true, 0)

	// Tick drawing
	fr.drawTick(1)

	if flush && (fr.Flush != nil) {
		fr.Flush(fr.R)
	}

	if predrawRects != nil {
		*predrawRects = append(*predrawRects, fr.R)
	}
}

func (fr *Frame) getSsel(i int) (bool, int) {
	if (i >= fr.Sel.S) && (i < fr.Sel.E) {
		return true, fr.SelColor + 1
	}
	return false, -1
}

func (fr *Frame) redrawIntl(glyphs []glyph, drawSels bool, n int) {
	ssel := 0
	cury := fixed.I(0)
	if len(fr.glyphs) > 0 {
		cury = fr.glyphs[0].p.Y
	}
	newline := true

	between := func(x, a, b int) bool {
		return x >= a && x < b
	}

	in := func(x int) bool {
		return between(x-fr.Top, n, n+len(glyphs))
	}

	if drawSels {
		if fr.PMatch.S != fr.PMatch.E && len(fr.Colors) > 4 && in(fr.PMatch.S) {
			fr.redrawSelection(fr.PMatch.S-fr.Top, fr.PMatch.E-fr.Top, &fr.Colors[4][0], nil)
		}

		if fr.Sel.S != fr.Sel.E && (in(fr.Sel.S) || in(fr.Sel.E) || between(n, fr.Sel.S-fr.Top, fr.Sel.E-fr.Top)) {
			fr.redrawSelection(fr.Sel.S-fr.Top, fr.Sel.E-fr.Top, &fr.Colors[fr.SelColor+1][0], nil)
		}
	}

	for i, g := range glyphs {
		// Selection drawing
		if ssel != 0 {
			if i+fr.Top+n >= fr.Sel.E {
				ssel = 0
			}
		} else {
			if found, nssel := fr.getSsel(i + fr.Top + n); found {
				ssel = nssel
			}
		}

		onpmatch := (fr.PMatch.S != fr.PMatch.E) && (i+fr.Top+n == fr.PMatch.S) && (len(fr.Colors) > 4) && (ssel == 0)

		// Softwrap mark drawing
		if (g.p.Y != cury) && ((fr.Hackflags & HF_MARKSOFTWRAP) != 0) {
			midline := util.FixedToInt(cury) - util.FixedToInt((fr.glyphBounds.Max.Y+fr.glyphBounds.Min.Y))/2
			if !newline {
				r := image.Rectangle{
					image.Point{util.FixedToInt(fr.rightMargin), midline},
					image.Point{util.FixedToInt(fr.rightMargin + fr.margin), midline + 1}}
				draw.Draw(fr.B, fr.R.Intersect(r), &fr.Colors[0][1], fr.R.Intersect(r).Min, draw.Src)
			}

			cury = g.p.Y

			midline = util.FixedToInt(cury) - util.FixedToInt((fr.glyphBounds.Max.Y+fr.glyphBounds.Min.Y))/2

			if !newline {
				r := image.Rectangle{
					image.Point{util.FixedToInt(fr.leftMargin - fr.margin), midline},
					image.Point{util.FixedToInt(fr.leftMargin), midline + 1}}
				draw.Draw(fr.B, fr.R.Intersect(r), &fr.Colors[0][1], fr.R.Intersect(r).Min, draw.Src)
			}
		}
		newline = (g.r == '\n')

		f := fr.Font
		if g.subfont.idx != 0 {
			f = fr.Font2
		}

		// Glyph drawing
		_, mask, gr, _ := f.Glyph(g.subfont.fontIdx, g.index, g.p)
		dr := fr.R.Intersect(gr)
		if !dr.Empty() {
			mp := image.Point{dr.Min.X - gr.Min.X, dr.Min.Y - gr.Min.Y}
			color := &fr.Colors[1][1]
			if (ssel >= 0) && (ssel < len(fr.Colors)) && (g.color >= 0) && (int(g.color) < len(fr.Colors[ssel])) {
				color = &fr.Colors[ssel][g.color]
			}
			if onpmatch && len(fr.Colors) > 4 {
				color = &fr.Colors[4][g.color]
			}
			draw.DrawMask(fr.B, dr, color, dr.Min, mask, mp, draw.Over)
		}
	}
}

func (fr *Frame) drawSingleGlyph(g *glyph, ssel int) {
	f := fr.Font
	if g.subfont.idx != 0 {
		f = fr.Font2
	}
	_, mask, gr, _ := f.Glyph(g.subfont.fontIdx, g.index, g.p)
	// Glyph drawing
	dr := fr.R.Intersect(gr)
	if !dr.Empty() {
		mp := image.Point{dr.Min.X - gr.Min.X, dr.Min.Y - gr.Min.Y}
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
		draw.DrawMask(fr.B, dr, bgcolor, dr.Min, &fr.scrubGlyph, mp, draw.Over)

		// Redraw glyph
		draw.DrawMask(fr.B, dr, color, dr.Min, mask, mp, draw.Over)
	}
}

func (fr *Frame) scrollDir(recalcPos image.Point) int {
	if (recalcPos.X < fr.R.Min.X) || (recalcPos.X > fr.R.Max.X) {
		return 0
	}

	mid := (fr.R.Min.Y + fr.R.Max.Y) / 2

	if recalcPos.Y < mid {
		return -1
	}

	if recalcPos.Y > mid {
		return +1
	}

	return 0
}

func (f *Frame) OnClick(e util.MouseDownEvent, events <-chan interface{}) *mouse.Event {
	if e.Which == mouse.ButtonWheelUp {
		f.Scroll(-1, 1)
		return nil
	}

	if e.Which == mouse.ButtonWheelDown {
		f.Scroll(+1, 1)
		return nil
	}

	p := f.CoordToPoint(e.Where)

	sel := int(math.Log2(float64(e.Which)))
	if sel >= len(f.Colors) {
		return nil
	}

	if p >= 0 {
		if (sel == 0) && (e.Count == 1) && (e.Modifiers&key.ModShift != 0) {
			// shift-click extends selection, but only for the first selection
			if p < f.Sel.S {
				f.setSelectEx(sel, e.Count, p, f.Sel.E)
			} else {
				f.setSelectEx(sel, e.Count, f.Sel.S, p)
			}
		} else {
			if sel != 0 && f.Sel.S != f.Sel.E && (p >= f.Sel.S-1) && (p <= f.Sel.E+1) {
				f.SelColor = sel
			} else {
				f.setSelectEx(sel, e.Count, p, p)
			}
		}
		f.Redraw(true, nil)
		ee := f.Select(sel, e.Count, e.Which, events)
		f.Redraw(true, nil)
		return ee
	}

	return nil
}

func (fr *Frame) LineNo() int {
	return int(float32(fr.R.Max.Y-fr.R.Min.Y) / float32(fr.Font.LineHeight()))
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
	y := fixed.I(0)
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

	off := -1
	for i := range fr.glyphs {
		fr.glyphs[i].p.Y -= fixed.Int26_6(ln) * lh
		if (off < 0) && (fr.glyphs[i].p.Y >= fr.ins.Y) {
			off = i
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
		h := ln * util.FixedToInt(lh)

		for fr.redrawOpt.scrollStart = len(fr.glyphs) - 1; fr.redrawOpt.scrollStart > 0; fr.redrawOpt.scrollStart-- {
			g := fr.glyphs[fr.redrawOpt.scrollStart]
			if util.FixedToInt(g.p.Y+lh) < (fr.R.Max.Y - h) {
				break
			}
		}
		fr.redrawOpt.scrollEnd = -1

		p := fr.R.Min
		p.Y += h
		r := fr.R
		r.Max.Y -= h
		draw.Draw(fr.B, r, fr.B, p, draw.Src)

		r = fr.R
		if (fr.redrawOpt.scrollStart >= 0) && (fr.redrawOpt.scrollStart < len(fr.glyphs)) {
			bounds := fr.Font.Bounds()
			r.Min.Y = util.FixedToInt(fr.glyphs[fr.redrawOpt.scrollStart].p.Y - bounds.Min.Y)
		} else {
			r.Min.Y = fr.R.Max.Y - h
		}
		draw.Draw(fr.B, r.Intersect(fr.R), &fr.Colors[0][0], r.Intersect(fr.R).Min, draw.Src)
	}

	return len(fr.glyphs)
}

func (fr *Frame) PushDown(ln int, a, b []ColorRune) {
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
		fr.redrawOpt.scrollStart = 0
		fr.redrawOpt.scrollEnd = len(fr.glyphs)

		h := len(fr.PhisicalLines()) * util.FixedToInt(lh)
		r := fr.R
		r.Min.Y += h
		draw.Draw(fr.B, r, fr.B, fr.R.Min, draw.Src)

		r = fr.R
		r.Max.Y = r.Min.Y + h
		r = r.Intersect(fr.R)
		draw.Draw(fr.B, r, &fr.Colors[0][0], r.Min, draw.Src)
	}

	fr.leftMargin = fixed.I(fr.R.Min.X) + fr.margin
	bottom := fixed.I(fr.R.Max.Y) + lh

	if fr.ins.X != fr.leftMargin {
		fr.ins.X = fr.leftMargin
		fr.ins.Y += lh
	}

	oldY := fixed.I(0)
	if len(oldglyphs) > 0 {
		oldY = oldglyphs[0].p.Y
	}

	for i := range oldglyphs {
		if fr.ins.Y > bottom {
			return
		}

		if fr.ins.Y < fixed.I(fr.R.Max.Y) {
			fr.lastFull = len(fr.glyphs)
		}

		if oldglyphs[i].p.Y != oldY {
			fr.ins.Y += lh
			oldY = oldglyphs[i].p.Y
		}

		oldglyphs[i].p.Y = fr.ins.Y
		fr.ins.X = oldglyphs[i].p.X

		fr.glyphs = append(fr.glyphs, oldglyphs[i])
	}

	if fr.ins.Y < fixed.I(fr.R.Max.Y) {
		fr.lastFull = len(fr.glyphs)
	}

	return
}

func (fr *Frame) Size() int {
	return len(fr.glyphs)
}

func (fr *Frame) LimitY() int {
	p := fr.PointToCoord(fr.Top + len(fr.glyphs) - 1)
	gb := fr.Font.Bounds()
	return p.Y - util.FixedToInt(gb.Min.Y)
}
