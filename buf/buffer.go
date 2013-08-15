package buf

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
	"yacco/config"
	"yacco/textframe"
	"yacco/util"
)

const SLOP = 128
const SHORT_NAME_LEN = 40

var nonwdRe = regexp.MustCompile(`\W+`)

type Buffer struct {
	Dir           string
	Name          string
	Editable      bool
	EditableStart int
	Modified      bool
	ModTime       time.Time // time the file was modified on disk

	Props map[string]string

	// gap buffer implementation
	buf        []textframe.ColorRune
	gap, gapsz int

	ul undoList

	lock              sync.RWMutex
	hlock             sync.Mutex
	ReadersPleaseStop bool

	DisplayLines  int
	lastCleanHl   int
	HighlightChan chan *Buffer

	RefCount int

	Words []string

	EditMark, EditMarkNext bool

	DumpCmd, DumpDir string
}

func NewBuffer(dir, name string, create bool, indentchar string) (b *Buffer, err error) {
	//println("NewBuffer call:", dir, name)
	b = &Buffer{
		Dir:           dir,
		Name:          name,
		Editable:      true,
		EditableStart: -1,
		Modified:      false,

		buf:   make([]textframe.ColorRune, SLOP),
		gap:   0,
		gapsz: SLOP,

		ul: undoList{0, []undoInfo{}}}

	dirfile, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer dirfile.Close()
	dirinfo, err := dirfile.Stat()
	if err != nil {
		return nil, err
	}
	if !dirinfo.IsDir() {
		return nil, fmt.Errorf("Not a directory: %s", dir)
	}

	b.lastCleanHl = -1

	if name[0] != '+' {
		err := b.Reload([]util.Sel{}, create)
		if err != nil {
			return nil, err
		}
	}

	b.Props = map[string]string{}
	b.Props["indentchar"] = indentchar
	b.Props["font"] = "main"
	b.Props["indent"] = "on"
	b.Props["tab"] = "8"

	b.EditMarkNext = true
	b.EditMark = true

	return b, nil
}

// (re)loads buffer from disk
func (b *Buffer) Reload(sels []util.Sel, create bool) error {
	path := filepath.Join(b.Dir, b.Name)
	infile, err := os.Open(path)
	if err == nil {
		defer infile.Close()

		fi, err := infile.Stat()
		if err != nil {
			return fmt.Errorf("Couldn't stat file (after opening it?) %s", path)
		}

		if fi.IsDir() {
			return b.reloadDir(sels, infile)
		}

		if fi.Size() > 10*1024*1024 {
			return fmt.Errorf("Refusing to open files larger than 10MB")
		}

		b.ModTime = fi.ModTime()

		bytes, err := ioutil.ReadAll(infile)
		testb := bytes
		if len(testb) > 1024 {
			testb = testb[:1024]
		}
		if !utf8.Valid(testb) {
			return fmt.Errorf("Can not open binary file")
		}
		if err != nil {
			return err
		}
		str := string(bytes)
		b.Words = util.Dedup(nonwdRe.Split(str, -1))
		runes := []rune(str)
		b.Replace(runes, &util.Sel{0, b.Size()}, sels, true, nil, 0, false)
		b.Modified = false
		b.ul.Reset()
	} else {
		if create {
			// doesn't exist, mark as modified
			b.Modified = true
			b.ModTime = time.Now()
		} else {
			return fmt.Errorf("File doesn't exist: %s", path)
		}
	}

	return nil
}

func (b *Buffer) reloadDir(sels []util.Sel, fh *os.File) error {
	if b.Name[len(b.Name)-1] != '/' {
		b.Name = b.Name + "/"
	}
	fis, err := fh.Readdir(-1)
	if err != nil {
		return err
	}

	r := make([]string, 0, len(fis))
	for _, fi := range fis {
		n := fi.Name()
		switch {
		case fi.IsDir():
			n += "/"
		case fi.Mode()&os.ModeSymlink != 0:
			n += "@"
		case fi.Mode()&0111 != 0:
			n += "*"
		}
		r = append(r, n)
	}

	b.Replace([]rune(strings.Join(r, "\t")), &util.Sel{0, b.Size()}, sels, true, nil, 0, false)

	b.Modified = false
	b.ul.Reset()
	return nil
}

