package edit

import (
	"fmt"
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

	switch c.cmdch {
	case 'a':
		sel.S = sel.E
	case 'i':
		sel.E = sel.S
	}

	ec.Buf.Replace([]rune(c.txtargs[0]), &sel, ec.Buf.EditMark, ec.EventChan, util.EO_MOUSE)
	ec.Buf.EditMark = ec.Buf.EditMarkNext

	if c.cmdch == 'c' {
		*ec.atsel = sel
	}
}

func mtcmdfn(del bool, c *Cmd, ec *EditContext) {
	selfrom := c.rangeaddr.Eval(ec.Buf, *ec.atsel)
	selto := c.argaddr.Eval(ec.Buf, *ec.atsel).E

	txt := ec.Buf.SelectionRunes(selfrom)

	if selto > selfrom.E {
		ec.Buf.Replace(txt, &util.Sel{selto, selto}, ec.Buf.EditMark, ec.EventChan, util.EO_MOUSE)
		ec.Buf.Replace([]rune{}, &selfrom, false, ec.EventChan, util.EO_MOUSE)
		ec.Buf.EditMark = ec.Buf.EditMarkNext
	} else {
		ec.Buf.Replace([]rune{}, &selfrom, ec.Buf.EditMark, ec.EventChan, util.EO_MOUSE)
		ec.Buf.Replace(txt, &util.Sel{selto, selto}, false, ec.EventChan, util.EO_MOUSE)
		ec.Buf.EditMark = ec.Buf.EditMarkNext
	}
}

func pcmdfn(c *Cmd, ec *EditContext) {
	*ec.atsel = c.rangeaddr.Eval(ec.Buf, *ec.atsel)
	txt := ec.Buf.SelectionRunes(*ec.atsel)
	Warnfn(string(txt))
}

