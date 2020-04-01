package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/aarzilli/go2def"
	"github.com/lionkov/go9p/p"
	"github.com/lionkov/go9p/p/clnt"

	"github.com/aarzilli/yacco/extra/Go/rename"
	"github.com/aarzilli/yacco/util"
)

const debug = false

func usage() {
	fmt.Fprintf(os.Stderr, `Implements Go integration in yacco:
	
	Go gurucmd	calls guru on the selection of active editor gurucmd is one of:
			callees callers callstack definition describe freevars implements
			peers pointsto  referrers what whicherrs
	Go d		equivalent of "Go describe"
	Go r		equivalent of "Go referrers"
	Go help		list of commands
`)
	os.Exit(1)
}

func runGofmt(argument string, paths map[string]bool) {
	wd, err := os.Getwd()
	util.Allergic(debug, err)
	args := []string{"fmt"}
	if argument != "" {
		args = append(args, argument)
	}
	out, err := exec.Command("go", args...).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "go %s\n%s\n%v\n", strings.Join(args, " "), string(out), err)
	}
	for _, path := range strings.Split(string(out), "\n") {
		paths[filepath.Join(wd, path)] = true
	}
}

func gitModifiedPaths() []string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").CombinedOutput()
	if err != nil {
		return nil
	}
	rootdir := strings.TrimSpace(string(out))
	out, err = exec.Command("git", "status", "--porcelain").CombinedOutput()
	if err != nil {
		return nil
	}
	r := []string{}
	for _, entry := range strings.Split(string(out), "\n") {
		if len(entry) < 3 {
			continue
		}
		r = append(r, filepath.Join(rootdir, entry[3:]))
	}
	return r
}

func gofmt() {
	paths := map[string]bool{}
	gitpaths := gitModifiedPaths()
	if gitpaths == nil {
		runGofmt("", paths)
	} else {
		for _, gitpath := range gitpaths {
			if strings.HasSuffix(gitpath, ".go") {
				runGofmt(gitpath, paths)
			}
		}
	}
	getChangedPaths(paths)
}

func gorename() {
	vpaths := rename.Auto()
	paths := make(map[string]bool)
	for _, path := range vpaths {
		paths[path] = true
	}
	getChangedPaths(paths)
}

func getChangedPaths(paths map[string]bool) {
	p9clnt, err := util.YaccoConnect()
	util.Allergic(debug, err)
	defer p9clnt.Unmount()
	index, err := util.ReadIndex(p9clnt)
	util.Allergic(debug, err)
	for _, ie := range index {
		if _, loaded := paths[ie.Path]; loaded {
			if ctlfd, err := p9clnt.FOpen(fmt.Sprintf("/%d/ctl", ie.Idx), p.OWRITE); err == nil {
				ctlfd.Write([]byte("get"))
				ctlfd.Close()
			}
		}
	}
}

func readlast(p9clnt *clnt.Clnt) int {
	if bi := os.Getenv("bi"); bi != "" {
		idx, _ := strconv.Atoi(bi)
		return idx
	}
	fh, err := p9clnt.FOpen("/last", p.OREAD)
	util.Allergic(debug, err)
	bs, err := ioutil.ReadAll(fh)
	fh.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "/last: %v\n", err)
		os.Exit(1)
	}
	v := strings.Split(string(bs), " ")
	idx, _ := strconv.Atoi(v[0])
	if idx < 0 {
		// nothing to read
		os.Exit(0)
	}
	return idx
}

func readaddr(p9clnt *clnt.Clnt, idx int) (pos, filename, body string, intpos [2]int) {
	ctlfd, err := p9clnt.FOpen(fmt.Sprintf("/%d/ctl", idx), p.ORDWR)
	util.Allergic(debug, err)
	ctlbs, err := ioutil.ReadAll(ctlfd)
	filename = strings.TrimSpace(string(ctlbs[12*8:]))
	ctlfd.Write([]byte("addr=dot"))
	ctlfd.Close()

	addrfd, err := p9clnt.FOpen(fmt.Sprintf("/%d/byteaddr", idx), p.OREAD)
	util.Allergic(debug, err)
	bs, err := ioutil.ReadAll(addrfd)
	addrfd.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "addr: %v", err)
		os.Exit(1)
	}
	addrfields := strings.Split(string(bs), ",")
	s, _ := strconv.Atoi(addrfields[0])
	e, _ := strconv.Atoi(addrfields[1])

	bodyfd, err := p9clnt.FOpen(fmt.Sprintf("/%d/body", idx), p.OREAD)
	util.Allergic(debug, err)
	bodyb, _ := ioutil.ReadAll(bodyfd)
	bodyfd.Close()

	return fmt.Sprintf("%s:#%d,#%d", filename, s, e), filename, string(bodyb), [2]int{s, e}
}

