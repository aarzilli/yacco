package main

import (
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/clnt"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"
	"yacco/util"
)

var debug = false

func usage() {
	fmt.Fprintf(os.Stderr, "y9p inst\t\t\tlist instances\n")
	fmt.Fprintf(os.Stderr, "y9p ls <path>\t\t\tlist directory\n")
	fmt.Fprintf(os.Stderr, "y9p read <path>\t\t\tread file\n")
	fmt.Fprintf(os.Stderr, "y9p write <path>\t\twrite file\n")
	fmt.Fprintf(os.Stderr, "y9p find <buffer name>\t\tfind buffer\n")
	fmt.Fprintf(os.Stderr, "y9p new <buffer name>\t\tcreate buffer\n")
	fmt.Fprintf(os.Stderr, "y9p exec <yacco command>\texecute command\n")
	fmt.Fprintf(os.Stderr, "y9p eventloop <buffer id>\treads event file, writes event function calls\n")
}

func read(fd io.Reader) (string, error) {
	b := make([]byte, 1024)
	n, err := fd.Read(b)
	if err != nil {
		return "", err
	}
	return string(b[:n]), nil
}

func argCheck(n int, connect bool) *clnt.Clnt {
	if n >= 0 {
		if len(os.Args) != n {
			fmt.Fprintf(os.Stderr, "Wrong number of arguments\n")
			usage()
			os.Exit(1)
		}
	}
	if !connect {
		return nil
	}
	p9clnt, err := util.YaccoConnect()
	util.Allergic(debug, err)
	return p9clnt
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Wrong number of arguments\n")
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "help":
		argCheck(2, false)
		usage()

	case "inst":
		argCheck(2, false)
		instList()

	case "ls":
		p9clnt := argCheck(3, true)
		defer p9clnt.Unmount()

		fd, err := p9clnt.FOpen(resolvePath(os.Args[2]), p.OREAD)
		util.Allergic(debug, err)
		defer fd.Close()
		entries, err := fd.Readdir(0)
		util.Allergic(debug, err)

		for _, entry := range entries {
			t := ""
			if (entry.Mode & p.DMDIR) != 0 {
				t = "/"
			} else if (entry.Mode&p.DMSYMLINK) != 0 || (entry.Mode&p.DMLINK) != 0 {
				t = "@"
			}

			fmt.Printf("%#o\t%s%s\n", entry.Mode&0777, entry.Name, t)
		}

	case "read":
		p9clnt := argCheck(3, true)
		defer p9clnt.Unmount()

		fd, err := p9clnt.FOpen(resolvePath(os.Args[2]), p.OREAD)
		util.Allergic(debug, err)
		defer fd.Close()
		io.Copy(os.Stdout, fd)

	case "eventloop":
		p9clnt := argCheck(3, true)
		defer p9clnt.Unmount()

		fd, err := p9clnt.FOpen(fmt.Sprintf("/%s/event", os.Args[2]), p.OREAD)
		util.Allergic(debug, err)
		defer fd.Close()
		rbuf := make([]byte, 1024)
		var er util.EventReader
		for {
			n, err := fd.Read(rbuf)
			if err != nil {
				break
			}
			if n < 2 {
				fmt.Fprintf(os.Stderr, "Not enough read form event file\n")
				os.Exit(1)
			}
			er.Reset()
			er.Insert(string(rbuf[:n]))

			for !er.Done() {
				n, err := fd.Read(rbuf)
				util.Allergic(debug, err)
				er.Insert(string(rbuf[:n]))
			}

			if ok, perr := er.Valid(); !ok {
				fmt.Fprintf(os.Stderr, "Error parsing event message(s): %s\n", perr)
				continue
			}

			p, s, e := er.Points()

			ps, pe := s, e
			if p >= 0 {
				ps, pe = p, p
			}

			txt, err := er.Text(nil, nil, nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error retrieving event text: %s\n", err)
			}
			xpath, _, _, xtxt := er.ExtraArg()

			fmt.Printf("event\t%c\t%c\t%d\t%d\t%d\t%d\t%d\t%d\t%s\t%s\t%s\n",
				er.Origin(),
				er.Type(),
				ps, pe,
				s, e,
				er.Flags(),
				utf8.RuneCountInString(txt),
				util.SingleQuote(txt),
				util.SingleQuote(xtxt),
				util.SingleQuote(xpath))
		}

	case "write":
		p9clnt := argCheck(3, true)
		defer p9clnt.Unmount()

		fd, err := p9clnt.FOpen(resolvePath(os.Args[2]), p.OWRITE)
		util.Allergic(debug, err)
		defer fd.Close()
		_, err = util.P9Copy(fd, os.Stdin)
		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
		}

	case "exec":
		p9clnt := argCheck(-1, true)
		defer p9clnt.Unmount()

		fd, err := p9clnt.FOpen(resolvePath("buf/event"), p.OWRITE)
		util.Allergic(debug, err)
		defer fd.Close()
		arg := strings.Join(os.Args[2:], " ")
		_, err = fd.Writen([]byte(fmt.Sprintf("EX0 0 0 %d %s\n", len(arg), arg)), 0)
		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
		}

	case "find":
		p9clnt := argCheck(3, true)
		defer p9clnt.Unmount()

		wd, _ := os.Getwd()
		dst := filepath.Join(wd, os.Args[2])

		buf, _, err := util.FindWinEx(os.Args[2], p9clnt)
		util.Allergic(debug, err)
		defer buf.Close()
		_, err = buf.CtlFd.Write([]byte("name " + dst))
		util.Allergic(debug, err)
		fmt.Printf("%s\n", buf.Id)

	case "find-new":
		p9clnt := argCheck(3, true)
		defer p9clnt.Unmount()

		wd, _ := os.Getwd()
		dst := filepath.Join(wd, os.Args[2])

		buf, isnew, err := util.FindWinEx(os.Args[2], p9clnt)
		util.Allergic(debug, err)
		defer buf.Close()
		_, err = buf.CtlFd.Write([]byte("name " + dst))
		util.Allergic(debug, err)
		fmt.Printf("%s %v\n", buf.Id, isnew)

	case "new":
		p9clnt := argCheck(3, true)
		defer p9clnt.Unmount()

		wd, _ := os.Getwd()
		dst := filepath.Join(wd, os.Args[2])

		ctlfd, err := p9clnt.FOpen("/new/ctl", p.ORDWR)
		util.Allergic(debug, err)
		defer ctlfd.Close()
		ctlln, err := read(ctlfd)
		util.Allergic(debug, err)
		outbufid := strings.TrimSpace(ctlln[:11])
		_, err = ctlfd.Write([]byte("name " + dst))
		util.Allergic(debug, err)
		fmt.Printf("%s\n", outbufid)

	default:
		fmt.Fprintf(os.Stderr, "Wrong command %s", cmd)
		usage()
		os.Exit(1)
	}
}

