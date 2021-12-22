package edit

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/aarzilli/yacco/buf"
	"github.com/aarzilli/yacco/regexp"
	"github.com/aarzilli/yacco/util"
)

var Warnfn func(msg string)
var NewJob func(wd, Cmd, input string, buf *buf.Buffer, resultChan chan<- string)

const LOOP_LIMIT = 2000

func nilcmdfn(c *Cmd, ec *EditContext) {
	*ec.atsel = c.rangeaddr.Eval(ec.Buf, *ec.atsel)
}

func inscmdfn(dir int, c *Cmd, ec *EditContext) {
	sel := c.rangeaddr.Eval(ec.Buf, *ec.atsel)

	ec.tracecmd(sel, c)

	switch c.cmdch {
	case 'a':
		sel.S = sel.E
	case 'i':
		sel.E = sel.S
	}

	ec.replace([]rune(c.txtargs[0]), &sel, ec.Buf.EditMark)
	ec.Buf.EditMark = ec.Buf.EditMarkNext

	if c.cmdch == 'c' {
		*ec.atsel = sel
	}
}

func mtcmdfn(del bool, c *Cmd, ec *EditContext) {
	selfrom := c.rangeaddr.Eval(ec.Buf, *ec.atsel)
	selto := c.argaddr.Eval(ec.Buf, *ec.atsel).E

	txt := ec.Buf.SelectionRunes(selfrom)

	ec.tracecmd(selfrom, c, selto)

	if selto > selfrom.E {
		ec.replace(txt, &util.Sel{selto, selto}, ec.Buf.EditMark)
		ec.replace([]rune{}, &selfrom, false)
		ec.Buf.EditMark = ec.Buf.EditMarkNext
	} else {
		ec.replace([]rune{}, &selfrom, ec.Buf.EditMark)
		ec.replace(txt, &util.Sel{selto, selto}, false)
		ec.Buf.EditMark = ec.Buf.EditMarkNext
	}
}

func pcmdfn(c *Cmd, ec *EditContext) {
	*ec.atsel = c.rangeaddr.Eval(ec.Buf, *ec.atsel)
	txt := ec.Buf.SelectionRunes(*ec.atsel)
	ec.tracecmd(*ec.atsel, c)
	Warnfn(string(txt))
}

func eqcmdfn(c *Cmd, ec *EditContext) {
	*ec.atsel = c.rangeaddr.Eval(ec.Buf, *ec.atsel)
	ec.tracecmd(*ec.atsel, c)

	var charpos bool

	switch c.bodytxt {
	case "#":
		charpos = true
	case "":
		charpos = false
	default:
		Warnfn("Wrong argument to =")
	}

	if charpos {
		if ec.atsel.S == ec.atsel.E {
			Warnfn(fmt.Sprintf("%s:#%d\n", ec.Buf.Path(), ec.atsel.S))
		} else {
			Warnfn(fmt.Sprintf("%s:#%d,#%d\n", ec.Buf.Path(), ec.atsel.S, ec.atsel.E))
		}
	} else {
		sln, _ := ec.Buf.GetLine(ec.atsel.S, false)
		eln, _ := ec.Buf.GetLine(ec.atsel.E, false)
		if ec.atsel.S == ec.atsel.E {
			Warnfn(fmt.Sprintf("%s:%d\n", ec.Buf.Path(), sln))
		} else {
			Warnfn(fmt.Sprintf("%s:%d,%d\n", ec.Buf.Path(), sln, eln))
		}
	}
}