// Replaces text between sel.S and sel.E with text, updates sels AND sel accordingly
// After the replacement the highlighter is restarted
func (b *Buffer) Replace(text []rune, sel *util.Sel, sels []util.Sel, solid bool, eventChan chan string, origin util.EventOrigin, dohl bool) {
	if !b.Editable {
		return
	}

	if sel.S < 0 {
		sel.S = sel.E
	}

	if sel.S < b.EditableStart {
		sel.S = b.EditableStart
		sel.E = b.EditableStart
		return
	}

	b.generateEvent(text, *sel, eventChan, origin)

	b.wrlock()

	b.Modified = true

	b.pushUndo(*sel, text, solid)
	b.replaceIntl(text, sel, sels)
	updateSels(sels, sel, len(text))

	if dohl {
		b.Highlight(sel.S-1, false, -1)
	} else {
		b.lastCleanHl = sel.S - 1
	}

	b.unlock()

	sel.S = sel.S + len(text)
	sel.E = sel.S
}

func (b *Buffer) generateEvent(text []rune, sel util.Sel, eventChan chan string, origin util.EventOrigin) {
	if eventChan == nil {
		return
	}

	if sel.S != sel.E {
		util.FmteventBase(eventChan, origin, b.Name == "+Tag", util.ET_BODYDEL, sel.S, sel.E, "")
	}

	if (sel.S == sel.E) || (len(text) != 0) {
		util.FmteventBase(eventChan, origin, b.Name == "+Tag", util.ET_BODYINS, sel.S, sel.S, string(text))
	}
}

// Undo last change. Redoes last undo if redo == true
func (b *Buffer) Undo(sels []util.Sel, redo bool) {
	if !b.Editable {
		return
	}

	first := true

	for {
		var ui *undoInfo
		if redo {
			ui = b.ul.PeekUndo()
			if (ui != nil) && ui.solid && !first {
				return
			}
			ui = b.ul.Redo()
		} else {
			ui = b.ul.Undo()
		}

		if ui == nil {
			return
		}

		first = false

		b.wrlock()

		var us undoSel
		var text []rune
		if redo {
			us = ui.before
			text = []rune(ui.after.text)
		} else {
			us = ui.after
			text = []rune(ui.before.text)
		}
		ws := util.Sel{us.S, us.E}
		b.replaceIntl(text, &ws, sels)
		updateSels(sels, &ws, len(text))

		b.unlock()

		sels[0].S = ws.S
		sels[0].E = ws.S

		mui := ui
		if !redo {
			mui = b.ul.PeekUndo()
		}

		b.Modified = (mui != nil) && !mui.saved

		if !redo {
			if ui.solid {
				return
			}
		}
	}
	b.Highlight(-1, true, -1)
}

func (b *Buffer) Rdlock() {
	b.lock.RLock()
}

func (b *Buffer) Rdunlock() {
	b.lock.RUnlock()
}

func (b *Buffer) wrlock() {
	b.lock.RLock()
	b.ReadersPleaseStop = true
	b.lock.RUnlock()
	b.lock.Lock()
	b.ReadersPleaseStop = false
}

func (b *Buffer) unlock() {
	b.lock.Unlock()
}

// Highlights buffers starting at "start"
// If full is specified the highlighting will continue until the end of the buffer,
// otherwise it will continue until after DisplayLines lines after pinc
func (b *Buffer) Highlight(start int, full bool, pinc int) {
	if !config.EnableHighlighting {
		return
	}

	if (len(b.Name) == 0) || (b.Name[0] == '+') || (b.DisplayLines == 0) {
		return
	}

	if (start < 0) && !full && (pinc >= 0) && ((pinc + 50*b.DisplayLines) < b.lastCleanHl) {
		return
	}

	if (start < 0) && (b.lastCleanHl >= b.Size()) {
		return
	}

	go b.highlightIntl(start, full, pinc)
}

// Replaces text between sel.S and sel.E with text, updates selections in sels except sel itself
// NOTE sel IS NOT modified, we need a pointer specifically so we can skip updating it in sels
func (b *Buffer) replaceIntl(text []rune, sel *util.Sel, sels []util.Sel) {
	regionSize := sel.E - sel.S

	if sel.S != sel.E {
		updateSels(sels, sel, -regionSize)
		b.MoveGap(sel.S)
		b.gapsz += regionSize // this effectively deletes the current selection
	} else {
		b.MoveGap(sel.S)
	}

	b.IncGap(len(text))
	for i, r := range text {
		b.buf[b.gap+i].C = 1
		b.buf[b.gap+i].R = r
	}
	b.gap += len(text)
	b.gapsz -= len(text)
}

