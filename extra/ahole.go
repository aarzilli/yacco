package main

import (
	"yacco/util"
)

var debug = true

func main() {
	p9clnt, err := util.YaccoConnect()
	util.Allergic(debug, err)
	defer p9clnt.Unmount()

	buf, err := util.FindWin("Block", p9clnt)
	util.Allergic(debug, err)

	_, err = buf.CtlFd.Write([]byte("name +Block"))
	util.Allergic(debug, err)

	// not closing it
}
