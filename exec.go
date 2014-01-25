package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"yacco/buf"
	"yacco/config"
	"yacco/edit"
	"yacco/textframe"
	"yacco/util"
)

type ExecContext struct {
	col *Col
	ed  *Editor

	br        BufferRefreshable
	ontag     bool
	fr        *textframe.Frame
	buf       *buf.Buffer
	eventChan chan string

	dir string
}

var KeyBindings = map[string]func(ec ExecContext){}

type Cmd func(ec ExecContext, arg string)

var cmds = map[string]Cmd{
	"Cut":    func(ec ExecContext, arg string) { CopyCmd(ec, arg, true) },
	"Get":    GetCmd,
	"Del":    func(ec ExecContext, arg string) { DelCmd(ec, arg, false) },
	"Delcol": DelcolCmd,
	"Delete": func(ec ExecContext, arg string) { DelCmd(ec, arg, true) },
	"Dump":   DumpCmd,
	"Edit":   EditCmd,
	"Exit":   ExitCmd,
	"Kill":   KillCmd,
	"Setenv": SetenvCmd,
	"Look":   LookCmd,
	"New":    NewCmd,
	"Newcol": NewcolCmd,
	"Paste":  func(ec ExecContext, arg string) { PasteCmd(ec, arg, false) },
	"Put":    PutCmd,
	"Putall": PutallCmd,
	"Redo":   RedoCmd,
	"Send":   SendCmd,
	"Snarf":  func(ec ExecContext, arg string) { CopyCmd(ec, arg, false) },
	"Copy":   func(ec ExecContext, arg string) { CopyCmd(ec, arg, false) },
	"Sort":   SortCmd,
	"Undo":   UndoCmd,
	"Zerox":  ZeroxCmd,
	"|":      PipeCmd,
	"<":      PipeInCmd,
	">":      PipeOutCmd,

	// New
	"Cd":            CdCmd,
	"Jobs":          JobsCmd,
	"Look!Again":    LookAgainCmd,
	"Look!Quit":     func(ec ExecContext, arg string) { SpecialSendCmd(ec, "!Quit") },
	"Look!Prev":     func(ec ExecContext, arg string) { SpecialSendCmd(ec, "!Prev") },
	"Paste!Primary": func(ec ExecContext, arg string) { PasteCmd(ec, arg, true) },
	"Paste!Indent":  PasteIndentCmd,
	"Rename":        RenameCmd,
	"Jump":          JumpCmd,
	"Getall":        GetallCmd,
}

var macros = map[string]Cmd{}

var spacesRe = regexp.MustCompile("\\s+")
var exitConfirmed = false

func init() {
	// this would otherwise cause an initialization loop
	cmds["Do"] = DoCmd
	cmds["Macro"] = MacroCmd
	cmds["LookFile"] = LookFileCmd
	cmds["Load"] = LoadCmd
	cmds["Builtin"] = BuiltinCmd
}

func fakebuf(name string) bool {
	return (len(name) == 0) || (name[0] == '+') || (name[len(name)-1] == '/')
}

func IntlCmd(cmd string) (Cmd, string, string, bool) {
	if len(cmd) <= 0 {
		return nil, "", "", true
	}

	if (cmd[0] == '<') || (cmd[0] == '>') || (cmd[0] == '|') {
		return cmds[cmd[:1]], cmd[1:], cmd[:1], true
	} else {
		v := spacesRe.Split(cmd, 2)
		if f, ok := macros[v[0]]; ok {
			arg := ""
			if len(v) > 1 {
				arg = v[1]
			}
			return f, arg, v[0], true
		} else if f, ok := cmds[v[0]]; ok {
			arg := ""
			if len(v) > 1 {
				arg = v[1]
			}
			return f, arg, v[0], true
		} else {
			return nil, "", "", false
		}
	}
}

func execGuard() {
	if r := recover(); r != nil {
		errmsg := fmt.Sprintf("%v\n", r)
		if config.EditErrorTrace {
			for i := 1; ; i++ {
				_, file, line, ok := runtime.Caller(i)
				if !ok {
					break
				}
				errmsg += fmt.Sprintf("  %s:%d\n", file, line)
			}
		}
		Warn(errmsg)
	}
}

func Exec(ec ExecContext, cmd string) {
	defer execGuard()
	execNoDefer(ec, cmd)
}

