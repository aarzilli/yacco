package main

import (
	"fmt"
	"github.com/skelterjohn/go.wde"
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"yacco/buf"
	"yacco/config"
	"yacco/edutil"
	"yacco/textframe"
	"yacco/util"
)

type Editor struct {
	r       image.Rectangle
	btnr    image.Rectangle
	rhandle image.Rectangle
	frac    float64
	last    bool

	sfr   textframe.ScrollFrame
	tagfr textframe.Frame

	bodybuf     *buf.Buffer
	tagbuf      *buf.Buffer
	confirmDel  bool
	confirmSave bool

	eventChan   chan string
	eventReader util.EventReader

	specialTag          string
	savedTag            string
	specialChan         chan string
	specialExitOnReturn bool

	pw int

	otherSel     []util.Sel
	jumps        []util.Sel
	restoredJump int
	jumpCount    int
}

const SCROLL_WIDTH = 10
const NUM_JUMPS = 7
const JUMP_THRESHOLD = 100

const (
	OS_TOP = iota
	OS_ADDR
	OS_TIP
	NUM_OTHER_SEL
)

const PMATCHSEL = 3

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
	hf := textframe.HF_MARKSOFTWRAP
	if config.QuoteHack {
		hf |= textframe.HF_QUOTEHACK
	}

	e.sfr = textframe.ScrollFrame{
		Width: SCROLL_WIDTH,
		Color: config.TheColorScheme.Scrollbar,
		Fr: textframe.Frame{
			Font:            config.MainFont,
			Hackflags:       hf,
			Scroll:          nil,
			ExpandSelection: edutil.MakeExpandSelectionFn(e.bodybuf),
			VisibleTick:     false,
			Colors:          editorColors,
		},
	}
	e.otherSel = make([]util.Sel, NUM_OTHER_SEL)
	e.sfr.Fr.Scroll = edutil.MakeScrollfn(e.bodybuf, &e.otherSel[OS_TOP], &e.sfr)
	hf = textframe.HF_TRUNCATE
	if config.QuoteHack {
		hf |= textframe.HF_QUOTEHACK
	}

	e.tagfr = textframe.Frame{
		Font:            config.TagFont,
		Hackflags:       hf,
		Scroll:          func(sd, sl int) {},
		ExpandSelection: edutil.MakeExpandSelectionFn(e.tagbuf),
		VisibleTick:     false,
		Colors:          tagColors,
	}

	e.jumps = make([]util.Sel, NUM_JUMPS)

	e.otherSel[OS_TOP].E = 0
	e.otherSel[OS_TIP].E = 0

	bodybuf.Props["font"] = Wnd.Prop["font"]
	if bodybuf.Props["font"] == "alt" {
		e.sfr.Fr.Font = config.AltFont
	} else {
		e.sfr.Fr.Font = config.MainFont
	}

	util.Must(e.sfr.Init(5), "Editor initialization failed")
	util.Must(e.tagfr.Init(5), "Editor initialization failed")

	e.GenTag()
	e.tagbuf.Replace([]rune("Look "), &util.Sel{e.tagbuf.Size(), e.tagbuf.Size()}, true, nil, util.EO_FILES, false)
	e.tagfr.Sels[0].S = e.tagbuf.Size()
	e.tagfr.Sels[0].E = e.tagbuf.Size()

	e.sfr.Set(0, e.bodybuf.Size())

	e.tagbuf.AddSels(&e.tagfr.Sels)
	e.bodybuf.AddSels(&e.sfr.Fr.Sels)
	e.bodybuf.AddSels(&e.jumps)
	e.bodybuf.AddSels(&e.otherSel)

	e.eventReader.Reset()

	return e
}

func (e *Editor) SetRects(b draw.Image, r image.Rectangle, last bool) {
	e.last = last
	e.r = r
	e.btnr = r
	e.btnr.Max.X = e.btnr.Min.X + SCROLL_WIDTH
	e.btnr.Max.Y = e.btnr.Min.Y + TagHeight(&e.tagfr) + 3

	sfrr := r
	sfrr.Min.Y = sfrr.Min.Y + TagHeight(&e.tagfr) + 3
	if !last {
		sfrr.Max.X -= 2
	}
	e.sfr.SetRects(b, sfrr)

	e.bodybuf.DisplayLines = int(float64(sfrr.Max.Y-sfrr.Min.Y) / float64(e.sfr.Fr.Font.LineHeight()))

	if (e.pw != e.r.Dx()) && e.bodybuf.IsDir() {
		e.pw = e.r.Dx()
		e.readDir()
	}
	e.pw = e.r.Dx()

	e.sfr.Fr.Clear()
	ba, bb := e.bodybuf.Selection(util.Sel{e.otherSel[OS_TOP].E, e.bodybuf.Size()})
	e.sfr.Fr.InsertColor(ba)
	e.sfr.Fr.InsertColor(bb)

	e.tagfr.R = r
	e.tagfr.R.Min.Y += 2
	e.tagfr.R.Min.X += SCROLL_WIDTH
	if !last {
		e.tagfr.R.Max.X -= 2
	}
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

	e.bodybuf.Highlight(-1, false, e.otherSel[OS_TOP].E)
}