// Saves undo information for replacement of text between sel.S and sel.E with text
func (b *Buffer) pushUndo(sel util.Sel, text []rune, solid bool) {
	if b.Name == "+Tag" {
		return
	}
	var ui undoInfo
	ui.before.S = sel.S
	ui.before.E = sel.E
	ui.before.text = string(b.SelectionRunes(sel))
	ui.after.S = sel.S
	ui.after.E = sel.S + len(text)
	ui.after.text = string(text)
	ui.solid = solid
	b.ul.Add(ui)
}

// Updates position of items in sels except for the one pointed by sel
// The update is for a text replacement starting at sel.S of size delta
func updateSels(sels []util.Sel, sel *util.Sel, delta int) {
	var end int
	if delta < 0 {
		end = sel.S - delta
	} else {
		end = -1
	}

	for i := range sels {
		if &sels[i] == sel {
			continue
		}

		if (sels[i].S >= sel.S) && (sels[i].S < end) {
			sels[i].S = sel.S
		} else if sels[i].S > sel.S {
			sels[i].S += delta
		}

		if (sels[i].E >= sel.S) && (sels[i].E < end) {
			sels[i].E = sel.S
		} else if sels[i].E > sel.S {
			sels[i].E += delta
		}
	}
}

// Increases the size of the gap to fit at least delta more items
func (b *Buffer) IncGap(delta int) {
	if b.gapsz > delta+1 {
		return
	}

	ngapsz := (delta/SLOP + 1) * SLOP

	nbuf := make([]textframe.ColorRune, len(b.buf)-b.gapsz+ngapsz)

	copy(nbuf, b.buf[:b.gap])
	copy(nbuf[b.gap+ngapsz:], b.buf[b.gap+b.gapsz:])

	b.buf = nbuf
	b.gapsz = ngapsz
}

// Displaces gap to start at point p
func (b *Buffer) MoveGap(p int) {
	pp := b.phisical(p)
	if pp > len(b.buf) {
		panic(fmt.Errorf("MoveGap point out of range: %d", pp))
	}

	if pp < b.gap {
		if b.gap-pp > 0 {
			//size =  b.gap - pp
			//memmove(buffer->buf + pp + buffer->gapsz, buffer->buf + pp, sizeof(my_glyph_info_t) * size);
			copy(b.buf[pp+b.gapsz:], b.buf[pp:b.gap])
		}
		b.gap = pp
	} else if pp > b.gap {
		if pp-b.gap-b.gapsz > 0 {
			//size = pp - b.gap - b.gapsz
			//memmove(buffer->buf + buffer->gap, buffer->buf + buffer->gap + buffer->gapsz, sizeof(my_glyph_info_t) * size);
			copy(b.buf[b.gap:], b.buf[b.gap+b.gapsz:pp])
		}
		b.gap = pp - b.gapsz
	}
}

func (b *Buffer) phisical(p int) int {
	if p < b.gap {
		return p
	} else {
		return p + b.gapsz
	}
}

func (b *Buffer) At(p int) *textframe.ColorRune {
	pp := b.phisical(p)
	if (pp < 0) || (pp >= len(b.buf)) {
		return nil
	}
	return &b.buf[pp]
}

// Returns the specified selection as two slices. The slices are to be treated as contiguous and may be empty
func (b *Buffer) Selection(sel util.Sel) ([]textframe.ColorRune, []textframe.ColorRune) {
	//println(sel.S, sel.E)
	ps := b.phisical(sel.S)
	pe := b.phisical(sel.E)

	if (ps < b.gap) && (pe >= b.gap) {
		//println(len(b.buf), b.gap, b.gapsz, ps, pe)
		return b.buf[ps:b.gap], b.buf[b.gap+b.gapsz : pe]
	} else {
		if pe <= ps {
			return []textframe.ColorRune{}, []textframe.ColorRune{}
		} else {
			return b.buf[ps:pe], []textframe.ColorRune{}
		}
	}
}

// Returns the specified selection as single slice of ColorRunes (will allocate)
func (b *Buffer) SelectionRunes(sel util.Sel) []rune {
	ba, bb := b.Selection(sel)
	r := make([]rune, len(ba)+len(bb))
	for i := range ba {
		r[i] = ba[i].R
	}
	for i := range bb {
		r[i+len(ba)] = bb[i].R
	}
	return r
}

