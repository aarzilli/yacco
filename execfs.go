package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"yacco/util"
)

func ExecFs(ec *ExecContext, cmd string) {
	cmd = strings.TrimSpace(cmd)
	switch cmd {
	case "addr=dot":
		ec.ed.otherSel[OS_ADDR] = ec.fr.Sels[0]

	case "clean":
		ec.buf.Modified = false
		ec.br.BufferRefresh(false)

	case "dirty":
		ec.buf.Modified = true
		ec.br.BufferRefresh(false)

	case "cleartag":
		ec.ed.tagbuf.Replace([]rune{}, &util.Sel{ec.ed.tagbuf.EditableStart, ec.ed.tagbuf.Size()}, true, nil, 0, false)
		ec.br.BufferRefresh(true)

	case "del":
		DelCmd(*ec, "", false)

	case "delete":
		DelCmd(*ec, "", true)

	case "dot=addr":
		ec.fr.Sels[0] = ec.ed.otherSel[OS_ADDR]
		ec.br.BufferRefresh(false)

	case "get":
		GetCmd(*ec, "")

	case "limit=addr":
		//XXX limit=addr not implemented

	case "mark":
		ec.buf.EditMarkNext = true

	case "nomark":
		ec.buf.EditMarkNext = false

	case "put":
		PutCmd(*ec, "")

	case "show":
		ec.ed.Recenter()

	default:
		if strings.HasPrefix(cmd, "dumpdir") {
			ec.buf.DumpDir = strings.TrimSpace(cmd[len("dumpdir"):])
		} else if strings.HasPrefix(cmd, "dump") {
			ec.buf.DumpCmd = strings.TrimSpace(cmd[len("dump"):])
		} else if strings.HasPrefix(cmd, "name ") {
			newName := strings.TrimSpace(cmd[len("name"):])
			abspath := util.ResolvePath(ec.buf.Dir, newName)
			ec.buf.Name = filepath.Base(abspath)
			ec.buf.Dir = filepath.Dir(abspath)
			ec.br.BufferRefresh(false)
		} else {
			fmt.Printf("Unrecognized ctl command <%s>\n", cmd)
		}
	}
}