func scmdfn(c *Cmd, ec *EditContext) {
	sel := c.rangeaddr.Eval(ec.Buf, *ec.atsel)
	var addrSave = util.Sel{sel.S, sel.E}
	ec.Buf.AddSel(&addrSave)
	defer func() {
		ec.Buf.RmSel(&addrSave)
		*ec.atsel = addrSave
		ec.Buf.EditMark = ec.Buf.EditMarkNext
	}()

	ec.tracecmd(*ec.atsel, c, "{")
	defer ec.tracecmd("}")

	re := c.sregexp
	subs := []rune(c.txtargs[1])
	first := ec.Buf.EditMark
	count := 0
	nmatch := 1
	globalrepl := (c.numarg == 0) || (c.flags&G_FLAG != 0)
	for {
		psel := sel.S
		loc := re.Match(ec.Buf, sel.S, addrSave.E, +1)
		if (loc == nil) || (len(loc) < 2) || loc[0] >= addrSave.E {
			return
		}
		sel = util.Sel{loc[0], loc[1]}
		allWhitespace := false
		if globalrepl || (c.numarg == nmatch) {
			realSubs := resolveBackreferences(subs, ec.Buf, loc)
			ec.tracemore("replace", sel, realSubs)
			ec.replace(realSubs, &sel, first)
			allWhitespace = isWhitespace(realSubs)
			if !globalrepl {
				break
			}
		} else {
			sel.S = sel.E
		}
		nmatch++

		if loc[0] == loc[1] && allWhitespace {
			sel.S++
			sel.E = sel.S
		}

		if sel.S == psel {
			count++
		} else {
			count = 0
		}
		if count > 100 {
			panic("s Loop got stuck")
		}
		first = false
	}
}

func isWhitespace(r []rune) bool {
	for i := range r {
		switch r[i] {
		case ' ', '\t':
			//ok
		default:
			return false
		}
	}
	return true
}

func resolveBackreferences(subs []rune, b *buf.Buffer, loc []int) []rune {
	var r []rune = nil
	initR := func(src int) {
		r = make([]rune, src, len(subs))
		copy(r, subs[:src])
	}
	replace := func(src, n int) {
		if r == nil {
			initR(src)
		}
		if 2*n+1 < len(loc) {
			r = append(r, b.SelectionRunes(util.Sel{loc[2*n], loc[2*n+1]})...)
		} else {
			panic(fmt.Errorf("Nonexistent backreference %d (%d)", n, len(loc)))
		}
	}
	for src := 0; src < len(subs); src++ {
		if (subs[src] == '\\') && (src+1 < len(subs)) {
			switch subs[src+1] {
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				n := int(subs[src+1] - '0')
				replace(src, n)
				src++
			default:
				if r == nil {
					initR(src)
				}
				r = append(r, subs[src+1])
				src++
			}
		} else if r != nil {
			r = append(r, subs[src])
		}
	}
	if r != nil {
		return r
	}
	return subs
}

func xycmdfn(c *Cmd, ec *EditContext, inv bool) {
	*ec.atsel = c.rangeaddr.Eval(ec.Buf, *ec.atsel)
	rngsel := *ec.atsel

	ec.tracecmd(*ec.atsel, c, "{")

	re := c.sregexp
	count := 0
	stash := []replaceOp{}

	for {
		ec.tracemore("searching regex at", rngsel)
		loc := re.Match(ec.Buf, rngsel.S, rngsel.E, +1)
		if (loc == nil) || (len(loc) < 2) {
			ec.tracecmd("}")
			ec.applyStash(stash)
			return
		}
		var cursel util.Sel
		if inv {
			cursel = util.Sel{rngsel.S, loc[0]}
		} else {
			cursel = util.Sel{loc[0], loc[1]}
		}
		ec.tracemore("match at", util.Sel{loc[0], loc[1]})
		subec := ec.subec(ec.Buf, &cursel)
		subec.stash = &stash
		c.body.fn(c.body, &subec)
		rngsel.S = loc[1]
		count++
		if count > LOOP_LIMIT {
			Warnfn("x/y loop seems stuck\n")
			return
		}
	}
}

type gcmdFlags uint8

const (
	gcmdInv gcmdFlags = 1 << iota
	gcmdFullMatch
)

