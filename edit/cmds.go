package edit

import (
	"fmt"
	"yacco/buf"
	"yacco/util"
)

var Warnfn func(msg string)
var NewJob func(wd, cmd, input string, buf *buf.Buffer, resultChan chan<- string)

const LOOP_LIMIT = 2000

func nilcmdfn(c *cmd, atsel *util.Sel, ec EditContext) {
	*atsel = c.rangeaddr.Eval(ec.Buf, *atsel)
}

func inscmdfn(dir int, c *cmd, atsel *util.Sel, ec EditContext) {
	sel := c.rangeaddr.Eval(ec.Buf, *atsel)

	switch c.cmdch {
	case 'a':
		sel.S = sel.E
	case 'i':
		sel.E = sel.S
	}

	ec.Buf.Replace([]rune(c.txtargs[0]), &sel, ec.Buf.EditMark, ec.EventChan, util.EO_MOUSE)
	ec.Buf.EditMark = ec.Buf.EditMarkNext

	if c.cmdch == 'c' {
		*atsel = sel
	}
}

func mtcmdfn(del bool, c *cmd, atsel *util.Sel, ec EditContext) {
	selfrom := c.rangeaddr.Eval(ec.Buf, *atsel)
	selto := c.argaddr.Eval(ec.Buf, *atsel).E

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

func pcmdfn(c *cmd, atsel *util.Sel, ec EditContext) {
	*atsel = c.rangeaddr.Eval(ec.Buf, *atsel)
	txt := ec.Buf.SelectionRunes(*atsel)
	Warnfn(string(txt))
}

func eqcmdfn(c *cmd, atsel *util.Sel, ec EditContext) {
	*atsel = c.rangeaddr.Eval(ec.Buf, *atsel)
	sline, scol := ec.Buf.GetLine(atsel.S)
	eline, ecol := ec.Buf.GetLine(atsel.E)
	if (sline == eline) && (scol == ecol) {
		Warnfn(fmt.Sprintf("%d:%d", sline, scol))
	} else {
		Warnfn(fmt.Sprintf("%d:%d %d:%d", sline, scol, eline, ecol))
	}
}

func scmdfn(c *cmd, atsel *util.Sel, ec EditContext) {
	sel := c.rangeaddr.Eval(ec.Buf, *atsel)
	var addrSave = []util.Sel{{sel.S, sel.E}}
	ec.Buf.AddSels(&addrSave)
	defer func() {
		ec.Buf.RmSels(&addrSave)
		*atsel = addrSave[0]
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
		loc := re.Match(ec.Buf, sel.S, addrSave[0].E, +1)
		if (loc == nil) || (len(loc) < 2) {
			return
		}
		sel = util.Sel{loc[0], loc[1]}
		if globalrepl || (c.numarg == nmatch) {
			realSubs := resolveBackreferences(subs, ec.Buf, loc)
			ec.Buf.Replace(realSubs, &sel, first, ec.EventChan, util.EO_MOUSE)
			if !globalrepl {
				break
			}
		} else {
			sel.S = sel.E
		}
		nmatch++

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

func xcmdfn(c *cmd, atsel *util.Sel, ec EditContext) {
	*atsel = c.rangeaddr.Eval(ec.Buf, *atsel)
	var xAddrs = []util.Sel{{atsel.S, atsel.E}, {atsel.S, atsel.S}, {atsel.S, atsel.S}}
	ec.Buf.AddSels(&xAddrs)
	ebn := ec.Buf.EditMarkNext
	ec.Buf.EditMarkNext = false
	defer func() {
		*atsel = xAddrs[0]
		ec.Buf.EditMarkNext = ebn
		ec.Buf.EditMark = ec.Buf.EditMarkNext
		ec.Buf.RmSels(&xAddrs)
	}()

	re := c.sregexp
	count := 0

	for {
		loc := re.Match(ec.Buf, xAddrs[1].S, xAddrs[0].E, +1)
		if (loc == nil) || (len(loc) < 2) {
			return
		}
		xAddrs[1].S, xAddrs[1].E = loc[0], loc[1]
		xAddrs[2] = xAddrs[1]
		c.body.fn(c.body, &xAddrs[2], ec)
		if xAddrs[1].S == xAddrs[1].E {
			xAddrs[1] = xAddrs[2]
		}
		xAddrs[1].S = xAddrs[1].E
		count++
		if count > LOOP_LIMIT {
			Warnfn("x/y loop seems stuck\n")
			return
		}
	}
}

func ycmdfn(c *cmd, atsel *util.Sel, ec EditContext) {
	*atsel = c.rangeaddr.Eval(ec.Buf, *atsel)
	var yAddrs = []util.Sel{{atsel.S, atsel.E}, {atsel.S, atsel.E}}
	ec.Buf.AddSels(&yAddrs)
	ebn := ec.Buf.EditMarkNext
	ec.Buf.EditMarkNext = false
	defer func() {
		*atsel = yAddrs[0]
		ec.Buf.EditMarkNext = ebn
		ec.Buf.EditMark = ec.Buf.EditMarkNext
		ec.Buf.RmSels(&yAddrs)
	}()

	re := c.sregexp
	count := 0

	for {
		loc := re.Match(ec.Buf, yAddrs[1].S, yAddrs[0].E, +1)
		if (loc == nil) || (len(loc) < 2) {
			return
		}
		yAddrs[1].E = loc[0]
		c.body.fn(c.body, &yAddrs[1], ec)
		yAddrs[1].S = yAddrs[1].S + (loc[1] - loc[0])
		yAddrs[1].E = yAddrs[1].S
		count++
		if count > LOOP_LIMIT {
			Warnfn("x/y loop seems stuck\n")
			return
		}
	}
}

func gcmdfn(inv bool, c *cmd, atsel *util.Sel, ec EditContext) {
	*atsel = c.rangeaddr.Eval(ec.Buf, *atsel)
	re := c.sregexp
	loc := re.Match(ec.Buf, atsel.S, atsel.E, +1)
	if (loc == nil) || (loc[0] != atsel.S) || (loc[1] != atsel.E) {
		if inv {
			c.body.fn(c.body, atsel, ec)
		}
	} else {
		if !inv {
			c.body.fn(c.body, atsel, ec)
		}
	}
}

func pipeincmdfn(c *cmd, atsel *util.Sel, ec EditContext) {
	resultChan := make(chan string)
	NewJob(ec.Buf.Dir, c.bodytxt, "", ec.Buf, resultChan)
	str := <-resultChan
	*atsel = c.rangeaddr.Eval(ec.Buf, *atsel)
	ec.Buf.Replace([]rune(str), atsel, ec.Buf.EditMark, ec.EventChan, util.EO_MOUSE)
	ec.Buf.EditMark = ec.Buf.EditMarkNext
}

func pipeoutcmdfn(c *cmd, atsel *util.Sel, ec EditContext) {
	*atsel = c.rangeaddr.Eval(ec.Buf, *atsel)
	str := string(ec.Buf.SelectionRunes(*atsel))
	NewJob(ec.Buf.Dir, c.bodytxt, str, ec.Buf, nil)
}

func pipecmdfn(c *cmd, atsel *util.Sel, ec EditContext) {
	*atsel = c.rangeaddr.Eval(ec.Buf, *atsel)
	str := string(ec.Buf.SelectionRunes(*atsel))
	resultChan := make(chan string)
	NewJob(ec.Buf.Dir, c.bodytxt, str, ec.Buf, resultChan)
	str = <-resultChan
	ec.Buf.Replace([]rune(str), atsel, ec.Buf.EditMark, ec.EventChan, util.EO_MOUSE)
	ec.Buf.EditMark = ec.Buf.EditMarkNext
}

func kcmdfn(c *cmd, atsel *util.Sel, ec EditContext) {
	*atsel = c.rangeaddr.Eval(ec.Buf, *atsel)
	ec.Sels[0] = *atsel
	ec.PushJump()
}
