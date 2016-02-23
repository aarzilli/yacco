package main

import (
	"fmt"
	"github.com/lionkov/go9p/p"
	"github.com/lionkov/go9p/p/clnt"
	"io"
	"os"
	"strings"
	"time"
	"yacco/util"
	"strconv"
)

var debug = false

func read(fd io.Reader) string {
	b := make([]byte, 1024)
	n, err := fd.Read(b)
	util.Allergic(debug, err)
	return string(b[:n])
}

func main() {
	if len(os.Args) < 2 {
		return
	}

	if os.Getenv("yp9") == "" {
		return
	}

	p9clnt, err := util.YaccoConnect()
	util.Allergic(debug, err)

	wd, _ := os.Getwd()
	toline := ""
	path := os.Args[1]
	if len(path) > 0 && path[0] == '+' {
		if len(os.Args) < 3 {
			return
		}
		toline = os.Args[1]
		path = os.Args[2]
	}
	abspath := util.ResolvePath(wd, path)
	
	outbufid := ""
	
	indexEntries, err := util.ReadIndex(p9clnt)
	for i := range indexEntries {
		if indexEntries[i].Path == abspath {
			outbufid = strconv.Itoa(indexEntries[i].Idx)
			break
		}
	}
	
	var ctlfd *clnt.File
	
	if outbufid == "" {
		ctlfd, err = p9clnt.FOpen("/new/ctl", p.ORDWR)
		util.Allergic(debug, err)
		ctlln := read(ctlfd)
		outbufid = strings.TrimSpace(ctlln[:11])

		_, err = fmt.Fprintf(ctlfd, "name %s", abspath)
		util.Allergic(debug, err)
		_, err = fmt.Fprintf(ctlfd, "get")
		util.Allergic(debug, err)
	} else {
		ctlfd, err = p9clnt.FOpen("/"+outbufid+"/ctl", p.ORDWR)
		util.Allergic(debug, err)
	}

	if toline != "" {
		addrfd, err := p9clnt.FOpen("/"+outbufid+"/addr", p.OWRITE)
		util.Allergic(debug, err)
		_, err = io.WriteString(addrfd, "0")
		util.Allergic(debug, err)
		_, err = io.WriteString(addrfd, toline)
		util.Allergic(debug, err)
		_, err = fmt.Fprintf(ctlfd, "dot=addr")
		util.Allergic(debug, err)
	}

	ctlfd.Close()

	for {
		_, err := p9clnt.FStat("/" + outbufid)
		if err != nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
}