func gcmdfn(flags gcmdFlags, c *Cmd, ec *EditContext) {
	inv := flags&gcmdInv != 0
	*ec.atsel = c.rangeaddr.Eval(ec.Buf, *ec.atsel)
	re := c.sregexp
	loc := re.Match(ec.Buf, ec.atsel.S, ec.atsel.E, +1)
	doit := loc != nil
	if flags&gcmdFullMatch != 0 {
		if doit {
			doit = (loc[0] == ec.atsel.S) && (loc[1] == ec.atsel.E)
		}
	}
	if inv {
		doit = !doit
	}
	ec.tracecmd(*ec.atsel, c)
	if doit {
		ec.tracecmd("{")
		ec.depth++
		c.body.fn(c.body, ec)
		ec.depth--
		ec.tracecmd("}")
	}
}

func pipeincmdfn(c *Cmd, ec *EditContext) {
	resultChan := make(chan string)
	NewJob(ec.Buf.Dir, c.bodytxt, "", ec.Buf, resultChan)
	str := <-resultChan
	*ec.atsel = c.rangeaddr.Eval(ec.Buf, *ec.atsel)
	ec.tracecmd(*ec.atsel, c, str)
	ec.replace([]rune(str), ec.atsel, ec.Buf.EditMark)
	ec.Buf.EditMark = ec.Buf.EditMarkNext
}

func pipeoutcmdfn(c *Cmd, ec *EditContext) {
	*ec.atsel = c.rangeaddr.Eval(ec.Buf, *ec.atsel)
	str := string(ec.Buf.SelectionRunes(*ec.atsel))
	NewJob(ec.Buf.Dir, c.bodytxt, str, ec.Buf, nil)
}

func pipecmdfn(c *Cmd, ec *EditContext) {
	*ec.atsel = c.rangeaddr.Eval(ec.Buf, *ec.atsel)
	str := string(ec.Buf.SelectionRunes(*ec.atsel))
	resultChan := make(chan string)
	NewJob(ec.Buf.Dir, c.bodytxt, str, ec.Buf, resultChan)
	str = <-resultChan
	ec.tracecmd(*ec.atsel, c, str)
	ec.replace([]rune(str), ec.atsel, ec.Buf.EditMark)
	ec.Buf.EditMark = ec.Buf.EditMarkNext
}

func kcmdfn(c *Cmd, ec *EditContext) {
	*ec.atsel = c.rangeaddr.Eval(ec.Buf, *ec.atsel)
	ec.tracecmd(*ec.atsel, c)
	if ec.Sel != nil {
		*ec.Sel = *ec.atsel
	}
}

func Mcmdfn(c *Cmd, ec *EditContext) {
	var p int

	if (*ec.atsel).S == ec.Buf.Markat {
		p = (*ec.atsel).E
	} else if (*ec.atsel).E == ec.Buf.Markat {
		p = (*ec.atsel).S
	} else {
		// arbitrary
		p = (*ec.atsel).E
		ec.Buf.Markat = (*ec.atsel).S
	}

	ms := c.rangeaddr.Eval(ec.Buf, util.Sel{p, p})

	ec.tracecmd(ms, c)

	if ms.S != ms.E {
		panic("M command called on non-empty selection")
	}

	if ms.S > ec.Buf.Markat {
		(*ec.atsel).S = ec.Buf.Markat
		(*ec.atsel).E = ms.S
	} else {
		(*ec.atsel).S = ms.S
		(*ec.atsel).E = ec.Buf.Markat
	}
}

func bcmdfn(openOthers bool, c *Cmd, ec *EditContext) {
	fileNames := processFileList(ec, c.bodytxt)
	open := bufferMap(ec)

	ec.tracecmd(*ec.atsel, c)

	first := true

	for i := range fileNames {
		ec.tracemore("opening", fileNames[i])
		buf, ok := open[fileNames[i]]

		if openOthers && !ok {
			buf = ec.BufMan.Open(fileNames[i])
		}

		if buf != nil && first {
			nec := ec.subec(buf, &util.Sel{0, 0})
			ec = &nec
			first = false
			if !openOthers {
				break
			}
		}
	}
}

