package main

import (
	"os"
	"io"
	"io/ioutil"
	"fmt"
	"strings"
	"path/filepath"
	"github.com/wendal/readline-go"
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/clnt"
)

func makeP9Addr(name string) string {
	ns := os.Getenv("NAMESPACE")
	if ns == "" {
		ns = fmt.Sprintf("/tmp/ns.%s.%s", os.Getenv("USER"), os.Getenv("DISPLAY"))
		os.MkdirAll(ns, 0700)
	}
	return filepath.Join(ns, name)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	user := p.OsUsers.Uid2User(os.Geteuid())
	c, err := clnt.Mount("unix", makeP9Addr("acme"), "", user)
	must(err)
	defer c.Unmount()
	
	ctlf, err := c.FOpen(fmt.Sprintf("/%s/ctl", os.Args[1]), p.OWRITE)
	must(err)
	addrf, err := c.FOpen(fmt.Sprintf("/%s/addr", os.Args[1]), p.ORDWR)
	must(err)
	dataf, err := c.FOpen(fmt.Sprintf("/%s/data", os.Args[1]), p.ORDWR)
	must(err)
	xdataf, err := c.FOpen(fmt.Sprintf("/%s/xdata", os.Args[1]), p.ORDWR)
	must(err)
	
	prompt := "> "
	
	showAddr(addrf)
	
	for {
		line := readline.ReadLine(&prompt)
		if line == nil {
			return
		}
		v := strings.SplitN(*line, " ", 2)
		
		cmd := v[0]
		arg := ""
		if len(v) > 1 {
			arg = v[1]
		}
		
		switch cmd {
		case "ctl":
			if arg[len(arg)-1] != '\n' {
				arg += "\n"
			}
			writeFile(ctlf, arg)
			showAddr(addrf)
		case "data":
			writeFile(dataf, arg)
			showAddr(addrf)
		case "xdata":
			writeFile(xdataf, arg)
			showAddr(addrf)
		case "rdata":
			readFile(dataf)
			showAddr(addrf)
		case "rxdata":
			readFile(xdataf)
			showAddr(addrf)
		case "addr":
			arg = strings.TrimSpace(arg)
			if arg != "" {
				writeFile(addrf, arg)
			}
			showAddr(addrf)
		case "exit":
			os.Exit(0)
		}
	}
}

func writeFile(f io.WriteSeeker, s string) {
	n, err := io.WriteString(f, s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "write error (%d): %v\n", n, err)
	}
	f.Seek(0, 0)
}

func showAddr(f io.ReadSeeker) {
	bs, err := ioutil.ReadAll(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		return
	}
	f.Seek(0, 0)
	fmt.Printf("addr = %s\n", string(bs))
}

func readFile(f io.ReadSeeker) {
	bs, err := ioutil.ReadAll(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		return
	}
	f.Seek(0, 0)
	os.Stdout.Write(bs)
	if len(bs) == 0 || bs[len(bs)-1] != '\n' {
		os.Stdout.Write([]byte{ '\n' })
	}
}
