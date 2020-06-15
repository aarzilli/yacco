package buf

import (
	"bufio"
	"crypto/sha1"
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

	"github.com/aarzilli/yacco/config"
	"github.com/aarzilli/yacco/hl"
	"github.com/aarzilli/yacco/util"
)

const SLOP = 128

var nonwdRe = regexp.MustCompile(`\W+`)

type Buffer struct {
	Dir           string
	Name          string
	Editable      bool
	EditableStart int
	Modified      bool

	modTime        time.Time // time the file was modified on disk
	onDiskChecksum *[sha1.Size]byte

	Props map[string]string

	// gap buffer implementation
	buf        []rune
	sels       []*util.Sel
	gap, gapsz int

	ul undoList

	lock sync.RWMutex

	Hl    hl.Highlighter
	hlbuf []uint8

	RevCount int

	Words       []string
	WordsUpdate time.Time
	updcount    int

	EditMark, EditMarkNext bool

	// used to implement shift+movement selection actions
	Markat int

	DumpCmd, DumpDir string
}

// description of buffer size, for debugging purposes
type BufferSize struct {
	GapUsed, GapGap uintptr
	Words           uintptr
	Undo            uintptr
}

func NewBuffer(dir, name string, create bool, indentchar string, hl hl.Highlighter) (b *Buffer, err error) {
	//println("NewBuffer call:", dir, name)
	b = &Buffer{
		Dir:           dir,
		Name:          name,
		Editable:      true,
		EditableStart: -1,
		Modified:      false,

		RevCount: 0,

		Hl: hl,

		buf:   make([]rune, SLOP),
		gap:   0,
		gapsz: SLOP,

		Markat: -1,

		ul: undoList{0, []undoInfo{}, true}}

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
		flag := ReloadFlag(0)
		if create {
			flag |= ReloadCreate
		}
		err := b.Reload(flag)
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

	b.sels = []*util.Sel{}

	return b, nil
}

func (b *Buffer) AddSel(sel *util.Sel) {
	for i := range b.sels {
		if b.sels[i] == nil {
			b.sels[i] = sel
			return
		}
	}
	b.sels = append(b.sels, sel)
}

func (b *Buffer) RmSel(sel *util.Sel) {
	for i := range b.sels {
		if b.sels[i] == sel {
			b.sels[i] = nil
			break
		}
	}
}

type ReloadFlag uint8

const (
	ReloadCreate ReloadFlag = 1 << iota
	ReloadPreserveCurlineWhitespace
)

// (re)loads buffer from disk
func (b *Buffer) Reload(flags ReloadFlag) error {
	create := flags&ReloadCreate != 0
	path := filepath.Join(b.Dir, b.Name)
	infile, err := os.Open(path)
	if err == nil {
		defer infile.Close()

		fi, err := infile.Stat()
		if err != nil {
			return fmt.Errorf("Couldn't stat file (after opening it?) %s", path)
		}

		if fi.IsDir() {
			return b.reloadDir(infile)
		}
		if fi.Size() > 500*1024*1024 {
			return fmt.Errorf("Refusing to open files larger than 500MB")
		}

		bytes, err := ioutil.ReadAll(infile)
		if err != nil {
			return err
		}
		if isBinary(bytes) {
			return fmt.Errorf("Can not open binary file")
		}

		restoreCurline := ""

		if len(b.sels) > 0 && b.sels[0].S == b.sels[0].E {
			s1 := b.Tonl(b.sels[0].S-1, -1)
			if s1 < b.sels[0].S {
				curline := string(b.SelectionRunes(util.Sel{s1, b.sels[0].S}))
				allspace := true
				for _, ch := range curline {
					if ch != ' ' && ch != '\t' {
						allspace = false
						break
					}
				}
				if allspace {
					restoreCurline = curline
				}
			}
		}

		b.ReplaceFull([]rune(string(bytes)))

		if restoreCurline != "" {
			s1 := b.Tonl(b.sels[0].S-1, -1)
			if s1 == b.sels[0].S {
				b.Replace([]rune(restoreCurline), b.sels[0], false, nil, 0)
			}
		}

		b.modTime = fi.ModTime()
		s1 := sha1.Sum(bytes)
		b.onDiskChecksum = &s1
		b.Modified = false
		//b.ul.Reset()
		b.ul.SetSaved()

		if len(b.buf)-b.gapsz < 1*1024*1024 {
			str := string(b.SelectionRunes(util.Sel{0, b.Size()}))
			b.Words = util.Dedup(nonwdRe.Split(str, -1))
			b.WordsUpdate = time.Now()
		}
	} else {
		if create {
			// doesn't exist, mark as modified
			b.Modified = true
			b.ul.nilIsSaved = false
			b.modTime = time.Now()
		} else {
			return fmt.Errorf("File doesn't exist: %s", path)
		}
	}

	return nil
}

