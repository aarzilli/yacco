package main

import (
	"io"
	"os"
	"io/ioutil"
	"fmt"
	"strings"
	"strconv"
	"yacco/util"
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/clnt"
)

var debug = false

func getCwd(p9clnt *clnt.Clnt) string {
	props, err := util.ReadProps(p9clnt)
	util.Allergic(debug, err)
	return props["cwd"]
}

func main() {
	p9clnt, err := util.YaccoConnect()
	util.Allergic(debug, err)
	defer p9clnt.Unmount()
	
	cwd := getCwd(p9clnt)
	
	os.Chdir(cwd)
	
	indexEntries, err := util.ReadIndex(p9clnt)
	util.Allergic(debug, err)
	
	var buf *util.BufferConn
	
	for i := range indexEntries {
		if strings.HasSuffix(indexEntries[i].Path, "/+Lookfile") {
			buf, err = util.OpenBufferConn(p9clnt, strconv.Itoa(indexEntries[i].Idx))
			defer buf.Close()
			util.Allergic(debug, err)
		}
	}
	
	if buf != nil {
		io.WriteString(buf.CtlFd, "show\n")
		return
	}
	
	ctlfd, err := p9clnt.FOpen("/new/ctl", p.ORDWR)
	util.Allergic(debug, err)
	ctlln, err := ioutil.ReadAll(ctlfd)
	ctlfd.Close()
	outbufid := strings.TrimSpace(string(ctlln[:11]))
	
	buf, err = util.OpenBufferConn(p9clnt, outbufid)
	util.Allergic(debug, err)
	
	fmt.Fprintf(buf.CtlFd, "dumpdir %s\n", cwd)
	io.WriteString(buf.CtlFd, "dump LookFile\n")
	fmt.Fprintf(buf.CtlFd, "name %s/+Lookfile\n", cwd)
	io.WriteString(buf.CtlFd, "show\n")
	
	//TODO: 
	// - listen for write to buffer events
	// - everything is bounced back
	// - load events cause the first line of the buffer to be selected
	// - typing a newline causes the first result to be loaded
	// - on every buffer change event (that isn't writing a newline) read the first line, produce a list of results and write it to the buffer
	// - call global and add results from that
}