func guru(arg string) {
	p9clnt, err := util.YaccoConnect()
	util.Allergic(debug, err)
	idx := readlast(p9clnt)
	pos, filename, body, _ := readaddr(p9clnt, idx)
	cmd := exec.Command("guru", "-modified", arg, pos)
	cmd.Stdin = modified(filename, body)
	bs, err := cmd.CombinedOutput()
	processout(bs, err, arg, idx, pos, false, p9clnt)
}

func go2defcmd() {
	p9clnt, err := util.YaccoConnect()
	util.Allergic(debug, err)
	idx := readlast(p9clnt)
	_, filename, body, pos := readaddr(p9clnt, idx)
	var buf bytes.Buffer
	go2def.Describe(filename, pos, &go2def.Config{Out: &buf, Modfiles: map[string][]byte{filename: []byte(body)}})

	var out io.Writer

	if os.Getenv("YACCO_TOOLTIP") != "1" {
		buf, _, err := util.FindWin("Guru", p9clnt)
		util.Allergic(debug, err)
		buf.CtlFd.Write([]byte("name +Guru\n"))
		buf.CtlFd.Write([]byte("show-nowarp\n"))
		buf.AddrFd.Write([]byte(","))
		buf.XDataFd.Write([]byte{0})
		buf.EventFd.Close()
		buf.AddrFd.Close()
		buf.XDataFd.Close()
		buf.TagFd.Close()
		buf.ColorFd.Close()
		defer buf.BodyFd.Close()
		out = writenWrapper{buf.BodyFd}
	} else {
		out = os.Stdout
	}

	out.Write(buf.Bytes())
}

func guruscope(arg string, scope string) {
	p9clnt, err := util.YaccoConnect()
	util.Allergic(debug, err)
	idx := readlast(p9clnt)
	pos, filename, body, _ := readaddr(p9clnt, idx)
	cmd := exec.Command("guru", "-modified", "-scope="+scope, arg, pos)
	cmd.Stdin = modified(filename, body)
	bs, err := cmd.CombinedOutput()
	processout(bs, err, arg, idx, pos, false, p9clnt)
}

func guruprepared(arg string, stridx, pos string) {
	p9clnt, err := util.YaccoConnect()
	util.Allergic(debug, err)
	idx, _ := strconv.Atoi(stridx)
	_, filename, body, _ := readaddr(p9clnt, idx)
	cmd := exec.Command("guru", arg, pos)
	cmd.Stdin = modified(filename, body)
	bs, err := cmd.CombinedOutput()
	processout(bs, err, arg, idx, pos, true, p9clnt)
}

func modified(filename, body string) io.Reader {
	return strings.NewReader(fmt.Sprintf("%s\n%d\n%s", filename, len(body), body))
}

type writenWrapper struct {
	f *clnt.File
}

func (w writenWrapper) Write(p []byte) (int, error) {
	return w.f.Writen(p, 0)
}

