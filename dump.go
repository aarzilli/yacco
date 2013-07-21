package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"yacco/buf"
	"yacco/config"
	"yacco/util"
)

type DumpWindow struct {
	Columns []DumpColumn
	Buffers []DumpBuffer
	Wd      string
}

type DumpColumn struct {
	Frac    float64
	Editors []DumpEditor
}

type DumpEditor struct {
	Id      int
	Frac    float64
	Font    string
	Special bool
}

type DumpBuffer struct {
	IsNil   bool
	Dir     string
	Name    string
	Props   map[string]string
	Text    string
	DumpCmd string
	DumpDir string
}

func DumpTo(dumpDest string) bool {
	os.MkdirAll(filepath.Dir(dumpDest), 0700)
	fh, err := os.OpenFile(dumpDest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		Warn("Could not dump to: " + dumpDest + " error: " + err.Error())
		return false
	}
	defer fh.Close()
	enc := json.NewEncoder(fh)
	dw := Wnd.Dump()
	err = enc.Encode(dw)
	if err != nil {
		Warn("Could not write to dump file " + dumpDest + " error: " + err.Error())
		return false
	}
	return true
}

func LoadFrom(dumpDest string) bool {
	fh, err := os.Open(dumpDest)
	if err != nil {
		Warn("Could not load dump from: " + dumpDest + " error: " + err.Error())
		return false
	}
	defer fh.Close()
	dec := json.NewDecoder(fh)
	var dw DumpWindow
	err = dec.Decode(&dw)
	if err != nil {
		Warn("Could not load dump from: " + dumpDest + " error: " + err.Error())
		return false
	}

	for i := range buffers {
		if buffers[i] != nil {
			fsNodefs.removeBuffer(i)
		}
	}
	Wnd.cols.cols = Wnd.cols.cols[0:0]

	cdIntl(dw.Wd)

	buffers = make([]*buf.Buffer, len(dw.Buffers))
	for i, db := range dw.Buffers {
		b, err := buf.NewBuffer(db.Dir, db.Name, true, Wnd.Prop["indentchar"])
		if err != nil {
			b, _ = buf.NewBuffer(dw.Wd, "+CouldntLoad", true, Wnd.Prop["indentchar"])
		}
		b.Props = db.Props
		if db.Text != "" {
			b.Replace([]rune(db.Text), &util.Sel{0, b.Size()}, []util.Sel{}, true, nil, util.EO_KBD, true)
		}
		buffers[i] = b
		fsNodefs.addBuffer(i, b)
	}

	for _, dc := range dw.Columns {
		col := Wnd.cols.AddAfter(-1)
		for _, de := range dc.Editors {
			b := buffers[de.Id]
			ed := NewEditor(b, false)
			b.RefCount++
			switch de.Font {
			case "main":
				ed.sfr.Fr.Font = config.MainFont
			case "alt":
				ed.sfr.Fr.Font = config.AltFont
			}
			col.AddAfter(ed, -1, 0.5)

			if de.Special && (b.Name == "+LookFile") {
				lookFile(ed)
			}
		}
		for i, de := range dc.Editors {
			col.editors[i].frac = de.Frac
		}
	}

	for i, dc := range dw.Columns {
		Wnd.cols.cols[i].frac = dc.Frac
	}

	Wnd.Resized()

	for _, db := range dw.Buffers {
		if db.DumpCmd != "" {
			NewJob(db.DumpDir, db.DumpCmd, "", &ExecContext{}, false, nil)
		}
	}

	for i := range buffers {
		if buffers[i] == nil {
			fsNodefs.removeBuffer(i)
		}
		if buffers[i].RefCount == 0 {
			fsNodefs.removeBuffer(i)
		}
	}

	return true
}
