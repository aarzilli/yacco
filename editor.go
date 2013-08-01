package main

import (
	"fmt"
	"github.com/skelterjohn/go.wde"
	"image"
	"image/draw"
	"strconv"
	"yacco/buf"
	"yacco/config"
	"yacco/textframe"
	"yacco/util"
)

type Editor struct {
	r       image.Rectangle
	rhandle image.Rectangle
	frac    float64

	sfr   textframe.ScrollFrame
	tagfr textframe.Frame

	bodybuf     *buf.Buffer
	top         int
	tagbuf      *buf.Buffer
	confirmDel  bool
	confirmSave bool

	eventChan chan string

	specialTag          string
	savedTag            string
	specialChan         chan string
	specialExitOnReturn bool

	restoredJump int
	jumpCount    int
}

const SCROLL_WIDTH = 10
const NUM_JUMPS = 7
const JUMP_THRESHOLD = 100

func scrollfn(e *Editor, sd int, sl int) {
	if sd < 0 {
		for i := 0; i < sl; i++ {
			e.top = e.bodybuf.Tonl(e.top-2, -1)
		}
	} else if sd > 0 {
		for i := 0; i < sl; i++ {
			e.top = e.bodybuf.Tonl(e.top, +1)
		}
	} else if sd == 0 {
		e.top = e.bodybuf.Tonl(sl, -1)
	}

	sz := e.bodybuf.Size()

	e.bodybuf.Rdlock()
	defer e.bodybuf.Rdunlock()
	a, b := e.bodybuf.Selection(util.Sel{e.top, sz})

	e.bodybuf.Highlight(-1, false, e.top)

	e.sfr.Set(e.top, sz)
	e.sfr.Fr.Clear()
	e.sfr.Fr.InsertColor(a)
	e.sfr.Fr.InsertColor(b)
	e.sfr.Redraw(true)
}

func (e *Editor) SetWnd(wnd wde.Window) {
	e.sfr.Wnd = wnd
	e.sfr.Fr.Wnd = wnd
	e.tagfr.Wnd = wnd
}

func NewEditor(bodybuf *buf.Buffer, addBuffer bool) *Editor {
	e := &Editor{}

	e.confirmDel = false

	e.bodybuf = bodybuf
	e.tagbuf, _ = buf.NewBuffer(bodybuf.Dir, "+Tag", true, Wnd.Prop["indentchar"])

	if addBuffer {
		bufferAdd(bodybuf)
	}

	e.sfr = textframe.ScrollFrame{
		Width: SCROLL_WIDTH,
		Color: config.TheColorScheme.Scrollbar,
		Fr: textframe.Frame{
			Font:        config.MainFont,
			Hackflags:   textframe.HF_MARKSOFTWRAP | textframe.HF_QUOTEHACK | textframe.HF_ELASTICTABS,
			Scroll:      func(sd, sl int) { scrollfn(e, sd, sl) },
			VisibleTick: false,
			Colors: [][]image.Uniform{
				config.TheColorScheme.EditorPlain,
				config.TheColorScheme.EditorSel1,                // 0 first button selection
				config.TheColorScheme.EditorSel2,                // 1 second button selection
				config.TheColorScheme.EditorSel3,                // 2 third button selection
				config.TheColorScheme.EditorPlain,               // 3 highlighted parenthesis?
				config.TheColorScheme.EditorPlain,               // 4 content of 'addr' file
				config.TheColorScheme.EditorMatchingParenthesis, // 5 matching parenthesis
				/* space for jumps - 6 through 12 */
				config.TheColorScheme.EditorPlain,
				config.TheColorScheme.EditorPlain,
				config.TheColorScheme.EditorPlain,
				config.TheColorScheme.EditorPlain,
				config.TheColorScheme.EditorPlain,
				config.TheColorScheme.EditorPlain,
				config.TheColorScheme.EditorPlain},
		},
	}
	e.tagfr = textframe.Frame{
		Font:        config.TagFont,
		Hackflags:   textframe.HF_MARKSOFTWRAP | textframe.HF_QUOTEHACK,
		Scroll:      func(sd, sl int) {},
		VisibleTick: false,
		Colors: [][]image.Uniform{
			config.TheColorScheme.TagPlain,
			config.TheColorScheme.TagSel1,
			config.TheColorScheme.TagSel2,
			config.TheColorScheme.TagSel3,
			config.TheColorScheme.TagPlain,
			config.TheColorScheme.TagMatchingParenthesis},
	}
	e.top = 0

	util.Must(e.sfr.Init(5), "Editor initialization failed")
	util.Must(e.tagfr.Init(5), "Editor initialization failed")

	e.GenTag()
	e.tagfr.Sels[0].S = e.tagbuf.Size()
	e.tagfr.Sels[0].E = e.tagbuf.Size()

	e.sfr.Set(0, e.bodybuf.Size())

	return e
}

