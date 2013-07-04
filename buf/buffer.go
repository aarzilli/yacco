package buf

import (
	"os"
	"io"
	"bufio"
	"fmt"
	"unicode"
	"unicode/utf8"
	"regexp"
	"io/ioutil"
	"yacco/util"
	"yacco/textframe"
	"path/filepath"
)

const SLOP = 128

var nonwdRe = regexp.MustCompile(`\W+`)

type Buffer struct {
	Dir string
	Name string
	Editable bool
	EditableStart int
	Modified bool

	// gap buffer implementation
	buf []textframe.ColorRune
	gap, gapsz int

	ul undoList
	
	Words []string
}

func NewBuffer(dir, name string, create bool) (b *Buffer, err error) {
	//println("NewBuffer call:", dir, name)
	b = &Buffer{
		Dir: dir,
		Name: name,
		Editable: true,
		EditableStart: -1,
		Modified: false,

		buf: make([]textframe.ColorRune, SLOP),
		gap: 0,
		gapsz: SLOP,

		ul: undoList{ 0, []undoInfo{} }, }

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

	if name[0] != '+' {
		infile, err := os.Open(filepath.Join(dir, name))
		if err == nil {
			defer infile.Close()
			
			fi, err := infile.Stat()
			if err != nil {
				return nil, fmt.Errorf("Couldn't stat file (after opening it?) %s", filepath.Join(dir, name))
			}
			
			if fi.Size() > 10 * 1024 * 1024 {
				return nil, fmt.Errorf("Refusing to open files larger than 10MB")
			}
			
			bytes, err := ioutil.ReadAll(infile)
			testb := bytes
			if len(testb) > 1024 {
				testb = testb[:1024]
			}
			if !utf8.Valid(testb) {
				return nil, fmt.Errorf("Can not open binary file")
			}
			if err != nil {
				return nil, err
			}
			str := string(bytes)
			b.Words = util.Dedup(nonwdRe.Split(str, -1))
			runes := []rune(str)
			b.Replace(runes, &util.Sel{ 0, 0 }, []util.Sel{}, true, nil, 0)
			b.Modified = false
			b.ul.Reset()
		} else {
			if create {
				// doesn't exist, mark as modified
				b.Modified = true
			} else {
				return nil, fmt.Errorf("File doesn't exist: %s", filepath.Join(dir, name))
			}
		}
	}

	return b, nil
}

