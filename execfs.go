package main

import (
	"fmt"
	"strings"
	"yacco/util"
	"path/filepath"
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
		ec.ed.tagbuf.Replace([]rune{}, &util.Sel{ ec.ed.tagbuf.EditableStart, ec.ed.tagbuf.Size() }, ec.ed.tagfr.Sels, true, nil, 0)
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
		//TODO

	case "mark":
		//TODO

	case "nomark":
		//TODO

	case "put":
		PutCmd(*ec, "")

	case "show":
		ec.ed.Recenter()

	default:
		if strings.HasPrefix(cmd, "dump ") {
			//TODO
		} else if strings.HasPrefix(cmd, "dumpdir ") {
			//TODO
		} else if strings.HasPrefix(cmd, "name ") {
			newName := strings.TrimSpace(cmd[len("name "):])
			abspath, err := filepath.Abs(newName)
			if err != nil {
				Warn("Name error " + err.Error())
				return
			}
			ec.buf.Name = filepath.Base(abspath)
			ec.buf.Dir = filepath.Dir(abspath)
			ec.br.BufferRefresh(false)
		} else {
			fmt.Printf("Unrecognized ctl command <%s>\n", cmd)
		}
	}
}