func (e *Editor) SetRects(b draw.Image, r image.Rectangle) {
	e.r = r
	sfrr := r
	sfrr.Min.Y = sfrr.Min.Y + TagHeight(&e.tagfr) + 3
	sfrr.Max.X -= 2
	//sfrr.Max.Y -= 1
	e.sfr.SetRects(b, sfrr)

	e.bodybuf.DisplayLines = int(float64(sfrr.Max.Y-sfrr.Min.Y) / float64(e.sfr.Fr.Font.LineHeight()))

	e.sfr.Fr.Clear()
	ba, bb := e.bodybuf.Selection(util.Sel{e.top, e.bodybuf.Size()})
	e.sfr.Fr.InsertColor(ba)
	e.sfr.Fr.InsertColor(bb)

	e.tagfr.R = r
	e.tagfr.R.Min.Y += 2
	e.tagfr.R.Min.X += SCROLL_WIDTH
	e.tagfr.R.Max.X -= 2
	e.tagfr.R.Max.Y = e.tagfr.R.Min.Y + TagHeight(&e.tagfr)
	e.tagfr.R = e.r.Intersect(e.tagfr.R)
	e.tagfr.B = b
	e.tagfr.Clear()
	ta, tb := e.tagbuf.Selection(util.Sel{0, e.tagbuf.Size()})
	e.tagfr.InsertColor(ta)
	e.tagfr.InsertColor(tb)

	e.rhandle = r
	e.rhandle.Min.Y++
	e.rhandle.Max.X = e.rhandle.Min.X + SCROLL_WIDTH
	e.rhandle.Max.Y = e.tagfr.R.Max.Y
	e.rhandle = e.r.Intersect(e.rhandle)

	e.bodybuf.Highlight(-1, false, e.top)
}

func (e *Editor) MinHeight() int {
	return TagHeight(&e.tagfr) + 2
}

func (e *Editor) Redraw() {
	drawingFuncs := textframe.GetOptimizedDrawing(e.sfr.Fr.B)

	// draw resize handle
	drawingFuncs.DrawFillSrc(e.sfr.Fr.B, e.rhandle, &config.TheColorScheme.HandleBG)

	hir := e.rhandle
	hir.Min.X += 2
	hir.Max.X -= 2
	hir.Min.Y += 2
	hir.Max.Y -= 1
	var rhc *image.Uniform
	if e.specialChan != nil {
		rhc = &config.TheColorScheme.HandleSpecialFG
	} else {
		if e.bodybuf.Modified {
			rhc = &config.TheColorScheme.HandleModifiedFG
		} else {
			rhc = &config.TheColorScheme.HandleFG
		}
	}
	drawingFuncs.DrawFillSrc(e.sfr.Fr.B, hir, rhc)

	// draw text frames
	e.tagfr.Redraw(false)
	e.sfr.Redraw(false)

	// draw two-pixel border
	border := e.r
	border.Max.Y = border.Min.Y + 2
	drawingFuncs.DrawFillSrc(e.sfr.Fr.B, e.r.Intersect(border), &config.TheColorScheme.Border)

	border = e.r
	border.Min.X = border.Max.X - 2
	drawingFuncs.DrawFillSrc(e.sfr.Fr.B, e.r.Intersect(border), &config.TheColorScheme.Border)

	// draw one-pixel tag border
	border = e.r
	border.Max.X -= 2
	border.Min.Y = e.tagfr.R.Max.Y
	border.Max.Y = border.Min.Y + 1
	drawingFuncs.DrawFillSrc(e.sfr.Fr.B, e.r.Intersect(border), &config.TheColorScheme.HandleBG)
}