// Replaces text between sel.S and sel.E with text, updates sels AND sel accordingly
// After the replacement the highlighter is restarted
func (b *Buffer) Replace(text []rune, sel *util.Sel, sels []util.Sel, solid bool, eventChan chan string, origin util.EventOrigin) {
	if (!b.Editable) {
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

	b.hlFrom(sel.S-1)
	b.unlock()

	sel.S = sel.S + len(text)
	sel.E = sel.S
}

func (b *Buffer) generateEvent(text []rune, sel util.Sel, eventChan chan string, origin util.EventOrigin) {
	if eventChan == nil {
		return
	}

	if sel.S != sel.E {
		etypec := 'D'
		util.Fmtevent(eventChan, origin, b.Name == "+Tag", util.EventType(etypec), sel.S, sel.E, 0, "")
	}

	if (sel.S == sel.E) || (len(text) != 0) {
		etypec := 'I'
		if b.Name == "+Tag" {
			etypec = unicode.ToLower(etypec)
		}
		util.Fmtevent(eventChan, origin, b.Name == "+Tag", util.EventType(etypec), sel.S, sel.S, 0, string(text))
	}
}

// Undo last change. Redoes last undo if redo == true
func (b *Buffer) Undo(sels []util.Sel, redo bool) {
	if (!b.Editable) {
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
		ws := util.Sel{ us.S, us.E }
		b.replaceIntl(text, &ws, sels)
		updateSels(sels, &ws, len(text))

		b.hlFrom(ws.S-1)

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
}

func (b *Buffer) Rdlock() {
	//TODO
}

func (b *Buffer) Rdunlock() {
	//TODO
}

func (b *Buffer) wrlock() {
	//TODO: stop highlighter here, lock buffer
}

func (b *Buffer) hlFrom(s int) {
	//TODO: start highlighter at s (needs how many lines to do?)
}

func (b *Buffer) unlock() {
	//TODO: unlock buffer
}

// Replaces text between sel.S and sel.E with text, updates selections in sels except sel itself
// NOTE sel IS NOT modified, we need a pointer specifically so we can skip updating it in sels
func (b *Buffer) replaceIntl(text []rune, sel *util.Sel, sels []util.Sel) {
	regionSize := sel.E - sel.S

	if (sel.S != sel.E) {
		updateSels(sels, sel, -regionSize)
		b.MoveGap(sel.S)
		b.gapsz += regionSize // this effectively deletes the current selection
	} else {
		b.MoveGap(sel.S)
	}

	b.IncGap(len(text))
	for i, r := range text {
		b.buf[b.gap + i].C = 1
		b.buf[b.gap + i].R = r
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
	ui.before.text = string(ToRunes(b.SelectionX(sel)))
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
		} else if (sels[i].S > sel.S) {
			sels[i].S += delta
		}

		if (sels[i].E >= sel.S) && (sels[i].E < end) {
			sels[i].E = sel.S
		} else if (sels[i].E > sel.S) {
			sels[i].E += delta
		}
	}
}

// Increases the size of the gap to fit at least delta more items
func (b *Buffer) IncGap(delta int) {
	if b.gapsz > delta+1 {
		return
	}

	ngapsz := (delta / SLOP + 1) * SLOP

	nbuf := make([]textframe.ColorRune, len(b.buf) - b.gapsz + ngapsz)

	copy(nbuf, b.buf[:b.gap])
	copy(nbuf[b.gap + ngapsz:], b.buf[b.gap+b.gapsz:])

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
		if b.gap - pp > 0 {
			//size =  b.gap - pp
			//memmove(buffer->buf + pp + buffer->gapsz, buffer->buf + pp, sizeof(my_glyph_info_t) * size);
			copy(b.buf[ pp + b.gapsz : ], b.buf[pp : b.gap])
		}
		b.gap = pp
	} else if pp > b.gap {
		if pp - b.gap - b.gapsz > 0 {
			//size = pp - b.gap - b.gapsz
			//memmove(buffer->buf + buffer->gap, buffer->buf + buffer->gap + buffer->gapsz, sizeof(my_glyph_info_t) * size);
			copy(b.buf[b.gap:], b.buf[b.gap + b.gapsz : pp])
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
	ps := b.phisical(sel.S)
	pe := b.phisical(sel.E)

	if (ps < b.gap) && (pe >= b.gap) {
		return b.buf[ps:b.gap], b.buf[b.gap+b.gapsz:pe]
	} else {
		if pe <= ps {
			return []textframe.ColorRune{}, []textframe.ColorRune{}
		} else {
			return b.buf[ps:pe], []textframe.ColorRune{}
		}
	}
}

// Returns the specified selection as single slice of ColorRunes (will allocate)
func (b *Buffer) SelectionX(sel util.Sel) []textframe.ColorRune {
	ba, bb := b.Selection(sel)
	r := make([]textframe.ColorRune, len(ba)+len(bb))
	copy(r, ba)
	copy(r[len(ba):], bb)
	return r
}

// Returns the specified selection as single slice of runes (will allocate)
func (b *Buffer) SelectionRunes(sel util.Sel) []rune {
	ba, bb := b.Selection(sel)
	r := make([]rune, len(ba)+len(bb))
	j := 0
	for i := range ba {
		r[j] = ba[i].R
		j++
	}
	for i := range bb {
		r[j] = bb[i].R
		j++
	}
	return r
}

func (b *Buffer) Size() int {
	return len(b.buf) - b.gapsz
}

func (b *Buffer) Tonl(start int, dir int) int {
	sz := b.Size()
	ba, bb := b.Selection(util.Sel{ 0, sz })

	i := start
	if i < 0 {
		i = 0
	}
	if i >= sz {
		i = sz-1
	}
	for ; (i >= 0) && (i < sz); i += dir {
		var c rune

		if i < len(ba) {
			c = ba[i].R
		} else {
			c = bb[i - len(ba)].R
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

func (b *Buffer) ShortName() string {
	ap := filepath.Clean(filepath.Join(b.Dir, b.Name))
	wd, _ := os.Getwd()
	p, _ := filepath.Rel(wd, ap)
	if len(ap) < len(p) {
		p = ap
	}
	//TODO: compress like ppwd
	return p
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
	ba, bb := b.Selection(util.Sel{ 0, b.Size() })
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
	return nil
}

func (b *Buffer) HasUndo() bool {
	return b.ul.cur > 0
}

func (b *Buffer) HasRedo() bool {
	return b.ul.cur < len(b.ul.lst)
}

func (b *Buffer) ReaderFrom(s, e int) io.RuneReader {
	return &runeReader{ b, s, e }
}

type runeReader struct {
	b *Buffer
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
	ba, bb := b.Selection(util.Sel{ 0, b.Size() })
	if i < len(ba) {
		n, c := countNl(ba[:i])
		return n+1, c
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

