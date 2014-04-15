package main

import (
	"fmt"
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
		ec.ed.BufferRefresh(false)

	case "tabadj":
		elasticTabs(ec.ed, true)
		ec.ed.BufferRefresh(false)

	case "disconnect":
		if ec.ed.eventChan != nil {
			ec.ed.eventChan <- ""
			close(ec.ed.eventChan)
			ec.ed.eventChan = nil
		}

	default:
		if strings.HasPrefix(cmd, "dumpdir") {
			ec.buf.DumpDir = strings.TrimSpace(cmd[len("dumpdir"):])
		} else if strings.HasPrefix(cmd, "dump") {
			ec.buf.DumpCmd = strings.TrimSpace(cmd[len("dump"):])
		} else if strings.HasPrefix(cmd, "name ") {
			RenameCmd(*ec, cmd[len("name"):])
		} else {
			fmt.Printf("Unrecognized ctl command <%s>\n", cmd)
		}
	}
}