func Dcmdfn(c *Cmd, ec *EditContext) {
	fileNames := processFileList(ec, c.bodytxt)
	open := bufferMap(ec)

	ec.tracecmd(*ec.atsel, c)

	for i := range fileNames {
		ec.tracemore("closing", fileNames[i])
		buf, ok := open[fileNames[i]]
		if ok {
			ec.BufMan.Close(buf)
		}
	}
}

func processFileList(ec *EditContext, body string) []string {
	body = strings.TrimSpace(body)
	var fileNames []string

	if body[0] == '<' {
		resultChan := make(chan string)
		NewJob(ec.Buf.Dir, body[1:], "", ec.Buf, resultChan)
		body = <-resultChan
		fileNames = util.QuotedSplit(body)
	} else {
		sources := util.QuotedSplit(body)
		fileNames = make([]string, 0, len(sources))
		for i := range sources {
			files, err := filepath.Glob(sources[i])
			if err == nil {
				fileNames = append(fileNames, files...)
			}
		}
	}

	for i := range fileNames {
		fileNames[i] = util.ResolvePath(ec.Buf.Dir, fileNames[i])
	}

	return fileNames
}

func bufferMap(ec *EditContext) map[string]*buf.Buffer {
	open := map[string]*buf.Buffer{}
	buffers := ec.BufMan.List()
	for i := range buffers {
		open[buffers[i].Buffer.Path()] = buffers[i].Buffer
	}
	return open
}

func extreplcmdfn(all bool, c *Cmd, ec *EditContext) {
	if all {
		ec.atsel.S = 0
		ec.atsel.E = ec.Buf.Size()
	} else {
		*ec.atsel = c.rangeaddr.Eval(ec.Buf, *ec.atsel)
	}

	ec.tracecmd(c)

	c.bodytxt = strings.TrimSpace(c.bodytxt)

	bytes, err := ioutil.ReadFile(util.ResolvePath(ec.Buf.Dir, c.bodytxt))
	if err != nil {
		Warnfn(fmt.Sprintf("Couldn't read file: %v\n", err))
	}

	ec.replace([]rune(string(bytes)), ec.atsel, ec.Buf.EditMark)
	ec.Buf.EditMark = ec.Buf.EditMarkNext

	if all {
		ec.atsel.S = 0
		ec.atsel.E = 0
	}
}

func wcmdfn(c *Cmd, ec *EditContext) {
	*ec.atsel = c.rangeaddr.Eval(ec.Buf, *ec.atsel)
	ec.tracecmd(*ec.atsel, c)
	if ec.atsel.S == ec.atsel.E {
		ec.atsel.S = 0
		ec.atsel.E = ec.Buf.Size()
	}
	c.bodytxt = strings.TrimSpace(c.bodytxt)
	str := []byte(string(ec.Buf.SelectionRunes(*ec.atsel)))
	err := ioutil.WriteFile(util.ResolvePath(ec.Buf.Dir, c.bodytxt), str, 0666)
	if err != nil {
		Warnfn(fmt.Sprintf("Couldn't write file: %v\n", err))
	}
}

