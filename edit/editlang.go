package edit

import (
	"fmt"
	"yacco/util"
	"yacco/buf"
)

type commandFlag uint16
const (
	G_FLAG = 1 << iota
)

type cmd struct {
	cmdch rune
	rangeaddr Addr
	txtargs []string
	numarg int
	flags commandFlag
	argaddr Addr
	body *cmd
	bodytxt string
	fn func(c *cmd, buf *buf.Buffer, atsel util.Sel, sels []util.Sel, eventChan chan string)
}

func Edit(pgm string, b *buf.Buffer, sels []util.Sel, eventChan chan string) {
	ppgm := parse([]rune(pgm))
	ppgm.Exec(b, sels, eventChan)
}

func (ecmd *cmd) Exec(b *buf.Buffer, sels []util.Sel, eventChan chan string) {
	if ecmd.fn == nil {
		panic(fmt.Errorf("Command '%c' not implemented", ecmd.cmdch))
	}
	
	func() {
		ecmd.fn(ecmd, b, sels[0], sels, eventChan)
	}()
}

func AddrEval(pgm string, b *buf.Buffer, sel util.Sel) util.Sel{
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

func (ecmd *cmd) String() string {
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

	return s
}

