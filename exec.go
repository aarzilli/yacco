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

	dir       string
	norefresh bool
}

var KeyBindings = map[string]func(ec ExecContext){}

type Cmd func(ec ExecContext, arg string)

var cmds = map[string]Cmd{}

var macros = map[string]Cmd{}

var spacesRe = regexp.MustCompile("\\s+")
var exitConfirmed = false

func init() {
	cmds["Cut"] = func(ec ExecContext, arg string) { CopyCmd(ec, arg, true) }
	cmds["Get"] = GetCmd
	cmds["Del"] = func(ec ExecContext, arg string) { DelCmd(ec, arg, false) }
	cmds["Delcol"] = DelcolCmd
	cmds["Delete"] = func(ec ExecContext, arg string) { DelCmd(ec, arg, true) }
	cmds["Dump"] = DumpCmd
	cmds["Edit"] = EditCmd
	cmds["Exit"] = ExitCmd
	cmds["Kill"] = KillCmd
	cmds["Setenv"] = SetenvCmd
	cmds["Look"] = LookCmd
	cmds["New"] = NewCmd
	cmds["Newcol"] = NewcolCmd
	cmds["Paste"] = PasteCmd
	cmds["Put"] = PutCmd
	cmds["Putall"] = PutallCmd
	cmds["Redo"] = RedoCmd
	cmds["Send"] = SendCmd
	cmds["Snarf"] = func(ec ExecContext, arg string) { CopyCmd(ec, arg, false) }
	cmds["Copy"] = func(ec ExecContext, arg string) { CopyCmd(ec, arg, false) }
	cmds["Sort"] = SortCmd
	cmds["Undo"] = UndoCmd
	cmds["Zerox"] = ZeroxCmd
	cmds["|"] = PipeCmd
	cmds["<"] = PipeInCmd
	cmds[">"] = PipeOutCmd

	// New
	cmds["Cd"] = CdCmd
	cmds["Jobs"] = JobsCmd
	cmds["Look!Again"] = LookAgainCmd
	cmds["Look!Quit"] = func(ec ExecContext, arg string) { SpecialSendCmd(ec, "!Quit") }
	cmds["Look!Prev"] = func(ec ExecContext, arg string) { SpecialSendCmd(ec, "!Prev") }
	/*cmds["Paste!Primary"] = func(ec ExecContext, arg string) { PasteCmd(ec, arg, true) }
	cmds["Paste!Indent"] = PasteIndentCmd*/
	cmds["Jump"] = JumpCmd
	cmds["Getall"] = GetallCmd
	cmds["Rename"] = RenameCmd
	cmds["Rehash"] = RehashCmd
	cmds["Do"] = DoCmd
	cmds["LookFile"] = LookFileCmd
	cmds["Load"] = LoadCmd
	cmds["Builtin"] = BuiltinCmd
	cmds["Debug"] = DebugCmd
	cmds["Help"] = HelpCmd
	cmds["Theme"] = ThemeCmd
}

