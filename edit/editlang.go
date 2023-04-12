package edit

import (
	"fmt"
	"strings"

	"github.com/aarzilli/yacco/buf"
	"github.com/aarzilli/yacco/regexp"
	"github.com/aarzilli/yacco/util"
)

type commandFlag uint16

const (
	G_FLAG = 1 << iota
)

type Cmd struct {
	cmdch       rune
	rangeaddr   Addr
	txtargdelim rune
	txtargs     []string
	numarg      int
	flags       commandFlag
	argaddr     Addr
	body        *Cmd
	mbody       []*Cmd
	bodytxt     string
	fn          func(c *Cmd, ec *EditContext)
	sregexp     *regexp.Regex
}

type EditContext struct {
	Buf       *buf.Buffer
	Sel       *util.Sel
	atsel     *util.Sel
	EventChan chan string
	BufMan    BufferManaging
	Trace     bool

	stash *[]buf.ReplaceOp
	depth int
}

type BufferManagingEntry struct {
	Buffer *buf.Buffer
	Sel    *util.Sel
}

type BufferManaging interface {
	Open(name string) *buf.Buffer
	List() []BufferManagingEntry
	Close(buf *buf.Buffer)
	RefreshBuffer(buf *buf.Buffer)
}

func Edit(pgm string, ec EditContext) {
	ppgm := Parse([]rune(pgm))
	ppgm.Exec(ec)
}

func (ecmd *Cmd) Exec(ec EditContext) {
	if ecmd.fn == nil {
		panic(fmt.Errorf("Command '%c' not implemented", ecmd.cmdch))
	}

	ec.atsel = ec.Sel

	ecmd.fn(ecmd, &ec)
}

func AddrEval(pgm string, b *buf.Buffer, sel util.Sel) util.Sel {
	rest := []rune(pgm)
	toks := []addrTok{}
	for {
		if len(rest) == 0 {
			break
		}
		var tok addrTok
		tok, rest = readAddressTok(rest)
		toks = append(toks, tok)
	}
	addr := parseAddr(toks)
	return addr.Eval(b, sel)
}

func (ecmd *Cmd) String(showregex bool) string {
	s := ""

	if ecmd.rangeaddr != nil {
		s += fmt.Sprintf("Range<%s> ", ecmd.rangeaddr.String())
	}

	s += fmt.Sprintf("Cmd<%c>", ecmd.cmdch)

	if ecmd.numarg != 0 {
		s += fmt.Sprintf(" Num<%d>", ecmd.numarg)
	}

	for _, t := range ecmd.txtargs {
		s += fmt.Sprintf(" Arg<%s>", t)
	}

	if ecmd.flags != 0 {
		s += fmt.Sprintf(" Flags<%d>", ecmd.flags)
	}

	if ecmd.argaddr != nil {
		s += fmt.Sprintf(" Addr<%s>", ecmd.argaddr)
	}

	if ecmd.body != nil {
		s += fmt.Sprintf(" Body<%s>", ecmd.body.String(showregex))
	}

	if ecmd.bodytxt != "" {
		s += fmt.Sprintf(" Body<%s>", ecmd.bodytxt)
	}

	if len(ecmd.mbody) > 0 {
		v := make([]string, len(ecmd.mbody))
		for i := range ecmd.mbody {
			v[i] = ecmd.mbody[i].String(false)
		}
		s += fmt.Sprintf(" Body<%s>", strings.Join(v, ", "))
	}

	if showregex && ecmd.sregexp != nil {
		s += fmt.Sprintf("\nCompiled Regex:\n")
		s += ecmd.sregexp.String()
	}

	return s
}

func ToMark(pgm *Cmd) *Cmd {
	if pgm.cmdch != ' ' {
		return nil
	}

	pgm.fn = Mcmdfn
	pgm.cmdch = 'M'
	return pgm
}

func (ec *EditContext) subec(buf *buf.Buffer, atsel *util.Sel) EditContext {
	if buf == ec.Buf {
		return EditContext{
			Buf:       ec.Buf,
			Sel:       ec.Sel,
			atsel:     atsel,
			EventChan: ec.EventChan,
			BufMan:    ec.BufMan,
			Trace:     ec.Trace,
			depth:     ec.depth + 1,
		}
	} else {
		return EditContext{
			Buf:       buf,
			Sel:       nil,
			atsel:     atsel,
			EventChan: nil,
			BufMan:    ec.BufMan,
			Trace:     ec.Trace,
			depth:     ec.depth + 1,
		}
	}
}
