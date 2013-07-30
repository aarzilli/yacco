package edit

import (
	"fmt"
	"regexp"
	"strconv"
	"yacco/buf"
	"yacco/util"
)

type Addr interface {
	Empty() bool
	String() string
	Eval(b *buf.Buffer, sel util.Sel) util.Sel
}

type AddrOp struct {
	Op string
	Lh Addr
	Rh Addr
}

func (e *AddrOp) Empty() bool {
	return false
}

func (e *AddrOp) String() string {
	return fmt.Sprintf("Op<%s %s %s>", e.Lh.String(), e.Op, e.Rh.String())
}

func (e *AddrOp) Eval(b *buf.Buffer, sel util.Sel) util.Sel {
	switch e.Op {
	default:
		fallthrough
	case ",":
		slh := e.Lh.Eval(b, sel)
		srh := e.Rh.Eval(b, sel)
		return util.Sel{slh.S, srh.E}
	case ";":
		slh := e.Lh.Eval(b, sel)
		return e.Rh.Eval(b, slh)
	}
}

type addrEmpty struct {
}

func (e *addrEmpty) Empty() bool {
	return true
}

func (e *addrEmpty) String() string {
	return "·"
}

func (e *addrEmpty) Eval(b *buf.Buffer, sel util.Sel) (rsel util.Sel) {
	return sel
}

type AddrBase struct {
	Batype string
	Value  string
	Dir    int
}

func (e *AddrBase) Empty() bool {
	return false
}

func (e *AddrBase) String() string {
	dirch := ""
	if e.Dir > 0 {
		dirch = "+"
	} else if e.Dir < 0 {
		dirch = "-"
	}
	return fmt.Sprintf("%s%s%s", dirch, e.Batype, e.Value)
}

func (e *AddrBase) Eval(b *buf.Buffer, sel util.Sel) (rsel util.Sel) {
	switch e.Batype {
	case ".":
		if e.Dir != 0 {
			panic(fmt.Errorf("Bad address syntax, non-absolute '.'"))
		}
		rsel = sel
		// Nothing to do

	case "":
		if e.Dir >= 0 {
			value := asnumber(e.Value)
			if e.Dir == 0 {
				value--
				rsel.S = 0
				rsel.E = 0
			} else {
				rsel = sel
			}
			if rsel.S != rsel.E {
				rsel.E -= 1
			}

			prev_lineend := rsel.E
			lineend := b.Tonl(rsel.S, 1)
			for i := 0; i < value; i++ {
				prev_lineend = lineend
				lineend = b.Tonl(lineend, 1)
			}
			rsel.S = prev_lineend
			rsel.E = lineend
		} else {
			rsel = sel
			prev_linestart := rsel.S
			linestart := b.Tonl(rsel.S-1, -1)
			if linestart >= prev_linestart {
				linestart = prev_linestart
			}
			for i := 0; i < asnumber(e.Value); i++ {
				prev_linestart = linestart
				linestart = b.Tonl(linestart-2, -1)
			}
			rsel.S = linestart
			rsel.E = prev_linestart
		}

		return rsel

	case "#w":
		if e.Dir >= 0 {
			if e.Dir == 0 {
				rsel.S = 0
				rsel.E = 0
			} else {
				rsel = sel
			}

			for i := 0; i < asnumber(e.Value); i++ {
				rsel.S = rsel.E
				rsel.E = b.Towd(rsel.E, +1)
			}
		} else {
			rsel = sel
			for i := 0; i < asnumber(e.Value); i++ {
				rsel.E = rsel.S
				rsel.S = b.Towd(rsel.S-1, -1)
			}
		}
		b.FixSel(&rsel)

	case "#":
		rsel = setStartSel(e.Dir, sel)
		dir := 1
		if e.Dir < 0 {
			dir = -1
		}
		rsel.S += dir * asnumber(e.Value)
		rsel.E = rsel.S
		b.FixSel(&rsel)
		rsel.E = rsel.S

	case "#?":
		if (e.Value != "1") || (e.Dir != -1) {
			panic(fmt.Errorf("Address starting with #? only supported as backward motion and with a value of 1"))
		}
		rsel = setStartSel(e.Dir, sel)
		if rsel.S < b.Size() {
			rsel.S--
			rsel.E = rsel.S
		}

	case "$":
		if e.Dir != 0 {
			panic(fmt.Errorf("Bad address syntax, non-absolute '$'"))
		}
		rsel.S = b.Size()
		rsel.E = rsel.S

	case "?":
		rsel = setStartSel(e.Dir, sel)
		rsel = regexpEval(b, rsel, e.Value, -e.Dir)

	case "/":
		rsel = setStartSel(e.Dir, sel)
		rsel = regexpEval(b, rsel, e.Value, e.Dir)

	}

	return rsel
}

func setStartSel(dir int, sel util.Sel) (rsel util.Sel) {
	if dir == 0 {
		rsel.S = 0
		rsel.E = 0
	} else if dir > 0 {
		rsel.S = sel.E
		rsel.E = sel.E
	} else if dir < 0 {
		rsel.S = sel.S
		rsel.E = sel.E
	}
	return
}

func regexpEval(b *buf.Buffer, sel util.Sel, rstr string, dir int) util.Sel {
	doerr := true
	if rstr[0] == '@' {
		rstr = rstr[1:]
		doerr = false
	}
	rstr = "(?m)" + rstr
	re := regexp.MustCompile(rstr)

	if dir >= 0 {
		return regexpEvalFwd(b, sel, re, rstr, doerr)
	} else {
		return regexpEvalBwd(b, sel, re, rstr, doerr)
	}

}

func regexpEvalFwd(b *buf.Buffer, sel util.Sel, re *regexp.Regexp, rstr string, doerr bool) util.Sel {
	loc := re.FindReaderIndex(b.ReaderFrom(sel.E, b.Size()))
	if loc == nil {
		if doerr {
			panic(fmt.Errorf("No match found for: %s", rstr))
		}
	} else {
		sel.S = sel.E + loc[0]
		sel.E = sel.E + loc[1]
	}
	return sel
}

func regexpEvalBwd(b *buf.Buffer, sel util.Sel, re *regexp.Regexp, rstr string, doerr bool) util.Sel {
	// XXX Awfully inefficient but I don't want to implement a regexp engine
	s := 0
	p := sel.S
	found := false
	for {
		loc := re.FindReaderIndex(b.ReaderFrom(s, b.Size()))
		if loc == nil {
			if !found && doerr {
				panic(fmt.Errorf("No match found for: %s", rstr))
			}
			return sel
		} else {
			found = true
			if loc[0]+s > p {
				return sel
			}
			sel.S = loc[0] + s
			sel.E = loc[1] + s
		}
	}
}

type AddrList struct {
	Addrs []Addr
}

func (e *AddrList) Empty() bool {
	return false
}

func (e *AddrList) String() string {
	s := "List<"
	for _, addr := range e.Addrs {
		s += addr.String() + " "
	}
	s += ">"
	return s
}

func (e *AddrList) Eval(b *buf.Buffer, sel util.Sel) (rsel util.Sel) {
	r := sel
	for _, addr := range e.Addrs {
		r = addr.Eval(b, r)
	}
	return r
}

func asnumber(v string) int {
	n, err := strconv.Atoi(v)
	if err != nil {
		n = 0
	}
	return n
}