func isBinary(bytes []byte) bool {
	testb := bytes
	if len(testb) > 1024 {
		testb = testb[:1024]
	}
	good := 0
	bad := 0
	for len(testb) > 0 {
		r, size := utf8.DecodeRune(testb)
		testb = testb[size:]
		if r == utf8.RuneError || r == 0 {
			bad++
		} else {
			good++
		}
	}
	if bad == 0 {
		return false
	}
	return bad > good
}

func (b *Buffer) reloadDir(fh *os.File) error {
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
		if config.HideHidden && (len(n) <= 0 || n[0] == '.') {
			continue
		}
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

	b.Replace([]rune(strings.Join(r, "\t")), &util.Sel{0, b.Size()}, true, nil, 0)

	b.Modified = false
	return nil
}

func (b *Buffer) ReplaceFull(text []rune) {
	saveSels := b.saveSels()
	b.Replace(text, &util.Sel{0, b.Size()}, true, nil, 0)
	b.restoreSels(saveSels)
}

// Replaces text between sel.S and sel.E with text, updates sels AND sel accordingly
// After the replacement the highlighter is restarted
func (b *Buffer) Replace(text []rune, sel *util.Sel, solid bool, eventChan chan string, origin util.EventOrigin) {
	if !b.Editable {
		return
	}

	b.FixSel(sel)
	if sel.S < 0 {
		sel.S = sel.E
	}
	if sel.S < b.EditableStart {
		sel.S = b.EditableStart
		sel.E = b.EditableStart
		return
	}

	b.wrlock()

	b.Modified = true

	osel := *sel

	b.pushUndo(*sel, text, solid)
	b.replaceIntl(text, sel)
	b.updateSels(sel, len(text))

	b.unlock()

	sel.S = sel.S + len(text)
	sel.E = sel.S

	b.generateEvent(text, osel, eventChan, origin)

	if origin == util.EO_FILES {
		b.updcount += len(text)
		if b.updcount > 2*1024 {
			b.updcount = 0
			b.UpdateWords()
		}
	}
}

func (b *Buffer) generateEvent(text []rune, sel util.Sel, eventChan chan string, origin util.EventOrigin) {
	if eventChan == nil {
		return
	}

	if sel.S != sel.E {
		util.FmteventBase(eventChan, origin, b.Name == "+Tag", util.ET_BODYDEL, sel.S, sel.E, "", func() {})
	}

	if (sel.S == sel.E) || (len(text) != 0) {
		util.FmteventBase(eventChan, origin, b.Name == "+Tag", util.ET_BODYINS, sel.S, sel.S, string(text), func() {})
	}
}

// Undo last change. Redoes last undo if redo == true
func (b *Buffer) Undo(sel *util.Sel, redo bool) {
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
		b.replaceIntl(text, &ws)
		b.updateSels(&ws, len(text))

		b.unlock()

		sel.S = ws.S
		sel.E = ws.S + len(text)

		mui := ui
		if !redo {
			mui = b.ul.PeekUndo()
		}

		if mui == nil {
			b.Modified = !b.ul.nilIsSaved
		} else {
			b.Modified = !mui.saved
		}

		if !redo {
			if ui.solid {
				return
			}
		}
	}
}

func (b *Buffer) LastEdit() time.Time {
	ui := b.ul.PeekUndo()
	if ui == nil {
		return time.Unix(0, 0)
	} else {
		return ui.ts
	}
}

func (b *Buffer) LastEditIsType(rev int) int {
	if rev != b.RevCount-1 {
		return -1
	}
	ui := b.ul.PeekUndo()
	if ui == nil {
		return -1
	}
	d := b.RevCount - ui.rev
	if (ui.before.S != ui.before.E) || (ui.before.S != ui.after.S) || (ui.after.S != ui.after.E-d) || (strings.Index(ui.after.text, "\n") >= 0) {
		return -1
	}
	return ui.after.E - 1
}

func (b *Buffer) Rdlock() {
	b.lock.RLock()
}

func (b *Buffer) Rdunlock() {
	b.lock.RUnlock()
}

func (b *Buffer) wrlock() {
	b.lock.RLock()
	b.lock.RUnlock()
	b.lock.Lock()
}

func (b *Buffer) unlock() {
	b.lock.Unlock()
}

