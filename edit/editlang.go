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
	fn func(c *cmd, buf *buf.Buffer, sels []util.Sel)
}

func Edit(pgm string, b *buf.Buffer, sels []util.Sel) {
	ppgm := parse([]rune(pgm))
	ppgm.Exec(b, sels)
}

func (ecmd *cmd) Exec(b *buf.Buffer, sels []util.Sel) {
	if ecmd.fn == nil {
		panic(fmt.Errorf("Command '%c' not implemented", ecmd.cmdch))
	}

	ecmd.fn(ecmd, b, sels)
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

