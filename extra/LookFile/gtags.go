package main

import (
	"bufio"
	"bytes"
	"os/exec"
	"regexp"
	"strconv"
)

var spaceseq = regexp.MustCompile(` +`)

func gtagsLoadMaybe() bool {
	if len(tags) > 0 {
		return true
	}

	bs, err := exec.Command("global", "-x", ".").CombinedOutput()
	if err != nil {
		return false
	}

	tagMu.Lock()
	defer tagMu.Unlock()

	scan := bufio.NewScanner(bytes.NewReader(bs))

	for scan.Scan() {
		fields := spaceseq.Split(scan.Text(), 4)
		if len(fields) < 3 {
			continue
		}

		lineno, _ := strconv.Atoi(fields[1])

		tags = append(tags, tagItem{fields[0], fields[2], "", lineno})
	}

	return true
}