func HelpCmd(ec ExecContext, arg string) {
	switch arg {
	case "Edit":
		Warn(`
== Commands ==

<addr>a/<text>/
	Insert after
<addr>c/<text>/
	Replace
<addr>i/<text>/
	Insert before
<addr>d
	Delete addr

<addr>s[<num>]/<regexp>/<text>/[g]
	Replace all instances of <regexp> with <text>. If <num> is specified replaces only <num>-th occourence of <regexp>

<addr>m<addr>
	Move from one address to another
<addr>t<addr>
	Swap contents of the two addresses
	
<addr>p
	Print contents of address
<addr>=
	Print current line/column
	
<addr>x/<regexp>/<command>
	Executes command for every match of <regexp>
<addr>y/<regexp>/<command>
	Executes command for every sequence of text delimited by <regexp>
<addr>g/<regexp>/<command>
	Executes command if the address matches <regexp>
<addr>v/<regexp>/<command>
	Opposite of g

<addr>"<"<command>	
<addr>">"<command>	
<addr>"|"<command>	
	Executes external commands

<addr>k
	Saves address as current selection
	
== Addresses ==
The initial <addr> can always be omitted, if it is it will default to "."

Simple Addresses:
#n			empty string after n-th character
n			n-th line
0			the point before the first character of the file
$			the point after the last character
#wn		empty string after the n-th word
#?n			don't use this
.			whatever is currently selected
/regexp/
?regexp?	forward or backward lookup for regexp match
/@regexp/
?@regexp?	just like /regexp/ and ?regexp? but suppresses errors

Compound Addresses
a1+a2		address a2 evaluated starting at the end of a1
a1-a2		address a2 evaluated looking in the reverse direction starting at the beginning of a1
a1,a2		substring starting at the start of a1 and ending at the end of a2
a1;a2		like a1,a2 but with a2 evaluated after a1

For + and - if a2 is missing it defaults to "1", if a1 is missing it defaults to ".".
For , and ; if a2 is missing it defaults to "$", if a1 is missing it defaults to "0".
The address "," represents the whole file.
`)
	default:
		Warn(`
== Mouse ==

Select = left click (and drag)
Execute = middle click, control + left click
Search = right click, alt + left click
Execute with argument = shift + middle click, super + left click, control + right click, shift+control+left click

== Files ==
Get
Put
Putall
Getall
Exit

== Editing ==
Undo
Redo
Edit <…>		Runs sed-like editing commands, see Help Edit
Look [<text>]	Search <text> or starts interactive search

== Frames and Columns ==
New
Del
Delete			Like Del but can not be blocked by an attached process
Newcol
Delcol
Zerox			Duplicates current frame
Sort			Sort frames in current column alphabetically
Rename <name>
LookFile			Opens special frame to search and open files interactively

== Clipboard ==
Cut
Copy
Snarf			Same as Copy
Paste [primary|indent]

== Session ==
Dump [<name>]	Starts saving session to <name>
Load [<name>]	Loads session from <name> (omit for a list of sessions)

== Jobs ==
| <ext. cmd.>	Runs selection through <ext. cmd.> replaces with output
> <ext. cmd.>	Runs selection through <ext. cmd.>
< <ext. cmd.>	Replaces selection with output of <ext. cmd.>
Jobs			Lists currently running jobs
Kill [<jobnum>]	Kill all jobs (or the one specified)
Setenv <var> <val>
Cd <dir>

== External Utilities ==
E <file>			Edits file
Watch <cmd>	Executes command every time a file changes in current directory
win <cmd>		Runs cmd within pty
y9p				Filesystem interface access
Font			Toggles alternate font
Fs				Removes redundant spaces in current file
Indent			Controls automatic indent and tab key behaviour
Tab				Controls tab character width
LookExact		Toggles smart case
Mount			Invokes p9fuse
a+, a-			indents/removes indent from selection
g				recursive grep
in <dir> <cmd>	execute <cmd> in <dir>

== Misc ==
Do <…>			Executes sequence of commands, one per line
Rehash			Recalculates completions
Send			Inserts clipboard or last selection in buffer
Builtin <…>		Runs command as builtin (skip attached processes)
Debug <…>		Run without arguments for informations
Jump			Cycles through insetion points
`)
	}
}

func fakebuf(name string) bool {
	return (len(name) == 0) || (name[0] == '+') || (name[0] == '-') || (name[len(name)-1] == '/')
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
		ExtExec(ec, cmd, true)
	}
}

func ExtExec(ec ExecContext, cmd string, dolog bool) {
	wd := Wnd.tagbuf.Dir
	if ec.dir != "" {
		wd = ec.dir
	}
	if dolog {
		LogExec(cmd, wd)
	}
	NewJob(wd, cmd, "", &ec, false, nil)
}

func BuiltinCmd(ec ExecContext, arg string) {
	execNoDefer(ec, arg)
}

func col2active(ec *ExecContext) {
	if activeSel.zeroxEd == nil {
		return
	}

	ec.ed = activeSel.zeroxEd
	ec.br = activeSel.zeroxEd
	ec.ontag = false
	ec.fr = &ec.ed.sfr.Fr
	ec.buf = ec.ed.bodybuf
	ec.dir = ec.ed.bodybuf.Dir
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
		ec.buf.Replace([]rune{}, &ec.fr.Sels[0], true, ec.eventChan, util.EO_MOUSE)
		if !ec.norefresh {
			ec.br.BufferRefresh(ec.ontag)
		}
	}
	Wnd.wnd.SetClipboard(s)
	Wnd.wnd.SetPrimarySelection(s)
}

