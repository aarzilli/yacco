package textframe

import (
	"image"
	"image/draw"
	"io/ioutil"
	"strings"
	"runtime"
	"time"
	"unicode"
	"math"
	"yacco/util"
	"github.com/skelterjohn/go.wde"
	"code.google.com/p/freetype-go/freetype"
	"code.google.com/p/freetype-go/freetype/raster"
	"code.google.com/p/freetype-go/freetype/truetype"
)

type Redrawable interface {
	Redraw(flush bool)
}

type Font struct {
	fonts   []*truetype.Font
	dpi     float64
	size    float64
	Spacing float64
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
)

type Frame struct {
	Font        *Font
	Hackflags   uint32
	B           draw.Image      // the image the text will be drawn upon
	R           image.Rectangle // the rectangle occupied by the frame
	VisibleTick bool
	Colors      [][]image.Uniform
	TabWidth    int
	Wnd wde.Window
	Scroll FrameScrollFn
	Top int

	margin raster.Fix32

	Sels []util.Sel

	cs      []*freetype.Context
	glyphs  []glyph
	ins     raster.Point
	atStart bool

	dblclickp int
	dblclickc int
	dblclickbtn wde.Button
	dblclickt time.Time
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
	widthy raster.Fix32
	p         raster.Point
	color     uint8
}

// Reads a Font: fontPath is a ':' separated list of ttf font files (they will be used to search characters)
func NewFont(dpi, size, lineSpacing float64, fontPath string) (*Font, error) {
	fontPathV := strings.Split(fontPath, ":")
	rf := &Font{make([]*truetype.Font, 0, len(fontPathV)), dpi, size, lineSpacing}
	for _, fontfile := range fontPathV {
		fontBytes, err := ioutil.ReadFile(fontfile)
		if err != nil {
			return nil, err
		}
		parsedfont, err := freetype.ParseFont(fontBytes)
		if err != nil {
			return nil, err
		}
		rf.fonts = append(rf.fonts, parsedfont)
	}
	return rf, nil
}

func (f *Font) LineHeight() int32 {
	bounds := f.fonts[0].Bounds(int32(f.size))
	return bounds.YMax - bounds.YMin
}

func (f *Font) Bounds() truetype.Bounds {
	return f.fonts[0].Bounds(int32(f.size))
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
		return FrameErrorNotEnoughColorLines
	}

	for i, _ := range fr.Colors {
		if len(fr.Colors[i]) < 2 {
			return FrameErrorNotEnoughColors(i)
		}
	}

	// create font contexts

	fr.cs = make([]*freetype.Context, len(fr.Font.fonts))
	for i, _ := range fr.Font.fonts {
		fr.cs[i] = freetype.NewContext()
		fr.cs[i].SetDPI(fr.Font.dpi)
		fr.cs[i].SetFont(fr.Font.fonts[i])
		fr.cs[i].SetFontSize(fr.Font.size)
		fr.cs[i].SetClip(fr.R)
	}

	fr.Clear()

	fr.dblclickbtn = wde.LeftButton
	fr.dblclickp = -1
	fr.dblclickc = 0
	fr.dblclickt = time.Now()

	return nil
}

func (fr *Frame) lineHeight() raster.Fix32 {
	lh := fr.Font.LineHeight()
	return fr.cs[0].PointToFix32(float64(lh) * fr.Font.Spacing)
}

func (fr *Frame) Clear() {
	gb := fr.Font.Bounds()
	fr.ins = raster.Point{raster.Fix32(fr.R.Min.X<<8) + fr.margin, raster.Fix32(fr.R.Min.Y << 8) + raster.Fix32(gb.YMax << 8)}
	fr.glyphs = fr.glyphs[:0]
	fr.atStart = true
}

// Inserts text into the frame, returns number of inserted runes
func (fr *Frame) Insert(runes []rune) int {
	cr := make([]ColorRune, len(runes))
	for i, _ := range runes {
		cr[i].C = 1
		cr[i].R = runes[i]
	}
	return fr.InsertColor(cr)
}

