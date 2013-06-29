package main

import (
	"os"
	"fmt"
	"strings"
	"regexp"
	"sort"
	"runtime"
	"strconv"
	"path/filepath"
	"yacco/buf"
	"yacco/util"
	"yacco/edit"
	"yacco/textframe"
	"yacco/config"
)

type ExecContext struct {
	col *Col
	ed *Editor

	br BufferRefreshable
	ontag bool
	fr *textframe.Frame
	buf *buf.Buffer
	eventChan chan string
}

type Cmd func(ec ExecContext, arg string)

var cmds = map[string]Cmd{
	"Cut": func (ec ExecContext, arg string) { CopyCmd(ec, arg, true) },
	"Get": GetCmd,
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
	"Jobs": JobsCmd,
}

var spacesRe = regexp.MustCompile("\\s+")
var exitConfirmed = false

func IntlCmd(cmd string) (Cmd, string, bool) {
	if len(cmd) <= 0 {
		return nil, "", true
	}

	if (cmd[0] == '<') || (cmd[0] == '>') || (cmd[0] == '|') {
		return cmds[cmd[:1]], cmd[1:], true
	} else {
		v := spacesRe.Split(cmd, 2)
		if f, ok := cmds[v[0]]; ok {
			arg := ""
			if len(v) > 1 {
				arg = v[1]
			}
			return f, arg, true
		} else {
			return nil, "", false
		}
	}
}

func Exec(ec ExecContext, cmd string) {
	defer func() {
		if r := recover(); r != nil {
			errmsg := fmt.Sprintf("%v\n", r)
			if config.EditErrorTrace {
				for i := 1; ; i++ {
					_, file, line, ok := runtime.Caller(i)
					if !ok { break }
					errmsg += fmt.Sprintf("  %s:%d\n", file, line)
				}
			}
			Warn(errmsg)
		}
	}()

	cmd = strings.TrimSpace(cmd)
	xcmd, arg, isintl := IntlCmd(cmd)
	if isintl {
		if xcmd != nil {
			xcmd(ec, arg)
		}
	} else {
		ExtExec(ec, cmd)
	}
}

func ExtExec(ec ExecContext, cmd string) {
	wd := wnd.tagbuf.Dir
	if ec.buf != nil {
		wd = ec.buf.Dir
	}
	NewJob(wd, cmd, "", &ec, false)
}

func CopyCmd(ec ExecContext, arg string, del bool) {
	exitConfirmed = false
	if ec.ed != nil {
		ec.ed.confirmDel = false
	}
	if (ec.buf == nil) || (ec.fr == nil) || (ec.br == nil) {
		return
	}

	s := string(buf.ToRunes(ec.buf.SelectionX(ec.fr.Sels[0])))
	if del {
		ec.buf.Replace([]rune{}, &ec.fr.Sels[0], ec.fr.Sels, true, ec.eventChan, util.EO_MOUSE)
		ec.br.BufferRefresh(ec.ontag)
	}
	wnd.wnd.SetClipboard(s)
}

func DelCmd(ec ExecContext, arg string, confirmed bool) {
	exitConfirmed = false
	if !ec.ed.bodybuf.Modified || (ec.ed.bodybuf.Name[0] == '+') || confirmed || ec.ed.confirmDel {
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
	if ec.col == nil {
		return
	}

	t := "The following files have unsaved changes:\n"
	n := 0
	for _, ed := range ec.col.editors {
		if ed.bodybuf.Modified && (ed.bodybuf.Name[0] != '+') && !ed.confirmDel {
			ed.confirmDel = true
			t += ed.bodybuf.ShortName() + "\n"
			n++
		}
	}

	if n == 0 {
		for _, ed := range ec.col.editors {
			removeBuffer(ed.bodybuf)
		}
		wnd.cols.Remove(wnd.cols.IndexOf(ec.col))
		wnd.wnd.FlushImage()
	} else {
		Warn(t)
	}
}

func DumpCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	//TODO
}

func EditCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed != nil {
		ec.ed.confirmDel = false
	}
	if (ec.buf == nil) || (ec.fr == nil) || (ec.br == nil) {
		return
	}

	edit.Edit(arg, ec.buf, ec.fr.Sels, ec.eventChan)
	ec.br.BufferRefresh(ec.ontag)
}

