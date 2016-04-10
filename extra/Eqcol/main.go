package main

import (
	"io"
	"io/ioutil"
	"math"
	"strconv"
	"strings"
	"yacco/util"

	"github.com/lionkov/go9p/p"
)

const debug = false

func main() {
	p9clnt, err := util.YaccoConnect()
	util.Allergic(debug, err)
	defer p9clnt.Unmount()

	cfd, err := p9clnt.FOpen("/columns", p.ORDWR)
	util.Allergic(debug, err)
	defer cfd.Close()

	bs, err := ioutil.ReadAll(cfd)
	util.Allergic(debug, err)
	lines := strings.Split(string(bs), "\n")
	fields := strings.Split(lines[0], " ")

	if len(fields) != 3 {
		return
	}

	szs := make([]float64, len(fields)-1)
	for i := range szs {
		szs[i], _ = strconv.ParseFloat(fields[i+1], 64)
	}

	if math.Abs(szs[0]-szs[1]) <= 0.01 {
		io.WriteString(cfd, "sz 0.6 0.4\n")
	} else {
		io.WriteString(cfd, "sz 0.5 0.5\n")
	}
}