// Replaces text between sel.S and sel.E with text, updates selections in sels except sel itself
// NOTE sel IS NOT modified, we need a pointer specifically so we can skip updating it in sels
func (b *Buffer) replaceIntl(text []rune, sel *util.Sel) {
	regionSize := sel.E - sel.S

	if sel.S != sel.E {
		b.updateSels(sel, -regionSize)
		b.MoveGap(sel.S)
		b.gapsz += regionSize // this effectively deletes the current selection
	} else {
		b.MoveGap(sel.S)
	}

	b.IncGap(len(text))
	for i, r := range text {
		b.buf[b.gap+i] = r
	}
	b.gap += len(text)
	b.gapsz -= len(text)
	b.Hl.Alter(sel.S - 1)

	b.RevCount++
}

// Saves undo information for replacement of text between sel.S and sel.E with text
func (b *Buffer) pushUndo(sel util.Sel, text []rune, solid bool) {
	var ui undoInfo
	ui.rev = b.RevCount
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
func (b *Buffer) updateSels(sel *util.Sel, delta int) {
	var end int
	if delta < 0 {
		end = sel.S - delta
	} else {
		end = -1
	}

	s := b.Size()

	sels := b.sels
	for i := range sels {
		if sels[i] == nil {
			continue
		}
		if sels[i] == sel {
			continue
		}

		if (sels[i].S >= sel.S) && (sels[i].S <= end) {
			sels[i].S = sel.S
		} else if sels[i].S > sel.S {
			sels[i].S += delta
		}

		if (sels[i].E >= sel.S) && (sels[i].E <= end) {
			sels[i].E = sel.S
		} else if sels[i].E > sel.S {
			sels[i].E += delta
		}

		if sels[i].E > s {
			sels[i].E = s
		}
		if sels[i].S > s {
			sels[i].S = s
		}
	}
}

// Increases the size of the gap to fit at least delta more items
func (b *Buffer) IncGap(delta int) {
	if b.gapsz > delta+1 {
		return
	}

	ngapsz := (delta/SLOP + 1) * SLOP

	nbuf := make([]rune, len(b.buf)-b.gapsz+ngapsz)

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

func (b *Buffer) At(p int) rune {
	pp := b.phisical(p)
	if (pp < 0) || (pp >= len(b.buf)) {
		return 0
	}
	return b.buf[pp]
}

// Returns the specified selection as two slices. The slices are to be treated as contiguous and may be empty
func (b *Buffer) Selection(sel util.Sel) ([]rune, []rune) {
	b.FixSel(&sel)
	ps := b.phisical(sel.S)
	pe := b.phisical(sel.E)

	if ps < 0 {
		ps = 0
	}
	if pe > len(b.buf) {
		pe = len(b.buf)
	}

	if (ps < b.gap) && (pe >= b.gap) {
		//println(len(b.buf), b.gap, b.gapsz, ps, pe)
		return b.buf[ps:b.gap], b.buf[b.gap+b.gapsz : pe]
	} else {
		if pe <= ps {
			return []rune{}, []rune{}
		} else {
			return b.buf[ps:pe], []rune{}
		}
	}
}

// Returns the specified selection as single slice of ColorRunes (will allocate)
func (b *Buffer) SelectionRunes(sel util.Sel) []rune {
	ba, bb := b.Selection(sel)
	r := make([]rune, 0, len(ba)+len(bb))
	r = append(r, ba...)
	r = append(r, bb...)
	return r
}

func (b *Buffer) ByteOffset(p int) int {
	n := 0
	ba, bb := b.Selection(util.Sel{0, p})
	for _, bcur := range [][]rune{ba, bb} {
		for _, ch := range bcur {
			switch {
			case ch <= 0x7f:
				n++
			case ch <= 0x7FF:
				n += 2
			case ch <= 0xFFFF:
				n += 3
			case ch <= 0x10FFFF:
				n += 4
			}
		}
	}
	return n
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
			c = ba[i]
		} else {
			c = bb[i-len(ba)]
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
func (b *Buffer) Towd(start int, dir int, dontForceAdvance bool) int {
	first := (dir < 0)
	notfirst := !first
	var i int
	for i = start; (i >= 0) && (i < b.Size()); i += dir {
		c := b.At(i)
		if !(unicode.IsLetter(c) || unicode.IsDigit(c) || (c == '_')) {
			if !first && !dontForceAdvance {
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
	return b.Tof(start, dir, unicode.IsSpace)
}

// Moves to the first position where f returns true
func (b *Buffer) Tof(start int, dir int, f func(rune) bool) int {
	first := (dir < 0)
	notfirst := !first
	var i int
	for i = start; (i >= 0) && (i < b.Size()); i += dir {
		c := b.At(i)
		if f(c) {
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
		c := b.At(i)
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
	if g == 0 {
		return -1
	}

	var open rune = 0
	var close rune = 0
	if dir > 0 {
		for i := range OPEN_PARENTHESIS {
			if g == rune(OPEN_PARENTHESIS[i]) {
				open = rune(OPEN_PARENTHESIS[i])
				close = rune(CLOSED_PARENTHESIS[i])
				break
			}
		}

	} else {
		for i := range CLOSED_PARENTHESIS {
			if g == rune(CLOSED_PARENTHESIS[i]) {
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
		if g == 0 {
			return -1
		}
		if g == open {
			depth++
		}
		if g == close {
			depth--
		}
		if depth == 0 {
			return i
		}
	}
	return -1
}

func (b *Buffer) ShortName() string {
	return util.ShortPath(filepath.Join(b.Dir, b.Name), true)
}

func (b *Buffer) Path() string {
	return filepath.Join(b.Dir, b.Name)
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

func (b *Buffer) UpdateWords() {
	ba, bb := b.Selection(util.Sel{0, b.Size()})
	sa := string(ba)
	sb := string(bb)

	newWordsA := nonwdRe.Split(sa, -1)
	newWordsB := nonwdRe.Split(sb, -1)
	b.Words = util.Dedup(append(newWordsA[:len(newWordsA)-1], newWordsB[:len(newWordsB)-1]...))
	b.WordsUpdate = time.Now()
}

func (b *Buffer) Put() error {
	out, err := os.OpenFile(filepath.Join(b.Dir, b.Name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer out.Close()

	ba, bb := b.Selection(util.Sel{0, b.Size()})

	b.UpdateWords()

	bout := bufio.NewWriter(out)
	h := sha1.New()

	for _, runes := range [][]rune{ba, bb} {
		x := []byte(string(runes))
		_, err = bout.Write(x)
		if err != nil {
			return err
		}
		h.Write(x)
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

	b.modTime = fi.ModTime()
	var hbytes [sha1.Size]byte
	h.Sum(hbytes[:0])
	b.onDiskChecksum = &hbytes
	b.ul.SetSaved()

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
	if cr != 0 {
		return cr, sizeOfRune(cr), nil
	} else {
		return 0, 0, io.EOF
	}
}

func (b *Buffer) GetLine(i int, utf16col bool) (int, int) {
	if i > b.Size() {
		println("GetLine Error:", i, b.Size())
		return 0, 0
	}
	ba, bb := b.Selection(util.Sel{0, b.Size()})
	if i < len(ba) {
		n, c := countNl(ba[:i], utf16col)
		return n + 1, c
	} else {
		di := i - len(ba)
		na, offa := countNl(ba, utf16col)
		nb, offb := countNl(bb[:di], utf16col)
		if nb == 0 {
			return na + 1, offa + offb
		} else {
			return na + nb + 1, offb
		}
	}
}

func (b *Buffer) CanSave() bool {
	path := filepath.Join(b.Dir, b.Name)
	fi, err := os.Stat(path)
	if err != nil {
		return true
	}

	if fi.ModTime().Sub(b.modTime) > 0 {
		if b.onDiskChecksum != nil {
			bytes, err := ioutil.ReadFile(path)
			if err == nil {
				hbytes := sha1.Sum(bytes)
				if *b.onDiskChecksum == hbytes {
					return true
				}
			}
		}
		return false
	} else {
		return true
	}
}

func countNl(rs []rune, utf16col bool) (int, int) {
	count := 0
	off := 0
	for _, r := range rs {
		if r == '\n' {
			count++
			off = 0
		} else {
			if utf16col {
				if r <= 0xFFFF {
					off++
				} else {
					off += 2
				}
			} else {
				off++
			}
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

func (b *Buffer) UndoWhere() int {
	return b.ul.cur
}

func (b *Buffer) Sels() []*util.Sel {
	return b.sels
}

func (b *Buffer) IsDir() bool {
	return b.Name[len(b.Name)-1] == '/'
}

func (b *Buffer) UndoReset() {
	b.ul.Reset()
}

type savedSels struct {
	sels      []util.Sel
	spacehack bool
}

const (
	maxSpaceHackSize = 100 * 1024
	spaceHackLineOff = 20
)

func init() {
	if (1 << spaceHackLineOff) <= maxSpaceHackSize {
		panic("bad configuration")
	}
}

func spaceHackAdvance(ch rune, linecol *int) {
	switch ch {
	case '\n':
		*linecol += (1 << spaceHackLineOff)
		*linecol = *linecol &^ ((1 << spaceHackLineOff) - 1)
	case ' ', '\r', '\t':
		// nothing
	default:
		(*linecol)++
	}
}

func (b *Buffer) saveSels() savedSels {
	r := []util.Sel{}
	sels := b.sels
	for i := range b.sels {
		if sels[i] != nil {
			r = append(r, *sels[i])
		}
	}
	if b.Size() > maxSpaceHackSize {
		return savedSels{r, false}
	}

	pos2nonspc := make(map[int]int)
	for _, sel := range r {
		pos2nonspc[sel.S] = -1
		pos2nonspc[sel.E] = -1
	}

	linecol := 0
	for i := 0; i < b.Size(); i++ {
		if _, ok := pos2nonspc[i]; ok {
			pos2nonspc[i] = linecol
		}
		spaceHackAdvance(b.At(i), &linecol)
	}

	for i := range r {
		if pos2nonspc[r[i].S] >= 0 && pos2nonspc[r[i].E] >= 0 {
			r[i].S = pos2nonspc[r[i].S]
			r[i].E = pos2nonspc[r[i].E]
		}
	}

	return savedSels{r, true}
}

func (b *Buffer) restoreSels(ssels savedSels) {
	if ssels.spacehack {
		linecol2pos := make(map[int]int)

		for _, sel := range ssels.sels {
			linecol2pos[sel.S] = -1
			linecol2pos[sel.E] = -1
		}

		linecol := 0
		for i := 0; i < b.Size(); i++ {
			if _, ok := linecol2pos[linecol]; ok && linecol2pos[linecol] < 0 {
				linecol2pos[linecol] = i
			}
			if b.At(i) == '\n' {
				linecol2pos[linecol|((1<<spaceHackLineOff)-1)] = i
			}
			spaceHackAdvance(b.At(i), &linecol)
		}

		for i := range ssels.sels {
			if linecol2pos[ssels.sels[i].S] >= 0 && linecol2pos[ssels.sels[i].E] >= 0 {
				ssels.sels[i].S = linecol2pos[ssels.sels[i].S]
				ssels.sels[i].E = linecol2pos[ssels.sels[i].E]
			} else {
				sline := ssels.sels[i].S | ((1 << spaceHackLineOff) - 1)
				eline := ssels.sels[i].E | ((1 << spaceHackLineOff) - 1)
				if linecol2pos[sline] >= 0 && linecol2pos[eline] >= 0 {
					ssels.sels[i].S = linecol2pos[sline]
					ssels.sels[i].E = linecol2pos[eline]
				} else {
					ssels.sels[i].S = b.Size()
					ssels.sels[i].E = b.Size()
				}
			}
		}
	}

	k := 0
	for i := range b.sels {
		if b.sels[i] != nil {
			b.FixSel(&ssels.sels[k])
			*b.sels[i] = ssels.sels[k]
			b.FixSel(b.sels[i])
			k++
		}
	}
}

func (b *Buffer) FlushUndo() {
	b.ul.cur = 0
	b.ul.lst = b.ul.lst[0:0]
}

func (b *Buffer) LastTypePos() int {
	j := -1

	for i := b.ul.cur - 1; i >= 0; i-- {
		if j < 0 {
			j = i
		}

		if b.ul.lst[j].ts.Sub(b.ul.lst[i].ts) > 10*time.Second {
			break
		}

		j = i
	}

	if j < 0 {
		if b.EditableStart < 0 {
			return 0
		}
		return b.EditableStart
	}

	start := b.ul.lst[j].before.S

	for i := j + 1; i < b.ul.cur; i++ {
		if start == b.ul.lst[i].before.E {
			start = b.ul.lst[i].before.S
		}
	}

	if start < 0 || start >= b.Size() {
		return 0
	}
	return start
}

func (buf *Buffer) BytesSize() (r BufferSize) {
	r.GapUsed = uintptr((cap(buf.buf) - buf.gapsz) * 6)
	r.GapGap = uintptr(buf.gapsz * 6)
	for i := range buf.Words {
		r.Words += uintptr(len(buf.Words[i]))
	}
	r.Undo = uintptr(cap(buf.ul.lst) * (8 + (8 * 3) + (8 * 3) + 16 + 2))
	for i := range buf.ul.lst {
		r.Undo += uintptr(len(buf.ul.lst[i].before.text) + len(buf.ul.lst[i].after.text))
	}
	return
}

func (buf *Buffer) Highlight(start, end int) []uint8 {
	buf.hlbuf = buf.Hl.Highlight(start, end, buf, buf.hlbuf[:0])
	return buf.hlbuf
}

func (buf *Buffer) Toregend(start int) int {
	return buf.Hl.Toregend(start, buf)
}