func (e *Editor) GenTag() {
	usertext := ""
	if e.tagbuf.EditableStart >= 0 {
		usertext = string(e.tagbuf.SelectionRunes(util.Sel{e.tagbuf.EditableStart, e.tagbuf.Size()}))
	}

	t := e.bodybuf.ShortName()

	if e.sfr.Fr.Sels[0].E <= 10000 {
		line, col := e.bodybuf.GetLine(e.sfr.Fr.Sels[0].E)
		//t += fmt.Sprintf(":%d:%d#%d", line, col, e.sfr.Fr.Sels[0].E)
		_ = col
		t += fmt.Sprintf(":%d", line)
	}

	if e.specialChan == nil {
		t += config.DefaultEditorTag
		if e.bodybuf.Modified {
			t += " Put"
		}

		if e.bodybuf.HasUndo() {
			t += " Undo"
		}
		if e.bodybuf.HasRedo() {
			t += " Redo"
		}
	} else {
		t += e.specialTag
	}

	t += " | " + usertext
	start := e.tagfr.Sels[0].S - e.tagbuf.EditableStart
	end := e.tagfr.Sels[0].E - e.tagbuf.EditableStart
	e.tagbuf.EditableStart = -1
	e.tagbuf.Replace([]rune(t), &util.Sel{0, e.tagbuf.Size()}, e.tagfr.Sels, true, nil, 0, false)
	TagSetEditableStart(e.tagbuf)
	e.tagfr.Sels[0].S = start + e.tagbuf.EditableStart
	e.tagfr.Sels[0].E = end + e.tagbuf.EditableStart
	e.tagbuf.FixSel(&e.tagfr.Sels[0])
}

func (e *Editor) refreshIntl() {
	e.sfr.Fr.Clear()
	e.sfr.Set(e.top, e.bodybuf.Size())

	e.bodybuf.Rdlock()
	defer e.bodybuf.Rdunlock()
	ba, bb := e.bodybuf.Selection(util.Sel{e.top, e.bodybuf.Size()})
	e.sfr.Fr.InsertColor(ba)
	e.sfr.Fr.InsertColor(bb)
}

func (e *Editor) BufferRefreshEx(ontag bool, recur bool) {
	match := findPMatch(e.tagbuf, e.tagfr.Sels[0])
	if match.S >= 0 {
		e.tagfr.Sels[5] = match
	} else {
		e.tagfr.Sels[5].S = e.tagfr.Sels[5].E
	}
	match = findPMatch(e.bodybuf, e.sfr.Fr.Sels[0])
	if match.S >= 0 {
		e.sfr.Fr.Sels[5] = match
	} else {
		e.sfr.Fr.Sels[5].S = e.sfr.Fr.Sels[5].E
	}

	if ontag {
		e.tagfr.Clear()
		ta, tb := e.tagbuf.Selection(util.Sel{0, e.tagbuf.Size()})
		e.tagfr.InsertColor(ta)
		e.tagfr.InsertColor(tb)
		e.tagfr.Redraw(true)
	} else {
		e.refreshIntl()
		if e.recenterIntl(false) {
			e.refreshIntl()
		}

		e.GenTag()
		e.tagfr.Clear()
		ta, tb := e.tagbuf.Selection(util.Sel{0, e.tagbuf.Size()})
		e.tagfr.InsertColor(ta)
		e.tagfr.InsertColor(tb)

		e.Redraw()
		e.sfr.Wnd.FlushImage()

		if (e.bodybuf.RefCount <= 1) || !recur {
			return
		}

		for _, col := range Wnd.cols.cols {
			for _, oe := range col.editors {
				if oe.bodybuf == e.bodybuf {
					oe.BufferRefreshEx(false, false)
				}
			}
		}
	}
}

func (e *Editor) BufferRefresh(ontag bool) {
	e.BufferRefreshEx(ontag, true)
}

func findPMatch(b *buf.Buffer, sel0 util.Sel) util.Sel {
	if sel0.S != sel0.E {
		return util.Sel{-1, -1}
	}

	match := b.Topmatch(sel0.S, +1)
	if match >= 0 {
		return util.Sel{match, match + 1}
	}

	if sel0.S > 0 {
		match = b.Topmatch(sel0.S-1, -1)
		if match >= 0 {
			return util.Sel{match, match + 1}
		}
	}

	return util.Sel{-1, -1}
}

func (ed *Editor) Column() *Col {
	for _, col := range Wnd.cols.cols {
		for _, ce := range col.editors {
			if ce == ed {
				return col
			}
		}
	}

	return nil
}

func (ed *Editor) Height() int {
	return ed.r.Max.Y - ed.r.Min.Y
}

func (e *Editor) UsedHeight() int {
	return e.sfr.Fr.Limit.Y - e.r.Min.Y + int(e.sfr.Fr.Font.LineHeight())
}

func (ed *Editor) recenterIntl(refresh bool) bool {
	if ed.sfr.Fr.Inside(ed.sfr.Fr.Sels[0].E) {
		return false
	}
	n := ed.sfr.Fr.LineNo() / 2
	x := ed.sfr.Fr.Sels[0].E
	for i := 0; i < n; i++ {
		x = ed.bodybuf.Tonl(x-2, -1)
	}
	ed.top = x
	if refresh {
		ed.BufferRefresh(false)
	}
	ed.bodybuf.Highlight(-1, false, ed.top)
	return true
}

