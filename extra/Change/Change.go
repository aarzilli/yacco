package main

import (
	"fmt"
	"github.com/lionkov/go9p/p"
	"github.com/lionkov/go9p/p/clnt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"yacco/util"
)

var debug = true

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: Change <directory>\n")
		return
	}

	newdir := os.Args[1]
	if newdir[0] == '~' {
		newdir = os.ExpandEnv("$HOME" + newdir[1:])
	}
	if newdir[len(newdir)-1] != '/' {
		newdir += "/"
	}

	p9clnt, err := util.YaccoConnect()
	util.Allergic(debug, err)

	closeOpenEditors(p9clnt)
	setColumns(p9clnt)
	changeCurDirectory(p9clnt, newdir)
	openGuide(p9clnt, newdir)
}

func closeOpenEditors(p9clnt *clnt.Clnt) {
	indexEntries, err := util.ReadIndex(p9clnt)
	util.Allergic(debug, err)

	for i := range indexEntries {
		ctlfd, err := p9clnt.FOpen(fmt.Sprintf("/%d/ctl", indexEntries[i].Idx), p.OWRITE)
		util.Allergic(debug, err)
		func() {
			defer ctlfd.Close()
			ctlfd.Write([]byte("del\n"))
		}()
	}
}

func setColumns(p9clnt *clnt.Clnt) {
	cfd, err := p9clnt.FOpen("/columns", p.ORDWR)
	if err != nil {
		return
	}
	defer cfd.Close()

	bs, err := ioutil.ReadAll(cfd)
	util.Allergic(debug, err)
	v := strings.Split(string(bs), "\n")
	if len(v) <= 2 {
		io.WriteString(cfd, "new\n")
	}
}

func openFile(p9clnt *clnt.Clnt, path string) {
	ctlfd, err := p9clnt.FOpen("/new/ctl", p.OWRITE)
	util.Allergic(debug, err)
	defer ctlfd.Close()
	_, err = fmt.Fprintf(ctlfd, "name %s", path)
	util.Allergic(debug, err)
	_, err = fmt.Fprintf(ctlfd, "get")
	util.Allergic(debug, err)
}

func changeCurDirectory(p9clnt *clnt.Clnt, newdir string) {
	propfd, err := p9clnt.FOpen("/prop", p.OWRITE)
	util.Allergic(debug, err)
	defer propfd.Close()
	_, err = fmt.Fprintf(propfd, "cwd=%s", newdir)
	util.Allergic(debug, err)
	openFile(p9clnt, "./")
}

func openGuide(p9clnt *clnt.Clnt, newdir string) {
	guidefile := filepath.Join(newdir, "guide")
	_, err := os.Stat(guidefile)
	if err != nil {
		return
	}
	openFile(p9clnt, "guide")
}