func ExitCmd(ec ExecContext, arg string) {
	t := "The following files have unsaved changes:\n"
	n := 0
	for _, buf := range buffers {
		if buf == nil {
			continue
		}
		if buf.Modified && (buf.Name[0] != '+') {
			t += buf.ShortName() + "\n"
			n++
		}
	}

	if (n == 0) || exitConfirmed {
		FsQuit()
	} else {
		exitConfirmed = true
		Warn(t)
	}
}

func JobsCmd(ec ExecContext, arg string) {
	t := ""
	for i, job := range jobs {
		if job == nil {
			continue
		}
		t += fmt.Sprintf("%d %s\n", i, job.descr)
	}
	Warnfull(filepath.Join(wnd.tagbuf.Dir, "+Jobs"), t)
}

func KillCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	arg = strings.TrimSpace(arg)
	if arg == "" {
		for i := range jobs {
			jobKill(i)
		}
	} else {
		n, _ := strconv.Atoi(arg)
		jobKill(n)
	}
}

func LoadCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	//TODO
}

func SetenvCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	v := spacesRe.Split(arg, 2)
	if len(v) != 2 {
		Warn("Setenv: wrong number of arguments")
		return
	}
	os.Setenv(v[0], v[1])
}

func LookCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	//TODO
}

func GetCmd(ec ExecContext, arg string) {
	//TODO: implement
}

func NewCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	arg = strings.TrimSpace(arg)
	if arg == "" {
		Warn("New: must specify argument")
		return
	}
	_, err := HeuristicOpen(arg, true)
	if err != nil {
		Warn("New: " + err.Error())
	}
}

func NewcolCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	wnd.cols.AddAfter(-1)
	wnd.wnd.FlushImage()
}

func PasteCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed != nil {
		ec.ed.confirmDel = false
	}
	if (ec.buf == nil) || (ec.fr == nil) || (ec.br == nil) {
		return
	}
	ec.buf.Replace([]rune(wnd.wnd.GetClipboard()), &ec.fr.Sels[0], ec.fr.Sels, true, ec.eventChan, util.EO_MOUSE)
	ec.br.BufferRefresh(ec.ontag)
}

func PutCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	if ec.ed.bodybuf.Name[0] == '+' {
		return
	}
	err := ec.ed.bodybuf.Put()
	if err != nil {
		Warn(fmt.Sprintf("Put: Couldn't save %s: %s", ec.ed.bodybuf.ShortName(), err.Error()))
	}
	ec.ed.BufferRefresh(false)
}

func PutallCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	t := "Putall: Saving the following files failed:\n"
	nerr := 0
	for _, col := range wnd.cols.cols {
		for _, ed := range col.editors {
			if (ed.bodybuf.Name[0] != '+') && ed.bodybuf.Modified {
				err := ed.bodybuf.Put()
				if err != nil {
					t += ed.bodybuf.ShortName() + ": " + err.Error() + "\n"
					nerr++
				}
				ed.BufferRefresh(false)
			}
		}
	}
	if nerr != 0 {
		Warn(t)
	}
}

func RedoCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	ec.buf.Undo(ec.fr.Sels, true)
	ec.br.BufferRefresh(ec.ontag)
}

func SendCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	//TODO: append
}

func SortCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.col == nil {
		return
	}

	sort.Sort((*Editors)(&ec.col.editors))
	ec.col.RecalcRects()
	ec.col.Redraw()
	wnd.wnd.FlushImage()
}

func UndoCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	ec.buf.Undo(ec.fr.Sels, false)
	ec.br.BufferRefresh(ec.ontag)
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

	wd := wnd.tagbuf.Dir
	if ec.buf != nil {
		wd = ec.buf.Dir
	}

	NewJob(wd, arg, "", &ec, false)
}

func PipeOutCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false

	wd := wnd.tagbuf.Dir
	if ec.buf != nil {
		wd = ec.buf.Dir
	}

	txt := string(buf.ToRunes(ec.ed.bodybuf.SelectionX(ec.fr.Sels[0])))
	NewJob(wd, arg, txt, &ec, false)
}

func CdCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	os.Chdir(arg)
	wd, _ := os.Getwd()

	wnd.tagbuf.Dir = wd

	for _, col := range wnd.cols.cols {
		col.tagbuf.Dir = wd
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

type Editors []*Editor

func (ev *Editors) Len() int {
	return len(*ev)
}

func (ev *Editors) Less(i, j int) bool {
	return (*ev)[i].bodybuf.Name < (*ev)[j].bodybuf.Name
}

func (ev *Editors) Swap(i, j int) {
	e := (*ev)[i]
	(*ev)[i] = (*ev)[j]
	(*ev)[j] = e
}