func execNoDefer(ec ExecContext, cmd string) {
	cmd = strings.TrimSpace(cmd)
	xcmd, arg, _, isintl := IntlCmd(cmd)
	if isintl {
		if xcmd != nil {
			xcmd(ec, arg)
		}
	} else {
		ExtExec(ec, cmd)
	}
}

func ExtExec(ec ExecContext, cmd string) {
	wd := Wnd.tagbuf.Dir
	if ec.dir != "" {
		wd = ec.dir
	}
	NewJob(wd, cmd, "", &ec, false, nil)
}

func BuiltinCmd(ec ExecContext, arg string) {
	execNoDefer(ec, arg)
}

func CopyCmd(ec ExecContext, arg string, del bool) {
	exitConfirmed = false
	if ec.ed != nil {
		ec.ed.confirmDel = false
		ec.ed.confirmSave = false
	}
	if (ec.buf == nil) || (ec.fr == nil) || (ec.br == nil) {
		return
	}

	s := string(ec.buf.SelectionRunes(ec.fr.Sels[0]))
	if s == "" {
		// Does not trash clipboard when accidentally copying nil text
		return
	}
	if del {
		ec.buf.Replace([]rune{}, &ec.fr.Sels[0], true, ec.eventChan, util.EO_MOUSE, true)
		ec.br.BufferRefresh(ec.ontag)
	}
	Wnd.wnd.SetClipboard(s)
	Wnd.wnd.SetPrimarySelection(s)
}

func DelCmd(ec ExecContext, arg string, confirmed bool) {
	exitConfirmed = false
	if !ec.ed.bodybuf.Modified || fakebuf(ec.ed.bodybuf.Name) || confirmed || ec.ed.confirmDel {
		if ec.ed.eventChan != nil {
			close(ec.ed.eventChan)
			ec.ed.eventChan = nil
		}
		col := ec.ed.Column()
		col.Remove(col.IndexOf(ec.ed))
		ec.ed.Close()
		removeBuffer(ec.ed.bodybuf)
		Wnd.wnd.FlushImage()
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
		if ed.bodybuf.Modified && !fakebuf(ed.bodybuf.Name) && !ed.confirmDel {
			ed.confirmDel = true
			t += ed.bodybuf.ShortName() + "\n"
			n++
		}
	}

	if n == 0 {
		for _, ed := range ec.col.editors {
			removeBuffer(ed.bodybuf)
		}
		Wnd.cols.Remove(Wnd.cols.IndexOf(ec.col))
		Wnd.wnd.FlushImage()
	} else {
		Warn(t)
	}
}

func DumpCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	dumpDest := getDumpPath(arg, true)
	if DumpTo(dumpDest) {
		AutoDumpPath = dumpDest
		setDumpTitle()
	}
}

func LoadCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	dumpDest := getDumpPath(arg, false)
	if LoadFrom(dumpDest) {
		AutoDumpPath = dumpDest
		setDumpTitle()
	}
}

func getDumpPath(arg string, dodef bool) string {
	dumpDest := strings.TrimSpace(arg)
	if dumpDest == "" {
		if AutoDumpPath != "" {
			return AutoDumpPath
		}
		if !dodef {
			return ""
		}
		dumpDest = "default"
	}
	dumpDest = filepath.Join(os.Getenv("HOME"), ".config", "yacco", dumpDest+".dump")
	return dumpDest
}

func EditCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed != nil {
		ec.ed.confirmDel = false
		ec.ed.confirmSave = false
	}
	if (ec.buf == nil) || (ec.fr == nil) || (ec.br == nil) {
		return
	}
	var pj func() = nil
	if ec.ed != nil {
		pj = ec.ed.PushJump
	}

	edc := edit.EditContext{
		Buf:       ec.buf,
		Sels:      ec.fr.Sels,
		EventChan: ec.eventChan,
		PushJump:  pj,
	}

	edit.Edit(arg, edc)
	ec.br.BufferRefresh(ec.ontag)
}

