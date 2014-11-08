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
		sideChan <- RefreshMsg(ec.buf, ec.br, true)

	case "dirty":
		ec.buf.Modified = true
		sideChan <- RefreshMsg(ec.buf, ec.br, true)

	case "cleartag":
		ec.ed.tagbuf.Replace([]rune{}, &util.Sel{ec.ed.tagbuf.EditableStart, ec.ed.tagbuf.Size()}, true, nil, 0)
		ec.br.BufferRefresh(true)

	case "del":
		DelCmd(*ec, "", false)

	case "delete":
		DelCmd(*ec, "", true)

	case "dot=addr":
		ec.fr.Sels[0] = ec.ed.otherSel[OS_ADDR]
		sideChan <- RefreshMsg(ec.buf, ec.br, true)

	case "get":
		ec.ed.bodybuf.Modified = false
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
		sideChan <- RefreshMsg(ec.buf, ec.br, true)

	case "tabadj":
		elasticTabs(ec.ed, true)
		sideChan <- RefreshMsg(ec.buf, ec.br, true)

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