func processout(bs []byte, err error, arg string, idx int, pos string, fullwrite bool, p9clnt *clnt.Clnt) {
	const (
		refToMethodFunc      = "reference to method func "
		refToIfaceMethodFunc = "reference to interface method func "
		refToFunc            = "reference to func "
	)

	var out io.Writer

	if os.Getenv("YACCO_TOOLTIP") != "1" {
		buf, _, err := util.FindWin("Guru", p9clnt)
		util.Allergic(debug, err)
		buf.CtlFd.Write([]byte("name +Guru\n"))
		buf.CtlFd.Write([]byte("show-nowarp\n"))
		buf.AddrFd.Write([]byte(","))
		buf.XDataFd.Write([]byte{0})
		buf.EventFd.Close()
		buf.AddrFd.Close()
		buf.XDataFd.Close()
		buf.TagFd.Close()
		buf.ColorFd.Close()
		defer buf.BodyFd.Close()
		out = writenWrapper{buf.BodyFd}
	} else {
		out = os.Stdout
	}

	if err != nil {
		fmt.Fprintf(out, "Guru error: %v\n", err)
		return
	}

	if arg != "describe" || fullwrite {
		out.Write(bs)
		return
	}

	scan := bufio.NewScanner(bytes.NewReader(bs))
	first := true
	skipdetails := false
	showonlydefined := false
	for scan.Scan() {
		const sep = ": "
		line := scan.Text()
		idx := strings.Index(line, sep)

		pos := line[:idx]
		rest := line[idx+len(sep):]

		if pos == "guru" {
			skipdetails = true
			first = false
		}

		if first {
			out.Write([]byte(pos))
			out.Write([]byte(":\n"))

			var funcname string
			for _, prefix := range []string{
				refToMethodFunc,
				refToIfaceMethodFunc,
				refToFunc} {
				if strings.HasPrefix(rest, prefix) {
					funcname = rest[len(prefix):]
					break
				}
			}
			if funcname != "" {
				ok := godoc(funcname, out)
				if ok {
					showonlydefined = true
					skipdetails = true
				}
			}
			if !showonlydefined {
				if funcname != "" {
					funcnameClear := clearPackagePaths(funcname)
					if len(funcnameClear) < linelen-len("reference to func ") {
						fmt.Fprintf(out, "reference to func %s\n", funcnameClear)
					} else {
						fmt.Fprintf(out, "reference to\n\tfunc %s\n", funcnameClear)
					}
				} else {
					out.Write([]byte(rest))
				}
			}
			first = false
		} else {
			if strings.Index(rest, "defined") >= 0 {
				fmt.Fprintf(out, "%s: %s\n", pos, rest)
			} else {
				if !showonlydefined {
					out.Write([]byte(rest))
				}
			}
		}
		out.Write([]byte("\n"))
	}

	if !skipdetails {
		fmt.Fprintf(out, "\nDetails:\n\tGo dd %d %s\n", idx, pos)
	}
}

func godoc(funcname string, out io.Writer) bool {
	pkg, receiver, name, ok := parseGuruFuncname(funcname)
	if !ok {
		return false
	}

	cmd := exec.Command("go", "doc", pkg, name)
	bs, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	scan := bufio.NewScanner(bytes.NewReader(bs))

	const funcPrefix = "func "

	found := false

	for scan.Scan() {
		line := scan.Text()

		if !strings.HasPrefix(line, funcPrefix) {
			continue
		}

		curreceiver, curname, ok := parseGodocFuncdef(line[len(funcPrefix):])
		if !ok {
			continue
		}
		if curreceiver == receiver && curname == name {
			out.Write([]byte(clearPackagePaths(line)))
			out.Write([]byte("\n"))
			found = true
			break
		}
	}

	if !found {
		return false
	}

	for scan.Scan() {
		line := scan.Text()
		if len(line) <= 0 || line[0] != ' ' {
			break
		}
		out.Write([]byte(line))
		out.Write([]byte("\n"))
	}

	return true
}

func parseGuruFuncname(funcname string) (pkg, receiver, name string, ok bool) {
	ok = false
	if len(funcname) < 1 {
		return
	}

	var rest string

	if funcname[0] != '(' {
		found := false
		for i := 1; i < len(funcname); i++ {
			if funcname[i] == '.' {
				found = true
				pkg = funcname[1:i]
				if i+1 >= len(funcname) {
					return
				}
				rest = funcname[i+1:]
				break
			}
		}
		if !found {
			return
		}
	} else {
		if len(funcname) < 2 {
			return
		}
		start := 1
		if funcname[1] == '*' {
			start++
		}
		lastdot := 0
	pkgloop:
		for i := 2; i < len(funcname); i++ {
			switch funcname[i] {
			case '.':
				lastdot = i
			case ')':
				if lastdot == 0 {
					lastdot = i
				}
				pkg = funcname[start:lastdot]
				if lastdot < i {
					receiver = funcname[lastdot+1 : i]
				}
				if i+1 >= len(funcname) {
					return
				}
				rest = funcname[i+1:]
				break pkgloop
			}
		}
	}

	if pkg == "" || rest == "" {
		return
	}

	if rest[0] == '.' {
		if len(rest) < 2 {
			return
		}
		rest = rest[1:]
	}

	for i := range rest {
		if rest[i] == '(' {
			name = rest[:i]
			break
		}
	}

	if name == "" {
		return
	}

	ok = true
	return
}