// Inserts text into the frame, returns the number of inserted runes
func (fr *Frame) InsertColor(runes []ColorRune) int {
	lh := fr.lineHeight()

	_, spaceIndex := fr.getIndex(' ')

	p := fr.ins
	prev, prevFontIdx, hasPrev := truetype.Index(0), 0, false

	rightMargin := raster.Fix32(fr.R.Max.X<<8) - fr.margin
	leftMargin := raster.Fix32(fr.R.Min.X<<8) + fr.margin
	bottom := raster.Fix32(fr.R.Max.Y<<8) + lh

	_, xIndex := fr.getIndex('x')
	spaceWidth := raster.Fix32(fr.Font.fonts[0].HMetric(fr.cs[0].Scale, spaceIndex).AdvanceWidth) << 2
	bigSpaceWidth := raster.Fix32(fr.Font.fonts[0].HMetric(fr.cs[0].Scale, xIndex).AdvanceWidth) << 2
	tabWidth := bigSpaceWidth * raster.Fix32(fr.TabWidth)

	for i, crune := range runes {
		if p.Y > bottom {
			return i
		}

		if crune.R == '\n' {
			g := glyph{
				r:         crune.R,
				fontIndex: 0,
				index:     spaceIndex,
				p:         p,
				color:     crune.C,
				width:     raster.Fix32(fr.R.Max.X<<8) - p.X - fr.margin,
				widthy: lh }

			fr.glyphs = append(fr.glyphs, g)

			p.X = leftMargin
			p.Y += lh
			prev, prevFontIdx, hasPrev = spaceIndex, 0, true
		} else if crune.R == '\t' {
			toNextCell := tabWidth - ((p.X - leftMargin) % tabWidth)
			if toNextCell <= spaceWidth/2 {
				toNextCell += tabWidth
			}

			g := glyph{
				r:         crune.R,
				fontIndex: 0,
				index:     spaceIndex,
				p:         p,
				color:     crune.C,
				width:     toNextCell}

			fr.glyphs = append(fr.glyphs, g)

			p.X += toNextCell
			prev, prevFontIdx, hasPrev = spaceIndex, 0, true
		} else if (crune.R == ' ') && (fr.Hackflags&HF_BOLSPACES != 0) {
			width := raster.Fix32(0)
			if i == 0 {
				if fr.atStart {
					width = bigSpaceWidth
				} else {
					width = spaceWidth
				}
			} else {
				switch fr.glyphs[i-1].r {
				case '\t':
					fallthrough
				case '\n':
					width = bigSpaceWidth
				case ' ':
					width = fr.glyphs[i-1].width
				default:
					width = spaceWidth
				}
			}

			g := glyph{
				r:         crune.R,
				fontIndex: 0,
				index:     spaceIndex,
				p:         p,
				color:     crune.C,
				width:     width}

			fr.glyphs = append(fr.glyphs, g)
			p.X += width
			prev, prevFontIdx, hasPrev = spaceIndex, 0, true
		} else {
			lur := crune.R

			if (fr.Hackflags & HF_QUOTEHACK) != 0 {
				if lur == '`' {
					lur = 0x2018;
				} else if lur == '\'' {
					lur = 0x2019;
				}
			}

			fontIdx, index := fr.getIndex(lur)
			if hasPrev && (fontIdx == prevFontIdx) {
				p.X += raster.Fix32(fr.Font.fonts[fontIdx].Kerning(fr.cs[fontIdx].Scale, prev, index)) << 2
			}

			width := raster.Fix32(fr.Font.fonts[fontIdx].HMetric(fr.cs[fontIdx].Scale, index).AdvanceWidth) << 2

			if fr.Hackflags&HF_TRUNCATE == 0 {
				if p.X+width > rightMargin {
					p.X = leftMargin
					p.Y += lh
				}
			}

			g := glyph{
				r:         crune.R,
				fontIndex: fontIdx,
				index:     index,
				p:         p,
				color:     crune.C,
				width:     width}

			fr.glyphs = append(fr.glyphs, g)

			p.X += width
			prev, prevFontIdx, hasPrev = index, fontIdx, true
		}

		fr.atStart = false
	}
	fr.ins = p
	return len(runes)
}