func (e *Editor) Close() {
	e.bodybuf.RmSels(&e.sfr.Fr.Sels)
	e.bodybuf.RmSels(&e.jumps)
	e.bodybuf.RmSels(&e.otherSel)
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

	// draw two-pixel border at the top and at the right of the editor
	border := e.r
	border.Max.Y = border.Min.Y + 2
	drawingFuncs.DrawFillSrc(e.sfr.Fr.B, e.r.Intersect(border), &config.TheColorScheme.Border)

	if !e.last {
		border = e.r
		border.Min.X = border.Max.X - 2
		drawingFuncs.DrawFillSrc(e.sfr.Fr.B, e.r.Intersect(border), &config.TheColorScheme.Border)
	}

	// draw one-pixel tag border
	border = e.r
	if !e.last {
		border.Max.X -= 2
	}
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
		if e.bodybuf.Modified && (e.bodybuf.Name[0] != '+' && !e.bodybuf.IsDir()) {
			t += " Put"
		}
		if e.bodybuf.HasUndo() {
			t += " Undo"
		}
		if e.bodybuf.HasRedo() {
			t += " Redo"
		}
		if e.bodybuf.IsDir() {
			t += " Get"
		}
	} else {
		t += e.specialTag
	}

	t += " | " + usertext
	start := e.tagfr.Sels[0].S - e.tagbuf.EditableStart
	end := e.tagfr.Sels[0].E - e.tagbuf.EditableStart
	if start < 0 || end < 0 {
		start = 0
		end = 0
	}
	e.tagbuf.EditableStart = -1
	e.tagbuf.Replace([]rune(t), &util.Sel{0, e.tagbuf.Size()}, true, nil, 0, false)
	TagSetEditableStart(e.tagbuf)
	e.tagfr.Sels[0].S = start + e.tagbuf.EditableStart
	e.tagfr.Sels[0].E = end + e.tagbuf.EditableStart
	e.tagbuf.FixSel(&e.tagfr.Sels[0])
}

func (e *Editor) refreshIntl() {
	e.sfr.Fr.Clear()
	e.sfr.Set(e.otherSel[OS_TOP].E, e.bodybuf.Size())

	e.bodybuf.Rdlock()
	defer e.bodybuf.Rdunlock()
	ba, bb := e.bodybuf.Selection(util.Sel{e.otherSel[OS_TOP].E, e.bodybuf.Size()})
	e.sfr.Fr.InsertColor(ba)
	e.sfr.Fr.InsertColor(bb)
}

func (e *Editor) BufferRefreshEx(ontag bool, recur, scroll bool) {
	match := findPMatch(e.tagbuf, e.tagfr.Sels[0])
	if match.S >= 0 {
		e.tagfr.Sels[PMATCHSEL] = match
	} else {
		e.tagfr.Sels[PMATCHSEL].S = e.tagfr.Sels[PMATCHSEL].E
	}
	match = findPMatch(e.bodybuf, e.sfr.Fr.Sels[0])
	if match.S >= 0 {
		e.sfr.Fr.Sels[PMATCHSEL] = match
	} else {
		e.sfr.Fr.Sels[PMATCHSEL].S = e.sfr.Fr.Sels[PMATCHSEL].E
	}

	if ontag {
		e.tagRefreshIntl()
		if e.tagRecenterIntl() {
			e.tagRefreshIntl()
		}
		e.tagfr.Redraw(true)
	} else {
		e.refreshIntl()
		if !e.sfr.Fr.Inside(e.sfr.Fr.Sels[0].E) && recur && scroll {
			x := e.bodybuf.Tonl(e.sfr.Fr.Sels[0].E-2, -1)
			e.otherSel[OS_TOP].E = x
			e.refreshIntl()
			edutil.Scrollfn(e.bodybuf, &e.otherSel[OS_TOP], &e.sfr, -1, e.sfr.Fr.LineNo()/4-1)
			e.bodybuf.Highlight(-1, false, e.otherSel[OS_TOP].E)
		}

		e.GenTag()
		e.tagfr.Clear()
		ta, tb := e.tagbuf.Selection(util.Sel{0, e.tagbuf.Size()})
		e.tagfr.InsertColor(ta)
		e.tagfr.InsertColor(tb)

		e.Redraw()
		e.sfr.Wnd.FlushImage(e.r)

		if (e.bodybuf.RefCount <= 1) || !recur {
			return
		}

		for _, col := range Wnd.cols.cols {
			for _, oe := range col.editors {
				if (oe.bodybuf == e.bodybuf) && (oe != e) {
					oe.BufferRefreshEx(false, false, false)
				}
			}
		}
	}
}

