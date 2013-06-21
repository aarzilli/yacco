package main

import (
	"os"
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
var exitConfirmed = false

func Exec(ec ExecContext, cmd string) {
	defer func() {
		if r := recover(); r != nil {
			errmsg := fmt.Sprintf("%v", r)
			Warn(errmsg)
		}
	}()

	cmd = strings.TrimSpace(cmd)

	if (cmd[0] == '<') || (cmd[0] == '>') || (cmd[0] == '|') {
		cmds[cmd[:1]](ec, cmd[1:])
	} else {
		v := spacesRe.Split(cmd, 2)
		if f, ok := cmds[v[0]]; ok {
			arg := ""
			if len(v) > 1 {
				arg = v[1]
			}
			f(ec, arg)
		} else {
			println("External command:", cmd)
		}
	}
}

func CopyCmd(ec ExecContext, arg string, del bool) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	//TODO
}

func DelCmd(ec ExecContext, arg string, confirmed bool) {
	exitConfirmed = false
	if !ec.ed.bodybuf.Modified || confirmed || ec.ed.confirmDel {
		col := ec.ed.Column()
		col.Remove(col.IndexOf(ec.ed))
		removeBuffer(ec.ed.bodybuf)
		wnd.wnd.FlushImage()
	} else {
		ec.ed.confirmDel = true
		Warn("File " + ec.ed.bodybuf.ShortName() + " has unsaved changes")
	}
}

func DelcolCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	//TODO
}

func DumpCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	//TODO
}

func EditCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false

	edit.Edit(arg, ec.buf, ec.fr.Sels)
	ec.br.BufferRefresh(ec.ontag)
}

func ExitCmd(ec ExecContext, arg string) {
	t := "The following files have unsaved changes:\n"
	n := 0
	for _, buf := range buffers {
		if buf.Modified && (buf.Name[0] != '+') {
			t += buf.ShortName() + "\n"
			n++
		}
	}

	if (n == 0) || exitConfirmed {
		os.Exit(0)
	} else {
		exitConfirmed = true
		Warn(t)
	}
}

func KillCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	//TODO
}

func LoadCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	//TODO
}

func SetenvCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	//TODO
}

func LookCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	//TODO
}

func NewCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	arg = strings.TrimSpace(arg)
	if arg == "" {
		Warn("New: must specify argument")
		return
	}
	HeuristicOpen(arg, true)
}

func NewcolCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	//TODO
}

func PasteCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	//TODO
}

func PutCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	//TODO
}

func PutallCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	//TODO
}

func RedoCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	//TODO
}

func SendCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	//TODO
}

func SortCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	//TODO
}

func UndoCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	//TODO
}

func ZeroxCmd(ec ExecContext, arg string) {
	exitConfirmed = false

	//TODO
}

func PipeCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	//TODO
}

func PipeInCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	//TODO
}

func PipeOutCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	//TODO
}

func CdCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	os.Chdir(arg)
	for _, col := range wnd.cols.cols {
		for _, ed := range col.editors {
			ed.BufferRefresh(false)
		}
	}

	wnd.GenTag()

	wnd.BufferRefresh(true)

	wnd.cols.Redraw()
	wnd.tagfr.Redraw(false)
	wnd.wnd.FlushImage()
}