func XYcmdfn(inv bool, c *Cmd, ec *EditContext) {
	buffers := ec.BufMan.List()

	ec.tracecmd(c, "{")
	defer ec.tracecmd("}")

	matchbuffers := make([]BufferManagingEntry, 0, len(buffers))

	for i := range buffers {
		p := []rune(buffers[i].Buffer.Path())
		loc := c.sregexp.Match(regexp.RuneArrayMatchable(p), 0, len(p), +1)
		match := loc != nil
		if inv {
			match = !match
		}
		if match {
			matchbuffers = append(matchbuffers, buffers[i])
		}
	}

	if len(matchbuffers) == 0 {
		Warnfn("No match")
		return
	}

	if c.txtargdelim == '"' && len(matchbuffers) != 1 {
		Warnfn("Too many matches")
		return
	}

	for i := range matchbuffers {
		ec.tracemore("executing on", matchbuffers[i].Buffer.Path())
		subec := ec.subec(matchbuffers[i].Buffer, matchbuffers[i].Sel)
		c.body.fn(c.body, &subec)
		ec.BufMan.RefreshBuffer(matchbuffers[i].Buffer)
	}
}

func blockcmdfn(c *Cmd, ec *EditContext) {
	sel := c.rangeaddr.Eval(ec.Buf, *ec.atsel)
	stash := []replaceOp{}

	ec.tracecmd(sel, c)
	defer ec.tracecmd("}")

	for i := range c.mbody {
		subec := ec.subec(ec.Buf, &sel)
		subec.stash = &stash
		c.mbody[i].fn(c.mbody[i], &subec)
	}

	ec.applyStash(stash)
}

type replaceOp struct {
	text []rune
	sel  util.Sel
}

func (ec *EditContext) replace(text []rune, sel *util.Sel, solid bool) {
	if ec.stash != nil {
		*ec.stash = append(*ec.stash, replaceOp{append([]rune{}, text...), *sel})
	} else {
		ec.Buf.Replace(text, sel, solid, ec.EventChan, util.EO_MOUSE)
	}
}

func (ec *EditContext) applyStash(stash []replaceOp) {
	if ec.stash != nil {
		*ec.stash = append(*ec.stash, stash...)
		return
	}

	curpos := 0
	delta := 0

	for i := range stash {
		stash[i].sel.S += delta
		stash[i].sel.E += delta
		if stash[i].sel.S < curpos {
			// discarded, not in sequence
			continue
		}

		delta += len(stash[i].text) - (stash[i].sel.E - stash[i].sel.S)

		ec.Buf.Replace(stash[i].text, &stash[i].sel, i == 0, ec.EventChan, util.EO_MOUSE)
		curpos = stash[i].sel.E
	}
}

func (ec *EditContext) printSel(w io.Writer, sel util.Sel) {
	fmt.Fprintf(w, "%d,%d", sel.S, sel.E)
	dot := false
	if sel.E-sel.S > 20 {
		sel.E = sel.S + 20
		dot = true
	}
	txt := ec.Buf.SelectionRunes(sel)
	fmt.Fprintf(w, " %q", string(txt))
	if dot {
		fmt.Fprintf(w, "...")
	}
}

func (ec *EditContext) tracecmd(args ...interface{}) {
	if !ec.Trace {
		return
	}
	var buf bytes.Buffer
	for i := 0; i < ec.depth; i++ {
		buf.Write([]byte("   "))
	}

	first := true
	for _, arg := range args {
		if !first {
			buf.WriteByte(' ')
		}
		first = false
		switch arg := arg.(type) {
		case *Cmd:
			c := arg
			fmt.Fprintf(&buf, "%c", c.cmdch)
			for i := range c.txtargs {
				fmt.Fprintf(&buf, " %q", c.txtargs[i])
			}
			if c.bodytxt != "" {
				fmt.Fprintf(&buf, " %q", c.bodytxt)
			}
		case util.Sel:
			ec.printSel(&buf, arg)
		case *util.Sel:
			ec.printSel(&buf, *arg)
		case []rune:
			fmt.Fprintf(&buf, "%q", string(arg))
		default:
			fmt.Fprintf(&buf, "%v", arg)
		}
	}
	buf.WriteByte('\n')
	Warnfn(buf.String())
}

func (ec *EditContext) tracemore(args ...interface{}) {
	ec.depth++
	ec.tracecmd(args...)
	ec.depth--
}