func parseGodocFuncdef(funcdef string) (receiver, name string, ok bool) {
	i := 0
	for i < len(funcdef) {
		if funcdef[i] != ' ' {
			break
		}
		i++
	}

	if i >= len(funcdef) {
		return
	}

	if funcdef[i] == '(' {
		for i < len(funcdef) {
			if funcdef[i] == ' ' {
				i++
				break
			}
			i++
		}

		if i >= len(funcdef) {
			return
		}

		if funcdef[i] == '*' {
			i++
			if i >= len(funcdef) {
				return
			}
		}

		start := i

		for i < len(funcdef) {
			if funcdef[i] == ')' {
				receiver = funcdef[start:i]
				i++
				break
			}
			i++
		}

		if i >= len(funcdef) {
			return
		}

		for i < len(funcdef) {
			if funcdef[i] != ' ' {
				break
			}
			i++
		}

		if i >= len(funcdef) {
			return
		}
	}

	start := i

	for i < len(funcdef) {
		if funcdef[i] == '(' {
			name = funcdef[start:i]
			ok = true
			return
		}
		i++
	}

	return
}

const linelen = 70

func clearPackagePaths(in string) string {
	type state uint8
	const (
		outofid state = iota
		inid
	)

	s := outofid
	r := make([]byte, 0, len(in))

	start := -1

	isid := func(ch rune) bool {
		return unicode.IsDigit(ch) || unicode.IsLetter(ch) || ch == '/' || ch == '.'
	}

	flushid := func(i int) {
		id := in[start:i]
		if idx := strings.LastIndex(id, "/"); idx >= 0 {
			r = append(r, []byte(id[idx+1:])...)
		} else {
			r = append(r, []byte(id)...)
		}
	}

	for i, ch := range in {
		switch s {
		case outofid:
			if isid(ch) {
				start = i
				s = inid
			} else {
				r = append(r, []byte(string(ch))...)
			}
		case inid:
			if !isid(ch) {
				flushid(i)
				r = append(r, []byte(string(ch))...)
				s = outofid
			}
		}
	}

	if s == inid {
		flushid(len(in))
	}

	rs := string(r)
	if len(rs) > linelen-8 {
		rs = splitFuncDef(rs)
	}
	return rs
}

func splitFuncDef(in string) string {
	r := make([]byte, 0, len(in))
	lastnl := 0
	for i, ch := range in {
		switch {
		case (ch == '(') && (i != 0):
			r = append(r, []byte(string(ch))...)
			r = append(r, []byte("\n\t\t")...)
			lastnl = len(r)
		case (ch == ' ') && len(r[lastnl:]) > linelen-16:
			r = append(r, []byte("\n\t\t")...)
			lastnl = len(r)
		default:
			r = append(r, []byte(string(ch))...)
		}
	}
	return string(r)
}

func main() {
	switch len(os.Args) {
	case 1:
		switch filepath.Base(os.Args[0]) {
		case "Go":
			usage()
		case "Gofmt":
			gofmt()
		case "Goren":
			gorename()
		case "God":
			guru("describe")
		case "Gor":
			guru("referrers")
		}
	default:
		if os.Args[0] == "Gofmt" {
			util.Allergic(debug, os.Chdir(os.Args[1]))
			gofmt()
			return
		}
		switch os.Args[1] {
		case "go2def":
			go2defcmd()
		case "fmt":
			if len(os.Args) >= 3 {
				util.Allergic(debug, os.Chdir(os.Args[2]))
				gofmt()
			} else {
				gofmt()
			}
		case "ren", "rename":
			gorename()
		case "definition", "describe", "freevars", "implements", "referrers", "what":
			guru(os.Args[1])
		case "callers", "callees", "callstack", "peers", "pointsto", "whicherrs":
			if len(os.Args) < 3 {
				fmt.Fprintf(os.Stderr, "Must specify scope for command: %s\n", os.Args[1])
				os.Exit(1)
			}
			guruscope(os.Args[1], os.Args[2])
		case "d":
			guru("describe")
		case "dd":
			if len(os.Args) < 4 {
				fmt.Fprintf(os.Stderr, "Wrong prepared guru command syntax\n")
				os.Exit(1)
			}
			guruprepared("describe", os.Args[2], os.Args[3])
		case "r":
			guru("referrers")
		case "help":
			usage()
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		}
	}
}
