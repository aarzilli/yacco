package main

import (
	"code.google.com/p/go9p/p"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"yacco/util"
)

var debug = false

func usage() {
	fmt.Fprintf(os.Stderr, "y9p ls <path>\ny9p read <path>\ny9p write <path>\ny9p find <buffer name>\ny9p new <buffer name>\n")
}

func read(fd io.Reader) (string, error) {
	b := make([]byte, 1024)
	n, err := fd.Read(b)
	if err != nil {
		return "", err
	}
	return string(b[:n]), nil
}

func main() {
	p9clnt, err := util.YaccoConnect()
	util.Allergic(debug, err)
	defer p9clnt.Unmount()

	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Wrong number of arguments\n")
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	arg := os.Args[2]

	switch cmd {
	case "ls":
		fd, err := p9clnt.FOpen(resolvePath(arg), p.OREAD)
		util.Allergic(debug, err)
		defer fd.Close()
		entries, err := fd.Readdir(0)
		util.Allergic(debug, err)

		for _, entry := range entries {
			t := ""
			if (entry.Mode & p.DMDIR) != 0 {
				t = "/"
			} else if (entry.Mode & p.DMSYMLINK) != 0 {
				t = "@"
			}
			//TODO: check other types maybe

			fmt.Printf("%#o\t%s%s\n", entry.Mode&0777, entry.Name, t)
		}

	case "read":
		fd, err := p9clnt.FOpen(resolvePath(arg), p.OREAD)
		util.Allergic(debug, err)
		defer fd.Close()
		io.Copy(os.Stdout, fd)

	case "write":
		fd, err := p9clnt.FOpen(resolvePath(arg), p.OWRITE)
		util.Allergic(debug, err)
		defer fd.Close()

		written := int64(0)
		buf := make([]byte, 4*1024)
		for {
			nr, er := os.Stdin.Read(buf)
			if nr > 0 {
				nw, ew := fd.Write(buf[0:nr])
				if nw > 0 {
					written += int64(nw)
				}
				if ew != nil {
					err = ew
					break
				}
				if nr != nw {
					err = io.ErrShortWrite
					break
				}
			}
			if er == io.EOF {
				break
			}
			if er != nil {
				break
			}
		}

		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
		}

	case "find":
		wd, _ := os.Getwd()
		dst := filepath.Join(wd, arg)

		buf, err := util.FindWinEx(arg, p9clnt)
		util.Allergic(debug, err)
		defer buf.Close()
		_, err = buf.CtlFd.Write([]byte("name " + dst))
		util.Allergic(debug, err)
		fmt.Printf("%s\n", buf.Id)

	case "new":
		wd, _ := os.Getwd()
		dst := filepath.Join(wd, arg)

		ctlfd, err := p9clnt.FCreate("/new/ctl", 0666, p.ORDWR)
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
