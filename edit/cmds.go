package edit

import (
	"fmt"
	"regexp"
	"yacco/util"
)

var Warnfn func(msg string)
var NewJob func(wd, cmd, input string, resultChan chan<- string)

const LOOP_LIMIT = 200

func nilcmdfn(c *cmd, atsel util.Sel, ec EditContext) {
	ec.Sels[0] = c.rangeaddr.Eval(ec.Buf, atsel)
}

func inscmdfn(dir int, c *cmd, atsel util.Sel, ec EditContext) {
	sel := c.rangeaddr.Eval(ec.Buf, atsel)

	switch c.cmdch {
	case 'a':
		sel.S = sel.E
	case 'i':
		sel.E = sel.S
	}

	ec.Buf.Replace([]rune(c.txtargs[0]), &sel, ec.Sels, ec.Buf.EditMark, ec.EventChan, util.EO_MOUSE, false)
	ec.Buf.EditMark = ec.Buf.EditMarkNext

	ec.Sels[0] = sel
}

func mtcmdfn(del bool, c *cmd, atsel util.Sel, ec EditContext) {
	selfrom := c.rangeaddr.Eval(ec.Buf, atsel)
	selto := c.argaddr.Eval(ec.Buf, atsel).E

	txt := ec.Buf.SelectionRunes(selfrom)

	if selto > selfrom.E {
		ec.Buf.Replace(txt, &util.Sel{selto, selto}, ec.Sels, ec.Buf.EditMark, ec.EventChan, util.EO_MOUSE, false)
		ec.Buf.Replace([]rune{}, &selfrom, ec.Sels, false, ec.EventChan, util.EO_MOUSE, true)
		ec.Buf.EditMark = ec.Buf.EditMarkNext
	} else {
		ec.Buf.Replace([]rune{}, &selfrom, ec.Sels, ec.Buf.EditMark, ec.EventChan, util.EO_MOUSE, false)
		ec.Buf.Replace(txt, &util.Sel{selto, selto}, ec.Sels, false, ec.EventChan, util.EO_MOUSE, true)
		ec.Buf.EditMark = ec.Buf.EditMarkNext
	}
}

func pcmdfn(c *cmd, atsel util.Sel, ec EditContext) {
	sel := c.rangeaddr.Eval(ec.Buf, atsel)
	txt := ec.Buf.SelectionRunes(sel)
	ec.Sels[0] = sel
	Warnfn(string(txt))
}

func eqcmdfn(c *cmd, atsel util.Sel, ec EditContext) {
	sel := c.rangeaddr.Eval(ec.Buf, atsel)
	sline, scol := ec.Buf.GetLine(sel.S)
	eline, ecol := ec.Buf.GetLine(sel.E)
	if (sline == eline) && (scol == ecol) {
		Warnfn(fmt.Sprintf("%d:%d", sline, scol))
	} else {
		Warnfn(fmt.Sprintf("%d:%d %d:%d", sline, scol, eline, ecol))
	}
}

func scmdfn(c *cmd, atsel util.Sel, ec EditContext) {
	sel := c.rangeaddr.Eval(ec.Buf, atsel)
	re := regexp.MustCompile("(?m)" + c.txtargs[0])
	subs := []rune(c.txtargs[1])
	end := sel.E
	first := ec.Buf.EditMark
	count := 0
	for {
		psel := sel.S
		br := ec.Buf.ReaderFrom(sel.S, end)
		loc := re.FindReaderIndex(br)
		if (loc == nil) || (len(loc) < 2) {
			return
		}
		sel = util.Sel{loc[0] + sel.S, loc[1] + sel.S}
		ec.Buf.Replace(subs, &sel, ec.Sels, first, ec.EventChan, util.EO_MOUSE, false)
		if sel.S != psel {
			count++
		} else {
			count = 0
		}
		if count > 100 {
			panic("s Loop got stuck")
		}
		first = false
	}
	ec.Buf.EditMark = ec.Buf.EditMarkNext
}

