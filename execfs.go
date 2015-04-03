package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"yacco/util"
)

func ExecFs(ec *ExecContext, cmd string) syscall.Errno {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return 0
	}
	switch cmd {
	case "addr=dot":
		ec.ed.otherSel[OS_ADDR] = ec.fr.Sel

	case "clean":
		ec.buf.Modified = false
		sideChan <- RefreshMsg(ec.buf, ec.ed, true)

	case "dirty":
		ec.buf.Modified = true
		sideChan <- RefreshMsg(ec.buf, ec.ed, true)

	case "cleartag":
		ec.ed.tagbuf.Replace([]rune{}, &util.Sel{ec.ed.tagbuf.EditableStart, ec.ed.tagbuf.Size()}, true, nil, 0)
		ec.ed.TagRefresh()

	case "del":
		DelCmd(*ec, "", false)

	case "delete":
		DelCmd(*ec, "", true)

	case "dot=addr":
		ec.fr.Sel = ec.ed.otherSel[OS_ADDR]
		sideChan <- RefreshMsg(ec.buf, ec.ed, true)

	case "get":
		ec.ed.bodybuf.Modified = false
		GetCmd(*ec, "")

	case "limit=addr":
		fmt.Fprintf(os.Stderr, "limit=addr not implemented\n")

	case "mark":
		ec.buf.EditMarkNext = true

	case "nomark":
		ec.buf.EditMarkNext = false

	case "put":
		PutCmd(*ec, "")

	case "show":
		sideChan <- RefreshMsg(ec.buf, ec.ed, true)

	case "tabadj":
		elasticTabs(ec.ed, true)
		sideChan <- RefreshMsg(ec.buf, ec.ed, true)

	case "compat":
		// legacy command, does nothing

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
			debugfsf("Unrecognized ctl command <%s>\n", cmd)
			return syscall.EINVAL
		}
	}
	return 0
}
