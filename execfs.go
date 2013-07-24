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
		ec.fr.Sels[4] = ec.fr.Sels[0]

	case "clean":
		ec.buf.Modified = false
		ec.br.BufferRefresh(false)

	case "dirty":
		ec.buf.Modified = true
		ec.br.BufferRefresh(false)

	case "cleartag":
		ec.ed.tagbuf.Replace([]rune{}, &util.Sel{ec.ed.tagbuf.EditableStart, ec.ed.tagbuf.Size()}, ec.ed.tagfr.Sels, true, nil, 0, false)
		ec.br.BufferRefresh(true)

	case "del":
		DelCmd(*ec, "", false)

	case "delete":
		DelCmd(*ec, "", true)

	case "dot=addr":
		ec.fr.Sels[0] = ec.fr.Sels[4]
		ec.br.BufferRefresh(false)

	case "get":
		GetCmd(*ec, "")

	case "limit=addr":
		//TODO limit=addr (fs exec)

	case "mark":
		//TODO mark (fs exec)

	case "nomark":
		//TODO nomark (fs exec)

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
			abspath := ResolvePath(ec.buf.Dir, newName)
			ec.buf.Name = filepath.Base(abspath)
			ec.buf.Dir = filepath.Dir(abspath)
			ec.br.BufferRefresh(false)
		} else {
			fmt.Printf("Unrecognized ctl command <%s>\n", cmd)
		}
	}
}