func (e *Editor) tagRefreshIntl() {
	e.tagfr.Clear()
	ta, tb := e.tagbuf.Selection(util.Sel{0, e.tagbuf.Size()})
	e.tagfr.InsertColor(ta)
	e.tagfr.InsertColor(tb)
}

func (e *Editor) tagRecenterIntl() bool {
	tagtxt := e.tagbuf.SelectionRunes(util.Sel{0, e.tagbuf.Size()})
	taglen := e.tagfr.Measure(tagtxt)
	if taglen < e.tagfr.R.Dx()-10 {
		if e.tagfr.Offset != 0 {
			e.tagfr.Offset = 0
			return true
		} else {
			return false
		}
	}

	p := e.tagfr.PointToCoord(e.tagfr.Sels[0].S)
	if e.tagfr.Inside(e.tagfr.Sels[0].S) && p.In(e.tagfr.R) {
		return false
	}

	dst := (e.tagfr.R.Max.X - e.tagfr.R.Min.X) / 2
	nm := -(p.X - e.tagfr.R.Min.X + e.tagfr.Offset) + dst
	if nm > 0 {
		nm = 0
	}

	if e.tagfr.Offset != nm {
		e.tagfr.Offset = nm
		return true
	} else {
		return false
	}
}

func (e *Editor) BufferRefresh(ontag bool) {
	e.BufferRefreshEx(ontag, true, true)
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
	bounds := e.sfr.Fr.Font.Bounds()
	return e.sfr.Fr.Limit.Y - e.r.Min.Y - int(bounds.YMin) + 2
}

func (ed *Editor) recenterIntl() bool {
	return true
}

func (ed *Editor) Warp() {
	if !HasFocus {
		return
	}
	p := ed.sfr.Fr.PointToCoord(ed.sfr.Fr.Sels[0].S)
	if !ed.sfr.Fr.VisibleTick {
		ed.sfr.Fr.VisibleTick = true
		ed.sfr.Fr.Redraw(false)
		Wnd.wnd.FlushImage(ed.sfr.Fr.R)
	}
	Wnd.wnd.WarpMouse(p)
}

func (ed *Editor) getDelPos() int {
	sep := []rune(" Del ")
	s := ed.tagbuf.SelectionRunes(util.Sel{0, ed.tagbuf.Size()})
	for i := range s {
		match := true
		for j := range sep {
			if s[i+j] != sep[j] {
				match = false
				break
			}
		}
		if match {
			return i + 1
		}
	}
	return -1
}

func (ed *Editor) WarpToDel() {
	delp := ed.getDelPos()
	if delp < 0 {
		return
	}
	delCoord := ed.tagfr.PointToCoord(delp)
	if !ed.tagfr.VisibleTick {
		ed.tagfr.VisibleTick = true
		ed.tagfr.Redraw(false)
		Wnd.wnd.FlushImage(ed.tagfr.R)
	}
	delCoord.Y -= 5
	delCoord.X += 5
	Wnd.wnd.WarpMouse(delCoord)
}

func (ed *Editor) EnterSpecial(specialChan chan string, specialTag string, exitOnReturn bool) bool {
	if ed.specialChan != nil {
		return false
	}
	ed.specialChan = specialChan
	ed.specialTag = specialTag
	ed.specialExitOnReturn = exitOnReturn
	ed.savedTag = string(ed.tagbuf.SelectionRunes(util.Sel{ed.tagbuf.EditableStart, ed.tagbuf.Size()}))
	ed.tagbuf.Replace([]rune{}, &util.Sel{ed.tagbuf.EditableStart, ed.tagbuf.Size()}, true, nil, 0, false)
	ed.BufferRefresh(false)
	return true
}

func (ed *Editor) ExitSpecial() {
	close(ed.specialChan)
	ed.specialChan = nil
	ed.specialTag = ""
	ed.tagbuf.Replace([]rune(ed.savedTag), &util.Sel{ed.tagbuf.EditableStart, ed.tagbuf.Size()}, true, nil, 0, false)
	ed.BufferRefresh(false)
}

func (ed *Editor) PropTrigger() {
	tabWidth, err := strconv.Atoi(ed.bodybuf.Props["tab"])
	if err == nil {
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
		ed.sfr.Fr.Sels[0].S,
	}
}

