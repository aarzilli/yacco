package edit

import (
	"fmt"
	"regexp"
	"yacco/util"
	"yacco/buf"
)

var Warnfn func(msg string)
var NewJob func (wd, cmd, input string, resultChan chan<- string)

const LOOP_LIMIT = 200

func nilcmdfn(c *cmd, b *buf.Buffer, atsel util.Sel, sels []util.Sel, eventChan chan string) {
	sels[0] = c.rangeaddr.Eval(b, atsel)
}

func inscmdfn(dir int, c *cmd, b *buf.Buffer, atsel util.Sel, sels []util.Sel, eventChan chan string) {
	sel := c.rangeaddr.Eval(b, atsel)

	switch c.cmdch {
	case 'a':
		sel.S = sel.E
	case 'i':
		sel.E = sel.S
	}

	b.Replace([]rune(c.txtargs[0]), &sel, sels, true, eventChan, util.EO_MOUSE, false)

	sels[0] = sel
}

func mtcmdfn(del bool, c *cmd, b *buf.Buffer, atsel util.Sel, sels []util.Sel, eventChan chan string) {
	selfrom := c.rangeaddr.Eval(b, atsel)
	selto := c.argaddr.Eval(b, atsel).E

	txt := buf.ToRunes(b.SelectionX(selfrom))

	if selto > selfrom.E {
		b.Replace(txt, &util.Sel{ selto, selto }, sels, true, eventChan, util.EO_MOUSE, false)
		b.Replace([]rune{}, &selfrom, sels, false, eventChan, util.EO_MOUSE, true)
	} else {
		b.Replace([]rune{}, &selfrom, sels, true, eventChan, util.EO_MOUSE, false)
		b.Replace(txt, &util.Sel{ selto, selto }, sels, false, eventChan, util.EO_MOUSE, true)
	}
}

func pcmdfn(c *cmd, b *buf.Buffer, atsel util.Sel, sels []util.Sel, eventChan chan string) {
	sel := c.rangeaddr.Eval(b, atsel)
	txt := b.SelectionX(sel)
	sels[0] = sel
	Warnfn(string(buf.ToRunes(txt)))
}

func eqcmdfn(c *cmd, b *buf.Buffer, atsel util.Sel, sels []util.Sel, eventChan chan string) {
	sel := c.rangeaddr.Eval(b, atsel)
	sline, scol := b.GetLine(sel.S)
	eline, ecol := b.GetLine(sel.E)
	if (sline == eline) && (scol == ecol) {
		Warnfn(fmt.Sprintf("%d:%d", sline, scol))
	} else {
		Warnfn(fmt.Sprintf("%d:%d %d:%d", sline, scol, eline, ecol))
	}
}

func scmdfn(c *cmd, b *buf.Buffer, atsel util.Sel, sels []util.Sel, eventChan chan string) {
	sel := c.rangeaddr.Eval(b, atsel)
	re := regexp.MustCompile("(?m)" + c.txtargs[0])
	subs := []rune(c.txtargs[1])
	end := sel.E
	first := true
	count := 0
	for {
		psel := sel.S
		br := b.ReaderFrom(sel.S, end)
		loc := re.FindReaderIndex(br)
		if (loc == nil) || (len(loc) < 2) {
			return
		}
		sel = util.Sel{ loc[0] + sel.S, loc[1] + sel.S }
		b.Replace(subs, &sel, sels, first, eventChan, util.EO_MOUSE, false)
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
}

func xcmdfn(c *cmd, b *buf.Buffer, atsel util.Sel, sels []util.Sel, eventChan chan string) {
	sel := c.rangeaddr.Eval(b, atsel)
	re := regexp.MustCompile("(?m)" + c.txtargs[0])
	end := sel.E
	count := 0
	for {
		oldS := sel.S
		br := b.ReaderFrom(sel.S, end)
		loc := re.FindReaderIndex(br)
		if (loc == nil) || (len(loc) < 2) {
			return
		}
		sel = util.Sel{ loc[0] + sel.S, loc[1] + sel.S }
		sels[0] = sel
		c.body.fn(c.body, b, sel, sels, eventChan)
		if (sels[0].S == sel.S) && (sels[0].E == sel.E) {
			sel.S = sel.E
		} else {
			sel.S = sels[0].E
		}
		if sel.S <= oldS {
			count++
			if count > LOOP_LIMIT {
				Warnfn("x/y loop seems stuck\n")
				return
			}
		}
	}
}

func ycmdfn(c *cmd, b *buf.Buffer, atsel util.Sel, sels []util.Sel, eventChan chan string) {
	sel := c.rangeaddr.Eval(b, atsel)
	re := regexp.MustCompile("(?m)" + c.txtargs[0])
	end := sel.E
	count := 0
	for {
		oldS := sel.S
		br := b.ReaderFrom(sel.S, end)
		loc := re.FindReaderIndex(br)
		if (loc == nil) || (len(loc) < 2) {
			return
		}
		sel.E = loc[0] + sel.S
		sels[0] = sel
		c.body.fn(c.body, b, sel, sels, eventChan)
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

func gcmdfn(inv bool, c *cmd, b *buf.Buffer, atsel util.Sel, sels []util.Sel, eventChan chan string) {
	sel := c.rangeaddr.Eval(b, atsel)
	rr := b.ReaderFrom(sel.S, sel.E)
	re := regexp.MustCompile("(?m)" + c.txtargs[0])
	loc := re.FindReaderIndex(rr)
	if loc == nil {
		if inv {
			c.body.fn(c.body, b, sel, sels, eventChan)
		}
	} else {
		if !inv {
			c.body.fn(c.body, b, sel, sels, eventChan)
		}
	}
}

func pipeincmdfn(c *cmd, b *buf.Buffer, atsel util.Sel, sels []util.Sel, eventChan chan string) {
	resultChan := make(chan string)
	NewJob(b.Dir, c.bodytxt, "", resultChan)
	str := <- resultChan
	sel := c.rangeaddr.Eval(b, atsel)
	b.Replace([]rune(str), &sel, sels, true, eventChan, util.EO_MOUSE, true)
}

func pipeoutcmdfn(c *cmd, b *buf.Buffer, atsel util.Sel, sels []util.Sel, eventChan chan string) {
	sel := c.rangeaddr.Eval(b, atsel)
	str := string(buf.ToRunes(b.SelectionX(sel)))
	NewJob(b.Dir, c.bodytxt, str, nil)
}

func pipecmdfn(c *cmd, b *buf.Buffer, atsel util.Sel, sels []util.Sel, eventChan chan string) {
	sel := c.rangeaddr.Eval(b, atsel)
	str := string(buf.ToRunes(b.SelectionX(sel)))
	resultChan := make(chan string)
	NewJob(b.Dir, c.bodytxt, str, resultChan)
	str = <- resultChan
	b.Replace([]rune(str), &sel, sels, true, eventChan, util.EO_MOUSE, true)
}