func ExitCmd(ec ExecContext, arg string) {
	t := "The following files have unsaved changes:\n"
	n := 0
	for _, buf := range buffers {
		if buf == nil {
			continue
		}
		if buf.Modified && !fakebuf(buf.Name) {
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
	UpdateJobs(true)
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
	ec.ed.confirmSave = false
	if arg != "" {
		lookfwd(ec.ed, []rune(arg), true, true, Wnd.Prop["lookexact"] == "yes")
	} else {
		go lookproc(ec)
	}
}

func LookAgainCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	if ec.ed.specialChan != nil {
		ec.ed.specialChan <- "!Again"
	} else {
		lookfwd(ec.ed, lastNeedle, true, true, Wnd.Prop["lookexact"] == "yes")
	}
}

func SpecialSendCmd(ec ExecContext, msg string) {
	exitConfirmed = false
	if (ec.ed == nil) || (ec.ed.specialChan == nil) {
		return
	}
	ec.ed.confirmDel = false
	ec.ed.confirmSave = false
	ec.ed.specialChan <- msg
}

func GetCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	if ec.ed.bodybuf.Modified && !ec.ed.confirmDel {
		ec.ed.confirmDel = true
		Warn("File " + ec.ed.bodybuf.ShortName() + " has unsaved changes")
		return
	}

	if ec.ed.bodybuf.IsDir() {
		ec.ed.readDir()
	} else {
		ec.ed.bodybuf.Reload(true)
	}
	ec.ed.BufferRefresh(false)
}

func NewCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	arg = strings.TrimSpace(arg)
	if arg == "" {
		Warn("New: must specify argument")
		return
	}
	path := util.ResolvePath(ec.dir, arg)
	_, err := HeuristicOpen(path, true, true)
	if err != nil {
		Warn("New: " + err.Error())
	}
}

func NewcolCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	Wnd.cols.AddAfter(NewCol(Wnd.wnd, Wnd.cols.r), -1, 0.4)
	Wnd.wnd.FlushImage()
}

func PasteCmd(ec ExecContext, arg string, primary bool) {
	exitConfirmed = false
	if ec.ed != nil {
		ec.ed.confirmDel = false
		ec.ed.confirmSave = false
	}
	if (ec.buf == nil) || (ec.fr == nil) || (ec.br == nil) {
		return
	}
	var cb string
	if primary {
		cb = Wnd.wnd.GetPrimarySelection()
	} else {
		cb = Wnd.wnd.GetClipboard()
	}
	ec.buf.Replace([]rune(cb), &ec.fr.Sels[0], true, ec.eventChan, util.EO_MOUSE, true)
	ec.br.BufferRefresh(ec.ontag)
}

func PasteIndentCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed != nil {
		ec.ed.confirmDel = false
		ec.ed.confirmSave = false
	}
	if (ec.buf == nil) || (ec.fr == nil) || (ec.br == nil) {
		return
	}
	cb := Wnd.wnd.GetClipboard()

	if (ec.fr.Sels[0].S == 0) || (ec.fr.Sels[0].S != ec.fr.Sels[0].E) || (ec.ed == nil) || (ec.buf != ec.ed.bodybuf) {
		ec.buf.Replace([]rune(cb), &ec.fr.Sels[0], true, ec.eventChan, util.EO_MOUSE, true)
		ec.br.BufferRefresh(ec.ontag)
		return
	}

	failed := false
	tgtIndent := ""
tgtIndentSearch:
	for i := ec.fr.Sels[0].S - 1; i > 0; i-- {
		r := ec.buf.At(i).R
		switch r {
		case '\n':
			tgtIndent = string(ec.buf.SelectionRunes(util.Sel{i + 1, ec.fr.Sels[0].S}))
			break tgtIndentSearch
		case ' ', '\t':
			// continue
		default:
			failed = true
			break tgtIndentSearch
		}
	}

	if failed {
		ec.buf.Replace([]rune(cb), &ec.fr.Sels[0], true, ec.eventChan, util.EO_MOUSE, true)
		ec.br.BufferRefresh(ec.ontag)
		return
	}

	pasteLines := strings.Split(cb, "\n")
	srcIndent := ""
	for i := range pasteLines[0] {
		if (pasteLines[0][i] != ' ') && (pasteLines[0][i] != '\t') {
			srcIndent = pasteLines[0][:i]
			break
		}
	}

	for i := range pasteLines {
		if strings.HasPrefix(pasteLines[i], srcIndent) {
			if i == 0 {
				pasteLines[i] = pasteLines[i][len(srcIndent):]
			} else {
				pasteLines[i] = tgtIndent + pasteLines[i][len(srcIndent):]
			}
		} else {
			pasteLines[i] = pasteLines[i]
		}
	}

	ecb := strings.Join(pasteLines, "\n")
	ec.buf.Replace([]rune(ecb), &ec.fr.Sels[0], true, ec.eventChan, util.EO_MOUSE, true)
	ec.br.BufferRefresh(ec.ontag)
}

func PutCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	if fakebuf(ec.ed.bodybuf.Name) {
		return
	}

	if !ec.ed.confirmSave {
		if !ec.ed.bodybuf.CanSave() {
			ec.ed.confirmSave = true
			Warn(fmt.Sprintf("Put: %s changed on disk, are you sure you want to overwrite?", ec.ed.bodybuf.ShortName()))
			return
		}
	}
	err := ec.ed.bodybuf.Put()
	if err != nil {
		Warn(fmt.Sprintf("Put: Couldn't save %s: %s", ec.ed.bodybuf.ShortName(), err.Error()))
	}
	ec.ed.BufferRefresh(false)
	if AutoDumpPath != "" {
		DumpTo(AutoDumpPath)
	}
}

func PutallCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	t := "Putall: Saving the following files failed:\n"
	nerr := 0
	for _, col := range Wnd.cols.cols {
		for _, ed := range col.editors {
			if !fakebuf(ed.bodybuf.Name) && ed.bodybuf.Modified {
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

func GetallCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	t := "Getall: Not reloading the following modified buffers:\n"
	nerr := 0
	for _, col := range Wnd.cols.cols {
		for _, ed := range col.editors {
			if !fakebuf(ed.bodybuf.Name) {
				if ed.bodybuf.Modified {
					t += ed.bodybuf.ShortName() + "\n"
					nerr++
				} else {
					ed.bodybuf.Reload(true)
					ed.BufferRefresh(false)
				}
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
	ec.ed.confirmSave = false
	ec.buf.Undo(&ec.fr.Sels[0], true)
	ec.br.BufferRefresh(ec.ontag)
}

func SendCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	ec.ed.confirmSave = false
	txt := []rune{}
	if ec.ed.sfr.Fr.Sels[0].S != ec.ed.sfr.Fr.Sels[0].E {
		txt = ec.ed.bodybuf.SelectionRunes(ec.ed.sfr.Fr.Sels[0])
	} else {
		txt = []rune(Wnd.wnd.GetClipboard())
	}
	ec.ed.sfr.Fr.Sels[0] = util.Sel{ec.buf.Size(), ec.buf.Size()}
	ec.ed.bodybuf.Replace(txt, &ec.ed.sfr.Fr.Sels[0], true, ec.eventChan, util.EO_MOUSE, true)
	ec.ed.BufferRefresh(false)
}

func SortCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.col == nil {
		return
	}

	sort.Sort((*Editors)(&ec.col.editors))
	ec.col.RecalcRects()
	ec.col.Redraw()
	Wnd.wnd.FlushImage()
}

func UndoCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if (ec.ed == nil) || (ec.buf == nil) {
		return
	}
	ec.ed.confirmDel = false
	ec.ed.confirmSave = false
	ec.buf.Undo(&ec.fr.Sels[0], false)
	if ec.br != nil {
		ec.br.BufferRefresh(ec.ontag)
	}
}

func ZeroxCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	ed := ec.ed
	if ed == nil {
		ed = activeSel.ed
	}
	if ed == nil {
		return
	}
	ed.confirmDel = false
	ed.confirmSave = false
	ed.bodybuf.RefCount++
	ned := NewEditor(ed.bodybuf, true)
	HeuristicPlaceEditor(ned, true)
}

func PipeCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	ec.ed.confirmSave = false
	wd := Wnd.tagbuf.Dir
	if ec.buf != nil {
		wd = ec.buf.Dir
	}

	txt := string(ec.ed.bodybuf.SelectionRunes(ec.fr.Sels[0]))
	NewJob(wd, arg, txt, &ec, true, nil)
}

func PipeInCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	ec.ed.confirmSave = false

	wd := Wnd.tagbuf.Dir
	if ec.buf != nil {
		wd = ec.buf.Dir
	}

	NewJob(wd, arg, "", &ec, true, nil)
}

func PipeOutCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	ec.ed.confirmSave = false

	wd := Wnd.tagbuf.Dir
	if ec.buf != nil {
		wd = ec.buf.Dir
	}

	txt := string(ec.ed.bodybuf.SelectionRunes(ec.fr.Sels[0]))
	NewJob(wd, arg, txt, &ec, false, nil)
}

func cdIntl(arg string) {
	os.Chdir(arg)
	wd, _ := os.Getwd()

	Wnd.tagbuf.Dir = wd

	for _, col := range Wnd.cols.cols {
		col.tagbuf.Dir = wd
		for _, ed := range col.editors {
			ed.BufferRefresh(false)
		}
	}
}

func CdCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	arg = strings.TrimSpace(arg)
	cdIntl(arg)

	Wnd.GenTag()

	Wnd.BufferRefresh(true)

	Wnd.cols.Redraw()
	Wnd.tagfr.Redraw(false)
	Wnd.wnd.FlushImage()
}

func DoCmd(ec ExecContext, arg string) {
	cmds := strings.Split(arg, "\n")
	for _, cmd := range cmds {
		execNoDefer(ec, cmd)
	}
}

func MacroCmd(ec ExecContext, arg string) {
	cmds := strings.Split(arg, "\n")
	if len(cmds) <= 0 {
		return
	}

	name := strings.TrimSpace(cmds[0])
	cmds = cmds[1:]

	macros[name] = func(ec ExecContext, arg string) {
		for _, cmd := range cmds {
			execNoDefer(ec, cmd)
		}
	}
}

func RenameCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed == nil {
		return
	}
	ec.ed.confirmDel = false
	ec.ed.confirmSave = false

	ec.ed.bodybuf.Name = arg
	ec.ed.bodybuf.Modified = true
	ec.ed.BufferRefresh(false)
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

func LookFileCmd(ec ExecContext, arg string) {
	ed, err := EditFind(Wnd.tagbuf.Dir, "+LookFile", true, true)
	if err != nil {
		Warn(err.Error())
		return
	}

	if ed.specialChan == nil {
		lookFile(ed)
	} else {
		ed.tagfr.Sels[0] = util.Sel{ed.tagbuf.EditableStart, ed.tagbuf.Size()}
	}
}

func JumpCmd(ec ExecContext, arg string) {
	if ec.ed == nil {
		return
	}
	if strings.ToLower(arg) == "tip" {
		ec.ed.sfr.Fr.Sels[0].S = ec.ed.otherSel[OS_TIP].E
		ec.ed.sfr.Fr.Sels[0].E = ec.ed.otherSel[OS_TIP].E
	} else {
		ec.ed.confirmDel = false
		ec.ed.confirmSave = false
		ec.ed.RestoreJump()
		ec.ed.BufferRefresh(false)
	}
}

func KeysInit() {
	for k := range config.KeyBindings {
		KeyBindings[k] = CompileCmd(config.KeyBindings[k])
	}
}

func CompileCmd(cmdstr string) func(ec ExecContext) {
	xcmd, arg, cmdname, isintl := IntlCmd(cmdstr)
	if !isintl {
		return func(ec ExecContext) {
			defer execGuard()
			ExtExec(ec, cmdstr)
		}
	} else if cmdname == "Edit" {
		pgm := edit.Parse([]rune(arg))
		return func(ec ExecContext) {
			defer execGuard()

			if (ec.buf == nil) || (ec.fr == nil) {
				return
			}

			var pj func() = nil
			if ec.ed != nil {
				pj = ec.ed.PushJump
			}

			edc := edit.EditContext{
				Buf:       ec.buf,
				Sels:      ec.fr.Sels,
				EventChan: ec.eventChan,
				PushJump:  pj,
			}
			pgm.Exec(edc)
			ec.br.BufferRefresh(ec.ontag)
		}
	} else if cmdname == "Do" {
		cmds := strings.Split(arg, "\n")
		fcmds := make([]func(ec ExecContext), len(cmds))
		for i := range cmds {
			fcmds[i] = CompileCmd(cmds[i])
		}
		return func(ec ExecContext) {
			for _, fcmd := range fcmds {
				fcmd(ec)
			}
		}
	} else if xcmd == nil {
		return func(ec ExecContext) {}
	} else {
		return func(ec ExecContext) {
			defer execGuard()
			xcmd(ec, arg)
		}
	}
}
