package main

import (
	"code.google.com/p/go9p/p"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"yacco/util"
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
	path := os.Args[1]
	abspath := util.ResolvePath(wd, path)
	
	ctlfd, err := p9clnt.FOpen("/new/ctl", p.ORDWR)
	util.Allergic(debug, err)
	ctlln := read(ctlfd)
	outbufid := strings.TrimSpace(ctlln[:11])
	
	_, err = ctlfd.Write([]byte(fmt.Sprintf("name %s", abspath)))
	util.Allergic(debug, err)
	_, err = ctlfd.Write([]byte(fmt.Sprintf("get")))
	util.Allergic(debug, err)
	ctlfd.Close()
	
	for {
		_, err := p9clnt.FStat("/" + outbufid)
		if err != nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
}