func (b *Buffer) Size() int {
	return len(b.buf) - b.gapsz
}

// Moves to the beginning or end of a line
func (b *Buffer) Tonl(start int, dir int) int {
	sz := b.Size()
	ba, bb := b.Selection(util.Sel{0, sz})

	i := start
	if i < 0 {
		return 0
	}
	if i >= sz {
		i = sz - 1
	}
	for ; (i >= 0) && (i < sz); i += dir {
		var c rune

		if i < len(ba) {
			c = ba[i].R
		} else {
			c = bb[i-len(ba)].R
		}

		if c == '\n' {
			return i + 1
		}
	}
	if dir < 0 {
		return 0
	} else {
		return sz
	}
}

// Moves to the beginning or end of an alphanumerically delimited word
func (b *Buffer) Towd(start int, dir int) int {
	first := (dir < 0)
	notfirst := !first
	var i int
	for i = start; (i >= 0) && (i < b.Size()); i += dir {
		c := b.At(i).R
		if !(unicode.IsLetter(c) || unicode.IsDigit(c) || (c == '_')) {
			if !first {
				i++
			}
			break
		}
		first = notfirst
	}
	if i < 0 {
		i = 0
	}
	return i
}

// Moves to the beginning or end of a space delimited word
func (b *Buffer) Tospc(start int, dir int) int {
	first := (dir < 0)
	notfirst := !first
	var i int
	for i = start; (i >= 0) && (i < b.Size()); i += dir {
		c := b.At(i).R
		if unicode.IsSpace(c) {
			if !first {
				i++
			}
			break
		}
		first = notfirst
	}
	if i < 0 {
		i = 0
	}
	return i
}

// Moves to the beginning or end of something that looks like a file path
func (b *Buffer) Tofp(start int, dir int) int {
	first := (dir < 0)
	notfirst := !first
	var i int
	for i = start; (i >= 0) && (i < b.Size()); i += dir {
		c := b.At(i).R
		if !(unicode.IsLetter(c) || unicode.IsDigit(c) || (c == '_') || (c == '-') || (c == '+') || (c == '/') || (c == '=') || (c == '~') || (c == '!') || (c == ':') || (c == ',') || (c == '.')) {
			if !first {
				i++
			}
			break
		}
		first = notfirst
	}
	if i < 0 {
		i = 0
	}
	return i

}

const OPEN_PARENTHESIS = "([{<"
const CLOSED_PARENTHESIS = ")]}>"

// Moves to the matching parenthesis of the one at 'start' in the specified direction
func (b *Buffer) Topmatch(start int, dir int) int {
	g := b.At(start)
	if g == nil {
		return -1
	}

	var open rune = 0
	var close rune = 0
	if dir > 0 {
		for i := range OPEN_PARENTHESIS {
			if g.R == rune(OPEN_PARENTHESIS[i]) {
				open = rune(OPEN_PARENTHESIS[i])
				close = rune(CLOSED_PARENTHESIS[i])
				break
			}
		}

	} else {
		for i := range CLOSED_PARENTHESIS {
			if g.R == rune(CLOSED_PARENTHESIS[i]) {
				open = rune(CLOSED_PARENTHESIS[i])
				close = rune(OPEN_PARENTHESIS[i])
				break
			}
		}
	}

	if (open == 0) || (close == 0) {
		return -1
	}

	depth := 0
	for i := start; i < b.Size(); i += dir {
		g := b.At(i)
		if g == nil {
			return -1
		}
		if g.R == open {
			depth++
		}
		if g.R == close {
			depth--
		}
		if depth == 0 {
			return i
		}
	}
	return -1
}

func (b *Buffer) Toregend(start int) int {
	if start >= b.Size() {
		return -1
	}

	c := b.At(start).C & 0x0f
	if c <= 1 {
		return -1
	}

	if (start != 0) && (b.At(start-1).C&0x0f) > 1 {
		return -1
	}

	var i int
	for i = start + 1; i < b.Size(); i++ {
		if (b.At(i).C & 0x0f) != c {
			break
		}
	}

	return i - 1
}