func DelCmd(ec ExecContext, arg string, confirmed bool) {
	exitConfirmed = false
	clearToRemove := !ec.ed.bodybuf.Modified || fakebuf(ec.ed.bodybuf.Name) || confirmed || ec.ed.confirmDel
	if !clearToRemove {
		clearToRemove = ec.ed.bodybuf.RefCount > 1
	}
	if clearToRemove {
		if ec.ed.eventChan != nil {
			close(ec.ed.eventChan)
			ec.ed.eventChan = nil
		}
		Log(bufferIndex(ec.ed.bodybuf), LOP_DEL, ec.ed.bodybuf)
		col := ec.ed.Column()
		col.Remove(col.IndexOf(ec.ed))
		ec.ed.Close()
		removeBuffer(ec.ed.bodybuf)
		Wnd.wnd.FlushImage(col.r)
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
		ec.col.Close()
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

type fileInfoSortByTime []os.FileInfo

func (f fileInfoSortByTime) Len() int {
	return len(f)
}

func (f fileInfoSortByTime) Less(i, j int) bool {
	return f[i].ModTime().Unix() >= f[j].ModTime().Unix()
}

func (f fileInfoSortByTime) Swap(i, j int) {
	t := f[i]
	f[i] = f[j]
	f[j] = t
}

func LoadCmd(ec ExecContext, arg string) {
	exitConfirmed = false

	if strings.TrimSpace(arg) == "" {
		wd, _ := os.Getwd()
		EditFind(wd, "+Dumps", false, false)

		dh, err := os.Open(os.ExpandEnv("$HOME/.config/yacco/"))
		if err == nil {
			defer dh.Close()
			var fis fileInfoSortByTime
			fis, err := dh.Readdir(-1)
			if err != nil {
				fis = []os.FileInfo{}
			}
			sort.Sort(fis)
			r := []string{}
			for i := range fis {
				n := fis[i].Name()
				if !strings.HasSuffix(n, ".dump") {
					continue
				}
				r = append(r, fmt.Sprintf("Load %s", n[:len(n)-len(".dump")]))
			}
			Warnfull("+Dumps", strings.Join(r, "\n"), false, false)
			Warnfull("+Dumps", "\n", false, false)
		}
	} else {
		dumpDest := getDumpPath(arg, false)
		if LoadFrom(dumpDest) {
			AutoDumpPath = dumpDest
			setDumpTitle()
		}
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
	if !ec.norefresh {
		ec.br.BufferRefresh(false)
	}
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
		jobKillLast()
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
		ec.fr = &ec.ed.sfr.Fr
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
	if ec.ed.bodybuf.Modified && !ec.ed.confirmDel && !ec.ed.bodybuf.IsDir() {
		ec.ed.confirmDel = true
		Warn("File " + ec.ed.bodybuf.ShortName() + " has unsaved changes")
		return
	}

	Log(bufferIndex(ec.ed.bodybuf), LOP_GET, ec.ed.bodybuf)

	if ec.ed.bodybuf.IsDir() {
		ec.ed.readDir()
	} else {
		ec.ed.bodybuf.Reload(true)
	}
	if !ec.norefresh {
		ec.ed.BufferRefresh(false)
	}
}

func NewCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	arg = strings.TrimSpace(arg)
	if arg == "" {
		arg = "+New"
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

func PasteCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.ed != nil {
		ec.ed.confirmDel = false
		ec.ed.confirmSave = false
	}
	if (ec.buf == nil) || (ec.fr == nil) || (ec.br == nil) {
		return
	}
	var cb string

	switch arg {
	case "Indent", "indent":
		PasteIndentCmd(ec, arg)
		return
	case "Primary", "primary":
		cb = Wnd.wnd.GetPrimarySelection()
	default:
		cb = Wnd.wnd.GetClipboard()
	}

	ec.buf.Replace([]rune(cb), &ec.fr.Sels[0], true, ec.eventChan, util.EO_MOUSE)
	if !ec.norefresh {
		ec.br.BufferRefresh(ec.ontag)
	}
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
		ec.buf.Replace([]rune(cb), &ec.fr.Sels[0], true, ec.eventChan, util.EO_MOUSE)
		if !ec.norefresh {
			ec.br.BufferRefresh(ec.ontag)
		}
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
		ec.buf.Replace([]rune(cb), &ec.fr.Sels[0], true, ec.eventChan, util.EO_MOUSE)
		if !ec.norefresh {
			ec.br.BufferRefresh(ec.ontag)
		}
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
	ec.buf.Replace([]rune(ecb), &ec.fr.Sels[0], true, ec.eventChan, util.EO_MOUSE)
	if !ec.norefresh {
		ec.br.BufferRefresh(ec.ontag)
	}
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
	Log(bufferIndex(ec.ed.bodybuf), LOP_PUT, ec.ed.bodybuf)
	err := ec.ed.bodybuf.Put()
	if err != nil {
		Warn(fmt.Sprintf("Put: Couldn't save %s: %s", ec.ed.bodybuf.ShortName(), err.Error()))
	}
	if !ec.norefresh {
		ec.ed.BufferRefresh(false)
	}
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
				if !ec.norefresh {
					ed.BufferRefresh(false)
				}
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
					if !ec.norefresh {
						ed.BufferRefresh(false)
					}
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
	if !ec.norefresh {
		ec.br.BufferRefresh(ec.ontag)
	}
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
	ec.ed.bodybuf.Replace(txt, &ec.ed.sfr.Fr.Sels[0], true, ec.eventChan, util.EO_MOUSE)
	if !ec.norefresh {
		ec.ed.BufferRefresh(false)
	}
}

func SortCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.col == nil {
		return
	}

	sort.Sort((*Editors)(&ec.col.editors))
	ec.col.RecalcRects(ec.col.last)
	ec.col.Redraw()
	Wnd.wnd.FlushImage(ec.col.r)
}

func UndoCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if (ec.ed == nil) || (ec.buf == nil) {
		return
	}
	ec.ed.confirmDel = false
	ec.ed.confirmSave = false
	ec.buf.Undo(&ec.fr.Sels[0], false)
	if ec.br != nil && !ec.norefresh {
		ec.br.BufferRefresh(ec.ontag)
	}
}

func ZeroxCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	ed := ec.ed
	if ed == nil {
		ed = activeSel.zeroxEd
	}
	if ed == nil {
		return
	}
	ed.confirmDel = false
	ed.confirmSave = false
	ned := NewEditor(ed.bodybuf, true)
	ned.sfr.Fr.Sels[0].S = ed.sfr.Fr.Sels[0].S
	ned.sfr.Fr.Sels[0].E = ed.sfr.Fr.Sels[0].E
	Log(bufferIndex(ed.bodybuf), LOP_ZEROX, ed.bodybuf)
	HeuristicPlaceEditor(ned, true)
}

func PipeCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	col2active(&ec)
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
	col2active(&ec)
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
	col2active(&ec)
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

	if ec.buf != nil {
		arg = util.ResolvePath(ec.buf.Dir, arg)
	} else {
		arg = util.ResolvePath(Wnd.tagbuf.Dir, arg)
	}

	cdIntl(arg)

	Wnd.GenTag()

	pwd, _ := os.Getwd()
	pwd = util.ShortPath(pwd, false)
	Wnd.wnd.SetTitle("Yacco " + pwd)

	Wnd.BufferRefresh(true)

	Wnd.cols.Redraw()
	Wnd.tagfr.Redraw(false, nil)
	Wnd.wnd.FlushImage()
}

func DoCmd(ec ExecContext, arg string) {
	cmds := strings.Split(arg, "\n")
	ec.norefresh = true
	for i, cmd := range cmds {
		if i == len(cmds)-1 {
			ec.norefresh = false
		}
		execNoDefer(ec, cmd)
	}
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
	}
	ec.ed.BufferRefresh(false)
}

func KeysInit() {
	for k := range config.KeyBindings {
		KeyBindings[k] = CompileCmd(config.KeyBindings[k])
		maybeAddSelExtension(k, config.KeyBindings[k])
	}
}

// Adds to KeyBindings a version of cmdstr with +shift+ that extends the current selection
func maybeAddSelExtension(k, cmdstr string) {
	// if there is already a shift in the modifier list we can not add a shifted version
	kcomps := strings.Split(k, "+")
	for _, kcomp := range kcomps {
		if kcomp == "shift" {
			return
		}
	}

	_, arg, cmdname, isintl := IntlCmd(cmdstr)

	if !isintl || (cmdname != "Edit") {
		return
	}

	pgm := edit.Parse([]rune(arg))
	pgm = edit.ToMark(pgm)
	if pgm == nil {
		return
	}

	kcomps = append(kcomps, kcomps[len(kcomps)-1])
	kcomps[len(kcomps)-2] = "shift"
	sort.Strings(kcomps[:len(kcomps)-1])
	newk := strings.Join(kcomps, "+")

	KeyBindings[newk] = editPgmToFunc(pgm)
}

