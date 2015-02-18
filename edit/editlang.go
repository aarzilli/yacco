package edit

import (
	"fmt"
	"yacco/buf"
	"yacco/regexp"
	"yacco/util"
)

type commandFlag uint16

const (
	G_FLAG = 1 << iota
)

type Cmd struct {
	cmdch     rune
	rangeaddr Addr
	txtargs   []string
	numarg    int
	flags     commandFlag
	argaddr   Addr
	body      *Cmd
	bodytxt   string
	fn        func(c *Cmd, atsel *util.Sel, ec EditContext)
	sregexp   regexp.Regex
}

type EditContext struct {
	Buf       *buf.Buffer
	Sels      []util.Sel
	EventChan chan string
	PushJump  func()
}

func Edit(pgm string, ec EditContext) {
	ppgm := Parse([]rune(pgm))
	ppgm.Exec(ec)
}

func (ecmd *Cmd) Exec(ec EditContext) {
	if ecmd.fn == nil {
		panic(fmt.Errorf("Command '%c' not implemented", ecmd.cmdch))
	}

	ecmd.fn(ecmd, &ec.Sels[0], ec)
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

func (ecmd *Cmd) String() string {
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
		s += fmt.Sprintf(" Body<%s>", ecmd.body.String())
	}

	if ecmd.sregexp != nil {
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