func (ed *Editor) Recenter() bool {
	return ed.recenterIntl(true)
}

func (ed *Editor) Warp() {
	if !HasFocus {
		return
	}
	p := ed.sfr.Fr.PointToCoord(ed.sfr.Fr.Sels[0].S)
	ed.sfr.Fr.VisibleTick = true
	Wnd.wnd.WarpMouse(p)
}

func (ed *Editor) EnterSpecial(specialChan chan string, specialTag string, exitOnReturn bool) bool {
	if ed.specialChan != nil {
		return false
	}
	ed.specialChan = specialChan
	ed.specialTag = specialTag
	ed.specialExitOnReturn = exitOnReturn
	ed.savedTag = string(ed.tagbuf.SelectionRunes(util.Sel{ed.tagbuf.EditableStart, ed.tagbuf.Size()}))
	ed.tagbuf.Replace([]rune{}, &util.Sel{ed.tagbuf.EditableStart, ed.tagbuf.Size()}, ed.tagfr.Sels, true, nil, 0, false)
	ed.BufferRefresh(false)
	return true
}

func (ed *Editor) ExitSpecial() {
	close(ed.specialChan)
	ed.specialChan = nil
	ed.specialTag = ""
	ed.tagbuf.Replace([]rune(ed.savedTag), &util.Sel{ed.tagbuf.EditableStart, ed.tagbuf.Size()}, ed.tagfr.Sels, true, nil, 0, false)
	ed.BufferRefresh(false)
}

func (ed *Editor) PropTrigger() {
	tabWidth, err := strconv.Atoi(ed.bodybuf.Props["tab"])
	if err != nil {
		ed.sfr.Fr.TabWidth = tabWidth
	}
	oldFont := ed.sfr.Fr.Font
	if ed.bodybuf.Props["font"] == "alt" {
		ed.sfr.Fr.Font = config.AltFont
	} else {
		ed.sfr.Fr.Font = config.MainFont
	}
	if oldFont != ed.sfr.Fr.Font {
		ed.sfr.Fr.ReinitFont()
	}

	ed.BufferRefresh(false)
}

func (ed *Editor) Dump() DumpEditor {
	fontName := ""
	switch ed.sfr.Fr.Font {
	default:
		fallthrough
	case config.MainFont:
		fontName = "main"
	case config.AltFont:
		fontName = "alt"
	}

	return DumpEditor{
		bufferIndex(ed.bodybuf),
		ed.frac,
		fontName,
		ed.specialChan != nil,
		string(ed.tagbuf.SelectionRunes(util.Sel{ed.tagbuf.EditableStart, ed.tagbuf.Size()})),
	}
}

func (ed *Editor) PushJump() {
	jb := len(ed.sfr.Fr.Sels) - NUM_JUMPS
	for i := len(ed.sfr.Fr.Sels) - 2; i >= jb; i-- {
		ed.sfr.Fr.Sels[i+1] = ed.sfr.Fr.Sels[i]
	}
	ed.sfr.Fr.Sels[jb].S = ed.sfr.Fr.Sels[0].S
	ed.restoredJump = 0
	ed.jumpCount++
}

func (ed *Editor) RestoreJump() {
	if ed.jumpCount == 0 {
		return
	}

	jb := len(ed.sfr.Fr.Sels) - NUM_JUMPS

	// if we haven't recently restored a jump since the last push, refer to the last pushed jump
	if ed.restoredJump < jb {
		ed.restoredJump = jb
	}

	// if we moved since the last restored (or pushed jump)
	if ed.sfr.Fr.Sels[0].S != ed.sfr.Fr.Sels[ed.restoredJump].S {
		// we push the current position, then restore the previously last jump done
		ed.PushJump()
		ed.sfr.Fr.Sels[0].S = ed.sfr.Fr.Sels[jb+1].S
		ed.sfr.Fr.Sels[0].E = ed.sfr.Fr.Sels[jb+1].S
		ed.restoredJump = jb + 1
		return
	}

	// we are on the last restored jump, cycle through jump
	ed.restoredJump++
	if (ed.restoredJump >= len(ed.sfr.Fr.Sels)) || (ed.restoredJump >= jb+ed.jumpCount) {
		ed.restoredJump = jb
	}

	ed.sfr.Fr.Sels[0].S = ed.sfr.Fr.Sels[ed.restoredJump].S
	ed.sfr.Fr.Sels[0].E = ed.sfr.Fr.Sels[ed.restoredJump].S
}

func (ed *Editor) LastJump() int {
	jb := len(ed.sfr.Fr.Sels) - NUM_JUMPS
	return ed.sfr.Fr.Sels[jb].S
}