func (b *Buffer) ShortName() string {
	ap := filepath.Clean(filepath.Join(b.Dir, b.Name))
	wd, _ := os.Getwd()
	p, _ := filepath.Rel(wd, ap)
	if len(ap) < len(p) {
		p = ap
	}

	if len(p) <= 0 {
		return p
	}

	curlen := len(p)
	pcomps := strings.Split(p, string(filepath.Separator))
	i := 0

	for curlen > SHORT_NAME_LEN {
		if i >= len(pcomps)-2 {
			break
		}

		if (len(pcomps[i])) == 0 || (pcomps[i][0] == '.') || (pcomps[i][0] == '~') {
			i++
			continue
		}

		curlen -= len(pcomps[i]) - 1
		pcomps[i] = pcomps[i][:1]
		i++
	}

	rp := filepath.Join(pcomps...)
	if p[0] == '/' {
		return "/" + rp
	} else {
		return rp
	}
}

func (b *Buffer) FixSel(sel *util.Sel) {
	if sel.S < 0 {
		sel.S = 0
	} else if sel.S > b.Size() {
		sel.S = b.Size()
	}
	if sel.E < 0 {
		sel.E = 0
	} else if sel.E > b.Size() {
		sel.S = b.Size()
	}
}

func (b *Buffer) Put() error {
	out, err := os.OpenFile(filepath.Join(b.Dir, b.Name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer out.Close()
	bout := bufio.NewWriter(out)
	ba, bb := b.Selection(util.Sel{0, b.Size()})
	sa := string(ToRunes(ba))
	newWordsA := nonwdRe.Split(sa, -1)
	_, err = bout.Write([]byte(sa))
	if err != nil {
		return err
	}
	sb := string(ToRunes(bb))
	newWordsB := nonwdRe.Split(sb, -1)
	b.Words = util.Dedup(append(newWordsA[:len(newWordsA)-1], newWordsB[:len(newWordsB)-1]...))
	_, err = bout.Write([]byte(sb))
	if err != nil {
		return err
	}
	err = bout.Flush()
	if err != nil {
		return err
	}
	b.Modified = false

	fi, err := os.Stat(filepath.Join(b.Dir, b.Name))
	if err != nil {
		return err
	}

	b.ModTime = fi.ModTime()

	return nil
}

func (b *Buffer) HasUndo() bool {
	return b.ul.cur > 0
}

func (b *Buffer) HasRedo() bool {
	return b.ul.cur < len(b.ul.lst)
}

func (b *Buffer) ReaderFrom(s, e int) io.RuneReader {
	return &runeReader{b, s, e}
}

type runeReader struct {
	b    *Buffer
	s, e int
}

func (rr *runeReader) ReadRune() (r rune, size int, err error) {
	if rr.s >= rr.e {
		return 0, 0, io.EOF
	}
	cr := rr.b.At(rr.s)
	rr.s++
	if cr != nil {
		return cr.R, sizeOfRune(cr.R), nil
	} else {
		return 0, 0, io.EOF
	}
}

func (b *Buffer) GetLine(i int) (int, int) {
	ba, bb := b.Selection(util.Sel{0, b.Size()})
	if i < len(ba) {
		n, c := countNl(ba[:i])
		return n + 1, c
	} else {
		di := i - len(ba)
		na, offa := countNl(ba)
		nb, offb := countNl(bb[:di])
		if nb == 0 {
			return na + 1, offa + offb
		} else {
			return na + nb + 1, offb
		}
	}
}

func (b *Buffer) CanSave() bool {
	fi, err := os.Stat(filepath.Join(b.Dir, b.Name))
	if err != nil {
		return true
	}

	if fi.ModTime().Sub(b.ModTime) > 0 {
		return false
	} else {
		return true
	}
}

func countNl(rs []textframe.ColorRune) (int, int) {
	count := 0
	off := 0
	for _, r := range rs {
		if r.R == '\n' {
			count++
			off = 0
		} else {
			off++
		}
	}
	return count, off
}

func sizeOfRune(r rune) int {
	if r <= 0x007F {
		return 1
	}
	if r <= 0x07FF {
		return 2
	}
	if r <= 0xFFFF {
		return 3
	}
	if r <= 0x1FFFFF {
		return 4
	}
	// this cases never actually happen
	if r <= 0x3FFFFFF {
		return 5
	}
	return 6
}

func ToRunes(v []textframe.ColorRune) []rune {
	r := make([]rune, len(v))
	for i := range v {
		r[i] = v[i].R
	}
	return r
}

func (b *Buffer) UndoWhere() int {
	return b.ul.cur
}