func CompileCmd(cmdstr string) func(ec ExecContext) {
	xcmd, arg, cmdname, isintl := IntlCmd(cmdstr)
	if !isintl {
		return func(ec ExecContext) {
			defer execGuard()
			ExtExec(ec, cmdstr, false)
		}
	} else if cmdname == "Edit" {
		pgm := edit.Parse([]rune(arg))
		return editPgmToFunc(pgm)
	} else if cmdname == "Do" {
		cmds := strings.Split(arg, "\n")
		fcmds := make([]func(ec ExecContext), len(cmds))
		for i := range cmds {
			fcmds[i] = CompileCmd(cmds[i])
		}
		return func(ec ExecContext) {
			ec.norefresh = true
			for i, fcmd := range fcmds {
				if i == len(fcmds)-1 {
					ec.norefresh = false
				}
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

func editPgmToFunc(pgm *edit.Cmd) func(ec ExecContext) {
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
		if !ec.norefresh {
			ec.br.BufferRefresh(ec.ontag)
		}
	}
}

func RenameCmd(ec ExecContext, arg string) {
	exitConfirmed = false
	if ec.buf == nil {
		return
	}

	if ec.br == nil {
		return
	}

	if ec.ed != nil {
		ec.ed.confirmDel = false
		ec.ed.confirmSave = false
	}

	newName := strings.TrimSpace(arg)
	abspath := util.ResolvePath(ec.buf.Dir, newName)
	ec.buf.Name = filepath.Base(abspath)
	ec.buf.Dir = filepath.Dir(abspath)
	if newName[len(newName)-1] == '/' {
		ec.buf.Name += "/"
	}
	ec.buf.Modified = true
	if !ec.norefresh {
		ec.br.BufferRefresh(false)
	}
}

func RehashCmd(ec ExecContext, arg string) {
	if ec.ed != nil {
		ec.ed.bodybuf.UpdateWords()
	} else {
		for i := range buffers {
			if buffers[i] != nil {
				buffers[i].UpdateWords()
			}
		}
	}
}

func ThemeCmd(ec ExecContext, arg string) {
	if arg == "" {
		Warn(`
Theme standard
Theme evening
Theme evening2
Theme midnight
Theme bw
`)
		return
	}
	setTheme(arg)
	Wnd.Resized()
}

func DebugCmd(ec ExecContext, arg string) {
	usage := func() {
		Warn(`Debug command help:
Debug trace
	Enables/disables trace on Edit errors
	
Debug compile <command>
	Compiles Edit command, shows the result of the compilation
`)
	}

	v := strings.SplitN(arg, " ", 2)

	if len(v) < 1 {
		usage()
		return
	}

	switch v[0] {
	case "trace":
		if len(v) != 1 {
			usage()
			return
		}
		config.EditErrorTrace = !config.EditErrorTrace
		if config.EditErrorTrace {
			Warn("Trace = on\n")
		} else {
			Warn("Trace = off\n")
		}
	case "compile":
		if len(v) != 2 {
			usage()
			return
		}
		pgm := edit.Parse([]rune(v[1]))
		Warn(pgm.String())
	default:
		usage()
		return
	}
}