func resolvePath(in string) string {
	if in == "prop" {
		if os.Getenv("bi") == "" {
			return "/prop"
		} else {
			return "/" + os.Getenv("bi") + "/prop"
		}
	}

	if (len(in) <= 4) || (in[:4] != "buf/") {
		return in
	}

	return "/" + os.Getenv("bi") + in[3:]
}

const YACCO_PREFIX = "yacco."

func instList() {
	ns := os.Getenv("NAMESPACE")
	if ns == "" {
		ns = fmt.Sprintf("/tmp/ns.%s.%s", os.Getenv("USER"), os.Getenv("DISPLAY"))
		fmt.Printf("Using default namespace: %s\n", ns)
	} else {
		fmt.Printf("Using NAMESPACE: %s\n", ns)
	}

	dh, err := os.Open(ns)
	util.Allergic(debug, err)
	defer dh.Close()

	files, err := dh.Readdir(-1)
	util.Allergic(debug, err)

	for i := range files {
		n := files[i].Name()
		p := filepath.Join(ns, n)

		if !strings.HasPrefix(n, YACCO_PREFIX) {
			continue
		}

		pid, err := strconv.Atoi(n[len(YACCO_PREFIX):])
		if err != nil {
			continue
		}

		if processExists(pid) {
			showInst(p)
		} else {
			removeDeadSocket(p)
		}
	}
}

func processExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = proc.Signal(syscall.Signal(0))
	if err != nil {
		return false
	}

	return true
}

func removeDeadSocket(p string) {
	fmt.Printf("Removing dead socket %s\n", p)
	err := os.Remove(p)
	if err != nil {
		fmt.Printf("Could not remove %s: %v\n", p, err)
	}
}

func showInst(p string) {
	util.Allergic(debug, os.Setenv("yp9", p))
	p9clnt, err := util.YaccoConnect()
	if err != nil {
		removeDeadSocket(p)
	}
	defer p9clnt.Unmount()

	fmt.Printf("export yp9=%s\n", p)

	props, err := util.ReadProps(p9clnt)
	util.Allergic(debug, err)

	if adp, ok := props["AutoDumpPath"]; ok && adp != "" {
		fmt.Printf("\t%s\n", adp)
	}

	index, err := util.ReadIndex(p9clnt)
	util.Allergic(debug, err)

	for i := range index {
		fmt.Printf("\t%d %s\n", index[i].Idx, index[i].Path)
	}

	fmt.Printf("\n")
}