func eqcmdfn(c *Cmd, ec *EditContext) {
	*ec.atsel = c.rangeaddr.Eval(ec.Buf, *ec.atsel)

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
			ec.Buf.Replace(realSubs, &sel, first, ec.EventChan, util.EO_MOUSE)
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
	for src := 0; src < len(subs); src++ {
		if (subs[src] == '\\') && (src+1 < len(subs)) {
			switch subs[src+1] {
			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				if r == nil {
					initR(src)
				}
				n := int(subs[src+1] - '0')
				if 2*n+1 < len(loc) {
					r = append(r, b.SelectionRunes(util.Sel{loc[2*n], loc[2*n+1]})...)
				} else {
					panic(fmt.Errorf("Nonexistent backreference %d (%d)", n, len(loc)))
				}
				src++
			case '\\':
				if r == nil {
					initR(src)
				}
				r = append(r, '\\')
				src++
			default:
				//do nothing
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

func xcmdfn(c *Cmd, ec *EditContext) {
	*ec.atsel = c.rangeaddr.Eval(ec.Buf, *ec.atsel)
	var xAddrs0 = util.Sel{ec.atsel.S, ec.atsel.E}
	var xAddrs1 = util.Sel{ec.atsel.S, ec.atsel.S}
	var xAddrs2 = util.Sel{ec.atsel.S, ec.atsel.S}
	ec.Buf.AddSel(&xAddrs0)
	ec.Buf.AddSel(&xAddrs1)
	ec.Buf.AddSel(&xAddrs2)
	ebn := ec.Buf.EditMarkNext
	ec.Buf.EditMarkNext = false
	defer func() {
		*ec.atsel = xAddrs0
		ec.Buf.EditMarkNext = ebn
		ec.Buf.EditMark = ec.Buf.EditMarkNext
		ec.Buf.RmSel(&xAddrs0)
		ec.Buf.RmSel(&xAddrs1)
		ec.Buf.RmSel(&xAddrs2)
	}()

	re := c.sregexp
	count := 0

	for {
		loc := re.Match(ec.Buf, xAddrs1.S, xAddrs0.E, +1)
		if (loc == nil) || (len(loc) < 2) {
			return
		}
		xAddrs1.S, xAddrs1.E = loc[0], loc[1]
		xAddrs2 = xAddrs1
		subec := ec.subec(ec.Buf, &xAddrs2)
		c.body.fn(c.body, &subec)
		if xAddrs1.S == xAddrs1.E {
			xAddrs1 = xAddrs2
		}
		xAddrs1.S = xAddrs1.E
		count++
		if count > LOOP_LIMIT {
			Warnfn("x/y loop seems stuck\n")
			return
		}
	}
}

func ycmdfn(c *Cmd, ec *EditContext) {
	*ec.atsel = c.rangeaddr.Eval(ec.Buf, *ec.atsel)
	var yAddrs0 = util.Sel{ec.atsel.S, ec.atsel.E}
	var yAddrs1 = util.Sel{ec.atsel.S, ec.atsel.E}
	ec.Buf.AddSel(&yAddrs0)
	ec.Buf.AddSel(&yAddrs1)
	ebn := ec.Buf.EditMarkNext
	ec.Buf.EditMarkNext = false
	defer func() {
		*ec.atsel = yAddrs0
		ec.Buf.EditMarkNext = ebn
		ec.Buf.EditMark = ec.Buf.EditMarkNext
		ec.Buf.RmSel(&yAddrs0)
		ec.Buf.RmSel(&yAddrs1)
	}()

	re := c.sregexp
	count := 0

	for {
		loc := re.Match(ec.Buf, yAddrs1.S, yAddrs0.E, +1)
		if (loc == nil) || (len(loc) < 2) {
			return
		}
		yAddrs1.E = loc[0]
		subec := ec.subec(ec.Buf, &yAddrs1)
		c.body.fn(c.body, &subec)
		yAddrs1.S = yAddrs1.S + (loc[1] - loc[0])
		yAddrs1.E = yAddrs1.S
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
	if doit {
		c.body.fn(c.body, ec)
	}
}

func pipeincmdfn(c *Cmd, ec *EditContext) {
	resultChan := make(chan string)
	NewJob(ec.Buf.Dir, c.bodytxt, "", ec.Buf, resultChan)
	str := <-resultChan
	*ec.atsel = c.rangeaddr.Eval(ec.Buf, *ec.atsel)
	ec.Buf.Replace([]rune(str), ec.atsel, ec.Buf.EditMark, ec.EventChan, util.EO_MOUSE)
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
	ec.Buf.Replace([]rune(str), ec.atsel, ec.Buf.EditMark, ec.EventChan, util.EO_MOUSE)
	ec.Buf.EditMark = ec.Buf.EditMarkNext
}

func kcmdfn(c *Cmd, ec *EditContext) {
	*ec.atsel = c.rangeaddr.Eval(ec.Buf, *ec.atsel)
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

	first := true

	for i := range fileNames {
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

	for i := range fileNames {
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

	c.bodytxt = strings.TrimSpace(c.bodytxt)

	bytes, err := ioutil.ReadFile(util.ResolvePath(ec.Buf.Dir, c.bodytxt))
	if err != nil {
		Warnfn(fmt.Sprintf("Couldn't read file: %v\n", err))
	}

	ec.Buf.Replace([]rune(string(bytes)), ec.atsel, ec.Buf.EditMark, ec.EventChan, util.EO_MOUSE)
	ec.Buf.EditMark = ec.Buf.EditMarkNext

	if all {
		ec.atsel.S = 0
		ec.atsel.E = 0
	}
}

func wcmdfn(c *Cmd, ec *EditContext) {
	*ec.atsel = c.rangeaddr.Eval(ec.Buf, *ec.atsel)
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
		subec := ec.subec(buffers[i].Buffer, buffers[i].Sel)
		c.body.fn(c.body, &subec)
		ec.BufMan.RefreshBuffer(buffers[i].Buffer)
	}
}

func blockcmdfn(c *Cmd, ec *EditContext) {
	for i := range c.mbody {
		c.mbody[i].fn(c.mbody[i], ec)
	}
}
