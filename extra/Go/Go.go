package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lionkov/go9p/p"
	"github.com/lionkov/go9p/p/clnt"

	"yacco/util"
)

const debug = false

func usage() {
	fmt.Fprintf(os.Stderr, `Implements Go integration in yacco:
	
	Go gurucmd	calls guru on the selection of active editor gurucmd is one of:
					callees callers callstack definition describe freevars implements
					peers pointsto  referrers what whicherrs
	Go d			equivalent of "Go describe"
	Go r			equivalent of "Go referrers"
	Go help			list of commands
`)
	os.Exit(1)
}

func gofmt() {
	wd, err := os.Getwd()
	util.Allergic(debug, err)
	out, err := exec.Command("go", "fmt").CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n%v\n", string(out), err)
	}
	paths := map[string]bool{}
	for _, path := range strings.Split(string(out), "\n") {
		paths[filepath.Join(wd, path)] = true
	}
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

func readaddr(p9clnt *clnt.Clnt, idx int) (pos, filename, body string) {
	ctlfd, err := p9clnt.FOpen(fmt.Sprintf("/%d/ctl", idx), p.ORDWR)
	util.Allergic(debug, err)
	ctlbs, err := ioutil.ReadAll(ctlfd)
	filename = strings.TrimSpace(string(ctlbs[12*8:]))
	ctlfd.Write([]byte("addr=dot"))
	ctlfd.Close()

	addrfd, err := p9clnt.FOpen(fmt.Sprintf("/%d/addr", idx), p.OREAD)
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

	return fmt.Sprintf("%s:#%d,#%d", filename, s, e), filename, string(bodyb)
}

func guru(arg string) {
	p9clnt, err := util.YaccoConnect()
	util.Allergic(debug, err)
	idx := readlast(p9clnt)
	pos, filename, body := readaddr(p9clnt, idx)
	cmd := exec.Command("guru", "-modified", arg, pos)
	cmd.Stdin = modified(filename, body)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func guruscope(arg string, scope string) {
	p9clnt, err := util.YaccoConnect()
	util.Allergic(debug, err)
	idx := readlast(p9clnt)
	pos, filename, body := readaddr(p9clnt, idx)
	cmd := exec.Command("guru", "-modified", "-scope="+scope, arg, pos)
	cmd.Stdin = modified(filename, body)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func modified(filename, body string) io.Reader {
	return strings.NewReader(fmt.Sprintf("%s\n%d\n%s", filename, len(body), body))
}

func main() {
	switch len(os.Args) {
	case 1:
		switch filepath.Base(os.Args[0]) {
		case "Go":
			usage()
		case "Gofmt":
			gofmt()
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
		case "fmt":
			if len(os.Args) >= 3 {
				util.Allergic(debug, os.Chdir(os.Args[2]))
				gofmt()
			} else {
				gofmt()
			}
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
		case "r":
			guru("referrers")
		case "help":
			usage()
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		}
	}
}
