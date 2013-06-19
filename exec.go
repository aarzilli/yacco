package main

import (
	"fmt"
	"strings"
	"regexp"
	"yacco/buf"
	"yacco/edit"
	"yacco/textframe"
)

type ExecContext struct {
	col *Col
	ed *Editor

	br BufferRefreshable
	ontag bool
	fr *textframe.Frame
	buf *buf.Buffer
}

type Cmd func(ec ExecContext, arg string)

var cmds = map[string]Cmd{
	"Cut": func (ec ExecContext, arg string) { CopyCmd(ec, arg, true) },
	"Del": func (ec ExecContext, arg string) { DelCmd(ec, arg, false) },
	"Delcol": DelcolCmd,
	"Delete": func (ec ExecContext, arg string) { DelCmd(ec, arg, true) },
	"Dump": DumpCmd,
	"Edit": EditCmd,
	"Exit": ExitCmd,
	"Kill": KillCmd,
	"Load": LoadCmd,
	"Setenv": SetenvCmd,
	"Look": LookCmd,
	"New": NewCmd,
	"Newcol": NewcolCmd,
	"Paste": PasteCmd,
	"Put": PutCmd,
	"Putall": PutallCmd,
	"Redo": RedoCmd,
	"Send": SendCmd,
	"Snarf": func (ec ExecContext, arg string) { CopyCmd(ec, arg, false) },
	"Copy": func (ec ExecContext, arg string) { CopyCmd(ec, arg, false) },
	"Sort": SortCmd,
	"Undo": UndoCmd,
	"Zerox": ZeroxCmd,
	"|": PipeCmd,
	"<": PipeInCmd,
	">": PipeOutCmd,

	// New
	"Cd": CdCmd,
}

var spacesRe = regexp.MustCompile("\\s+")

func Exec(ec ExecContext, cmd string) {
	defer func() {
		if r := recover(); r != nil {
			errmsg := fmt.Sprintf("%v", r)
			Warn(errmsg)
		}
	}()
	v := spacesRe.Split(strings.TrimSpace(cmd), 2)
	if f, ok := cmds[v[0]]; ok {
		f(ec, v[1])
	} else {
		println("External command:", cmd)
	}
}

func CopyCmd(ec ExecContext, arg string, del bool) {
	//TODO
}

func DelCmd(ec ExecContext, arg string, del bool) {
	//TODO
}

func DelcolCmd(ec ExecContext, arg string) {
	//TODO
}

func DumpCmd(ec ExecContext, arg string) {
	//TODO
}

func EditCmd(ec ExecContext, arg string) {
	edit.Edit(arg, ec.buf, ec.fr.Sels)
	ec.br.BufferRefresh(ec.ontag)
}

func ExitCmd(ec ExecContext, arg string) {
	//TODO
}

func KillCmd(ec ExecContext, arg string) {
	//TODO
}

func LoadCmd(ec ExecContext, arg string) {
	//TODO
}

func SetenvCmd(ec ExecContext, arg string) {
	//TODO
}

func LookCmd(ec ExecContext, arg string) {
	//TODO
}

func NewCmd(ec ExecContext, arg string) {
	//TODO
}

func NewcolCmd(ec ExecContext, arg string) {
	//TODO
}

func PasteCmd(ec ExecContext, arg string) {
	//TODO
}

func PutCmd(ec ExecContext, arg string) {
	//TODO
}

func PutallCmd(ec ExecContext, arg string) {
	//TODO
}

func RedoCmd(ec ExecContext, arg string) {
	//TODO
}

func SendCmd(ec ExecContext, arg string) {
	//TODO
}

func SortCmd(ec ExecContext, arg string) {
	//TODO
}

func UndoCmd(ec ExecContext, arg string) {
	//TODO
}

func ZeroxCmd(ec ExecContext, arg string) {
	//TODO
}

func PipeCmd(ec ExecContext, arg string) {
	//TODO
}

func PipeInCmd(ec ExecContext, arg string) {
	//TODO
}

func PipeOutCmd(ec ExecContext, arg string) {
	//TODO
}

func CdCmd(ec ExecContext, arg string) {
	//TODO
}