// Tracks the mouse position, selecting text, the events channel is from go.wde
// kind is 1 for character by character selection, 2 for word by word selection, 3 for line by line selection
func (fr *Frame) Select(idx, kind int, button wde.Button, events <-chan interface{}) {
	fix := fr.Sels[idx].S
	var autoscrollTicker *time.Ticker
	var autoscrollChan <-chan time.Time

	var lastPos image.Point

	for {
		runtime.Gosched()
		select {
		case ei := <- events:
			switch e := ei.(type) {
			case wde.MouseDraggedEvent:
				lastPos  = e.Where
				if e.Where.In(fr.R) {
					if autoscrollTicker != nil {
						autoscrollTicker.Stop()
						autoscrollTicker = nil
					}

					p := fr.CoordToPoint(e.Where)
					fr.SetSelect(idx, kind, fix, p)
					fr.Redraw(true)
				} else {
					if autoscrollTicker == nil {
						autoscrollTicker = time.NewTicker(100 * time.Millisecond)
						autoscrollChan = autoscrollTicker.C
					}
				}

			case wde.MouseUpEvent:
				if e.Which == button {
					return
				}
			}

		case <- autoscrollChan:
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
	for i := range fr.Sels {
		if i != idx {
			fr.Sels[i].E = fr.Sels[i].S
		}
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
		first := true
		for s = start-fr.Top; s >= 0; s-- {
			g := fr.glyphs[s].r
			if !(unicode.IsLetter(g) || unicode.IsDigit(g) || (g == '_')) {
				if !first { s++ }
				break
			}
			first = false
		}
		if s < 0 {
			s = 0
		}

		first = true
		for e = end-fr.Top; e < len(fr.glyphs); e++ {
			g := fr.glyphs[e].r
			if !(unicode.IsLetter(g) || unicode.IsDigit(g) || (g == '_')) {
				if first { e++ }
				break
			}
			first = false
		}

		start = s + fr.Top
		end = e + fr.Top

	case 3:
		var s, e int
		for s = start-1-fr.Top; s > 0; s-- {
			if fr.glyphs[s].r == '\n' {
				s++
				break
			}
		}

		if s < 0 {
			s = 0
		}

		for e = end-fr.Top; e < len(fr.glyphs); e++ {
			if fr.glyphs[e].r == '\n' {
				e++
				break
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
	glyphBounds := fr.Font.fonts[0].Bounds(int32(fr.Font.size))

	for i, g := range fr.glyphs {
		if g.p.Y - raster.Fix32(glyphBounds.YMin << 8) < ftcoord.Y {
			continue
		} else if (g.p.Y - lh) > ftcoord.Y {
			return i + fr.Top
		} else if ftcoord.X < g.p.X {
			return i + fr.Top
		} else if g.r == '\n' {
			return i + fr.Top
		} else if (ftcoord.X >= g.p.X) && (ftcoord.X <= g.p.X + g.width) {
			return i + fr.Top
		}
	}

	return fr.Top + len(fr.glyphs)
}

func (fr *Frame) redrawSelection(s, e int, color *image.Uniform) {
	if s < 0 {
		s = 0
	}
	glyphBounds := fr.Font.fonts[0].Bounds(int32(fr.Font.size))
	rightMargin := raster.Fix32(fr.R.Max.X<<8) - fr.margin
	leftMargin := raster.Fix32(fr.R.Min.X<<8) + fr.margin
	drawingFuncs := GetOptimizedDrawing(fr.B)

	var sp, ep, sep image.Point

	ss := fr.glyphs[s]
	sp = image.Point{ int(ss.p.X >> 8), int((ss.p.Y) >> 8) - int(glyphBounds.YMax) }

	var se glyph

	if e < len(fr.glyphs) {
		se = fr.glyphs[e]
		sep = image.Point{ int(leftMargin >> 8), int((se.p.Y) >> 8) - int(glyphBounds.YMax) }
		ep = image.Point{ int(se.p.X >> 8), int((se.p.Y) >> 8) - int(glyphBounds.YMin) }
	} else if e == len(fr.glyphs) {
		se = fr.glyphs[len(fr.glyphs)-1]
		sep = image.Point{ int(leftMargin >> 8), int((se.p.Y) >> 8) - int(glyphBounds.YMax) }
		ep = image.Point{ int((se.p.X + se.width) >> 8), int((se.p.Y) >> 8) - int(glyphBounds.YMin) }
	} else {
		se = fr.glyphs[len(fr.glyphs)-1]
		sep = image.Point{ int(leftMargin >> 8), int((se.p.Y) >> 8) - int(glyphBounds.YMax) }
		ep = image.Point{ int(rightMargin >> 8), fr.R.Max.Y }
	}

	if ss.p.Y == se.p.Y {
		r := image.Rectangle{ sp, ep }
		drawingFuncs.DrawFillSrc(fr.B, fr.R.Intersect(r), color)
	} else {
		rs := image.Rectangle{ sp, image.Point{ int(rightMargin >> 8), int((ss.p.Y) >> 8) - int(glyphBounds.YMin) } }
		re := image.Rectangle{ sep, ep }
		rb := image.Rectangle{
			image.Point{ sep.X, int((ss.p.Y) >> 8) - int(glyphBounds.YMin) },
			image.Point{ int(rightMargin >> 8), sep.Y },
		}
		drawingFuncs.DrawFillSrc(fr.B, fr.R.Intersect(rs), color)
		drawingFuncs.DrawFillSrc(fr.B, fr.R.Intersect(re), color)
		drawingFuncs.DrawFillSrc(fr.B, fr.R.Intersect(rb), color)
	}
}

func (fr *Frame) Redraw(flush bool) {
	glyphBounds := fr.Font.fonts[0].Bounds(int32(fr.Font.size))
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
				if (i+fr.Top >= fr.Sels[j].S) && (i+fr.Top < fr.Sels[j].E) {
					ssel = j + 1

					fr.redrawSelection(fr.Sels[j].S-fr.Top, fr.Sels[j].E-fr.Top, &fr.Colors[ssel][0])
				}
			}
		}

		// Softwrap mark drawing
		if (g.p.Y != cury) && ((fr.Hackflags & HF_MARKSOFTWRAP) != 0) {
			midline := int(cury >> 8) - int((glyphBounds.YMax + glyphBounds.YMin)/2)
			if !newline {
				r := image.Rectangle{
					image.Point{ int(rightMargin>>8), midline },
					image.Point{ int(rightMargin>>8) + int(fr.margin >> 8), midline+1 } }
				drawingFuncs.DrawFillSrc(fr.B, fr.R.Intersect(r), &fr.Colors[0][1])
			}

			cury = g.p.Y

			midline = int(cury >> 8) - int((glyphBounds.YMax + glyphBounds.YMin)/2)

			if !newline {
				r := image.Rectangle{
					image.Point{ int(leftMargin>>8) - int(fr.margin >> 8), midline },
					image.Point{ int(leftMargin>>8), midline+1 } }
				drawingFuncs.DrawFillSrc(fr.B, fr.R.Intersect(r), &fr.Colors[0][1])
			}
		}
		newline = (g.r == '\n')

		// Glyph drawing
		fr.cs[g.fontIndex].SetSrc(&fr.Colors[ssel][g.color])
		mask, offset, err := fr.cs[g.fontIndex].Glyph(g.index, g.p)
		if err != nil {
			panic(err)
		}
		glyphRect := mask.Bounds().Add(offset)
		dr := fr.R.Intersect(glyphRect)
		if !dr.Empty() {
			mp := image.Point{0, dr.Min.Y - glyphRect.Min.Y}
			drawingFuncs.DrawGlyphOver(fr.B, dr, &fr.Colors[ssel][g.color], mask, mp)
		}
	}

	// Tick drawing
	if (fr.Sels[0].S == fr.Sels[0].E) && fr.VisibleTick && (len(fr.glyphs) > 0) && (fr.Sels[0].S - fr.Top >= 0) && (fr.Sels[0].S - fr.Top <= len(fr.glyphs)) {
		var x, y int
		if fr.Sels[0].S - fr.Top < len(fr.glyphs) {
			g := fr.glyphs[fr.Sels[0].S - fr.Top]
			x = int(g.p.X >> 8)
			y = int(g.p.Y >> 8)
		} else {
			g := fr.glyphs[len(fr.glyphs)-1]

			if g.widthy > 0 {
				x = fr.R.Min.X + int(fr.margin >> 8)
				y = int((g.p.Y + g.widthy) >> 8)
			} else {
				x = int((g.p.X + g.width) >> 8) + 1
				y = int(g.p.Y >> 8)
			}

		}
		r := image.Rectangle{
			Min: image.Point{ x, y - int(glyphBounds.YMax) },
			Max: image.Point{ x+1, y - int(glyphBounds.YMin) } }

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
	for fontIdx, font = range fr.Font.fonts {
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

func (f *Frame) OnClick(e wde.MouseDownEvent, events <- chan interface{}) {

	if e.Which == wde.WheelUpButton {
		f.Scroll(-1, 1)
		f.Redraw(true)
		return
	}

	if e.Which == wde.WheelDownButton {
		f.Scroll(+1, 1)
		f.Redraw(true)
		return
	}

	p := f.CoordToPoint(e.Where)
	now := time.Now()

	if (e.Which == f.dblclickbtn) && (p == f.dblclickp) && (now.Sub(f.dblclickt) < time.Duration(200 * time.Millisecond)) {
		f.dblclickt = now
		f.dblclickc++
	} else {
		f.dblclickbtn = e.Which
		f.dblclickp = p
		f.dblclickt = now
		f.dblclickc = 1
	}

	if f.dblclickc > 3 {
		f.dblclickc = 1
	}

	sel := int(math.Log2(float64(e.Which)))
	if sel >= len(f.Sels) {
		return
	}

	if p >= 0 {
		f.SetSelect(sel, f.dblclickc, p, p)
		if f.dblclickc > 1 {
			f.Redraw(true)
		}
		f.Select(sel, f.dblclickc, e.Which, events)
		f.Redraw(true)
	}
}
