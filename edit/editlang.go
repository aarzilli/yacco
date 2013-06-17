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
	delim rune
	numarg int
	flags commandFlag
	argaddr Addr
	body *cmd
}

func Edit(pgm string, buf *buf.Buffer, sels []util.Sel) {
	ppgm := parse([]rune(pgm))
	ppgm.Exec(buf, sels)
}

func (ecmd *cmd) Exec(buf *buf.Buffer, sels []util.Sel) {
	//TODO
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