func (ed *Editor) PushJump() {
	for i := len(ed.jumps) - 2; i >= 0; i-- {
		ed.jumps[i+1] = ed.jumps[i]
	}
	ed.jumps[0].S = ed.sfr.Fr.Sels[0].S
	ed.restoredJump = 0
	ed.jumpCount++
}

func (ed *Editor) RestoreJump() {
	if ed.jumpCount == 0 {
		return
	}

	// if we haven't recently restored a jump since the last push, refer to the last pushed jump
	if ed.restoredJump < 0 {
		ed.restoredJump = 0
	}

	// if we moved since the last restored (or pushed jump)
	if ed.sfr.Fr.Sels[0].S != ed.jumps[ed.restoredJump].S {
		// we push the current position, then restore the previously last jump done
		ed.PushJump()
		ed.sfr.Fr.Sels[0].S = ed.jumps[1].S
		ed.sfr.Fr.Sels[0].E = ed.jumps[1].S
		ed.restoredJump = 1
		return
	}

	// we are on the last restored jump, cycle through jump
	ed.restoredJump++
	if (ed.restoredJump >= len(ed.jumps)) || (ed.restoredJump >= ed.jumpCount) {
		ed.restoredJump = 0
	}

	ed.sfr.Fr.Sels[0].S = ed.jumps[ed.restoredJump].S
	ed.sfr.Fr.Sels[0].E = ed.jumps[ed.restoredJump].S
}

func (ed *Editor) LastJump() int {
	return ed.sfr.Fr.Sels[0].S
}

func (e *Editor) readDir() {
	fh, err := os.Open(filepath.Join(e.bodybuf.Dir, e.bodybuf.Name))
	if err != nil {
		return
	}
	defer fh.Close()

	fis, err := fh.Readdir(-1)
	if err != nil {
		return
	}

	sort.Sort(fileInfos(fis))

	r := make([]string, 0, len(fis))
	for _, fi := range fis {
		n := fi.Name()
		if config.HideHidden && (len(n) <= 0 || n[0] == '.') {
			continue
		}
		switch {
		case fi.IsDir():
			n += "/"
		case fi.Mode()&os.ModeSymlink != 0:
			n += "@"
		case fi.Mode()&0111 != 0:
			n = "./" + n
		default:
			if strings.Index(n, " ") >= 0 || strings.Index(n, "\n") >= 0 || !easyCommand(n) {
				n = util.SingleQuote(n)
			}
		}
		r = append(r, n)
	}

	spaceWidth := e.sfr.Fr.Measure([]rune(" ")) * _ELASTIC_TABS_SPACING

	szs := make([]int, len(r))

	for i := range r {
		szs[i] = e.sfr.Fr.Measure([]rune(r[i]))
	}

	L := e.sfr.Fr.R.Dx() - 10
	var n int
	for n = 15; n > 0; n-- {
		max := make([]int, n)
		for i := range max {
			max[i] = 0
		}

		for i := range szs {
			if szs[i] > max[i%n] {
				max[i%n] = szs[i]
			}
		}

		tot := 0
		for i := range max {
			if i != 0 {
				tot += spaceWidth
			}
			tot += max[i]
		}

		if tot < L {
			break
		}
	}

	if n <= 0 {
		n = 1
	}

	rr := []string{}
	for i := range r {
		if (i != 0) && ((i % n) == 0) {
			rr = append(rr, "\n")
		}
		rr = append(rr, r[i])
		if (i % n) != n-1 {
			rr = append(rr, "\t")
		}
	}

	rr = append(rr, "\n")

	e.bodybuf.Replace([]rune(strings.Join(rr, "")), &util.Sel{0, e.bodybuf.Size()}, true, nil, 0, false)
	e.bodybuf.Modified = false
	e.bodybuf.UndoReset()
	elasticTabs(e, true)
}

func (e *Editor) closeEventChan() {
	if e.eventChan == nil {
		return
	}
	close(e.eventChan)
	e.eventChan = nil
	Warn(fmt.Sprintf("Event channel for %s was unresponsive, closed", e.bodybuf.ShortName()))
}

type fileInfos []os.FileInfo

func (fis fileInfos) Less(i, j int) bool {
	isdiri := fis[i].IsDir()
	isdirj := fis[j].IsDir()

	switch {
	case isdiri && !isdirj:
		return true
	case !isdiri && isdirj:
		return false
	default:
		return fis[i].Name() < fis[j].Name()
	}
}

func (fis fileInfos) Swap(i, j int) {
	fi := fis[i]
	fis[i] = fis[j]
	fis[j] = fi
}

func (fis fileInfos) Len() int {
	return len(fis)
}