func xcmdfn(c *cmd, atsel util.Sel, ec EditContext) {
	sel := c.rangeaddr.Eval(ec.Buf, atsel)
	re := regexp.MustCompile("(?m)" + c.txtargs[0])
	end := sel.E
	count := 0
	ebn := ec.Buf.EditMarkNext
	ec.Buf.EditMarkNext = false
	for {
		oldS := sel.S
		br := ec.Buf.ReaderFrom(sel.S, end)
		loc := re.FindReaderIndex(br)
		if (loc == nil) || (len(loc) < 2) {
			return
		}
		sel = util.Sel{loc[0] + sel.S, loc[1] + sel.S}
		ec.Sels[0] = sel
		c.body.fn(c.body, sel, ec)
		if (ec.Sels[0].S == sel.S) && (ec.Sels[0].E == sel.E) {
			sel.S = sel.E
		} else {
			sel.S = ec.Sels[0].E
		}
		if sel.S <= oldS {
			count++
			if count > LOOP_LIMIT {
				Warnfn("x/y loop seems stuck\n")
				return
			}
		}
	}
	ec.Buf.EditMarkNext = ebn
	ec.Buf.EditMark = ec.Buf.EditMarkNext
}

func ycmdfn(c *cmd, atsel util.Sel, ec EditContext) {
	sel := c.rangeaddr.Eval(ec.Buf, atsel)
	re := regexp.MustCompile("(?m)" + c.txtargs[0])
	end := sel.E
	count := 0
	for {
		oldS := sel.S
		br := ec.Buf.ReaderFrom(sel.S, end)
		loc := re.FindReaderIndex(br)
		if (loc == nil) || (len(loc) < 2) {
			return
		}
		sel.E = loc[0] + sel.S
		ec.Sels[0] = sel
		c.body.fn(c.body, sel, ec)
		sel.S = sel.E
		if sel.S <= oldS {
			count++
			if count > LOOP_LIMIT {
				Warnfn("x/y loop seems stuck\n")
				return
			}
		}
	}
}

func gcmdfn(inv bool, c *cmd, atsel util.Sel, ec EditContext) {
	sel := c.rangeaddr.Eval(ec.Buf, atsel)
	rr := ec.Buf.ReaderFrom(sel.S, sel.E)
	re := regexp.MustCompile("(?m)" + c.txtargs[0])
	loc := re.FindReaderIndex(rr)
	if loc == nil {
		if inv {
			c.body.fn(c.body, sel, ec)
		}
	} else {
		if !inv {
			c.body.fn(c.body, sel, ec)
		}
	}
}

func pipeincmdfn(c *cmd, atsel util.Sel, ec EditContext) {
	resultChan := make(chan string)
	NewJob(ec.Buf.Dir, c.bodytxt, "", resultChan)
	str := <-resultChan
	sel := c.rangeaddr.Eval(ec.Buf, atsel)
	ec.Buf.Replace([]rune(str), &sel, ec.Sels, ec.Buf.EditMark, ec.EventChan, util.EO_MOUSE, true)
	ec.Buf.EditMark = ec.Buf.EditMarkNext
}

func pipeoutcmdfn(c *cmd, atsel util.Sel, ec EditContext) {
	sel := c.rangeaddr.Eval(ec.Buf, atsel)
	str := string(ec.Buf.SelectionRunes(sel))
	NewJob(ec.Buf.Dir, c.bodytxt, str, nil)
}

func pipecmdfn(c *cmd, atsel util.Sel, ec EditContext) {
	sel := c.rangeaddr.Eval(ec.Buf, atsel)
	str := string(ec.Buf.SelectionRunes(sel))
	resultChan := make(chan string)
	NewJob(ec.Buf.Dir, c.bodytxt, str, resultChan)
	str = <-resultChan
	ec.Buf.Replace([]rune(str), &sel, ec.Sels, ec.Buf.EditMark, ec.EventChan, util.EO_MOUSE, true)
	ec.Buf.EditMark = ec.Buf.EditMarkNext
}

func kcmdfn(c *cmd, atsel util.Sel, ec EditContext) {
	sel := c.rangeaddr.Eval(ec.Buf, atsel)
	ec.Sels[0] = sel
	ec.PushJump()
}
