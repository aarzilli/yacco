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
	TagText string
}

type DumpColumn struct {
	Frac    float64
	Editors []DumpEditor
	TagText string
}

type DumpEditor struct {
	Id      int
	Frac    float64
	Font    string
	Special bool
	TagText string
	SelS    int
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

	activeEditor = nil
	activeCol = nil
	activeSel.Reset()

	for i := range buffers {
		if buffers[i] != nil {
			FsRemoveBuffer(i)
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
			b.Replace([]rune(db.Text), &util.Sel{0, b.Size()}, true, nil, util.EO_KBD)
		}
		buffers[i] = b
		FsAddBuffer(i, b)
	}

	for _, dc := range dw.Columns {
		col := Wnd.cols.AddAfter(NewCol(Wnd.wnd, Wnd.cols.r), -1, 0.4)

		col.tagbuf.Replace([]rune(dc.TagText), &util.Sel{0, col.tagbuf.Size()}, true, nil, util.EO_MOUSE)

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

			ed.tagbuf.Replace([]rune(de.TagText), &util.Sel{ed.tagbuf.EditableStart, ed.tagbuf.Size()}, true, nil, util.EO_MOUSE)
			if de.SelS != 0 {
				ed.sfr.Fr.Sels[0].S = de.SelS
				ed.sfr.Fr.Sels[0].E = de.SelS
			}

		}
		for i, de := range dc.Editors {
			col.editors[i].frac = de.Frac
		}
	}

	for i, dc := range dw.Columns {
		Wnd.cols.cols[i].frac = dc.Frac
	}

	Wnd.GenTag()
	Wnd.tagbuf.Replace([]rune(dw.TagText), &util.Sel{Wnd.tagbuf.EditableStart, Wnd.tagbuf.Size()}, true, nil, util.EO_MOUSE)
	Wnd.BufferRefresh(true)
	Wnd.tagfr.Redraw(true)
	Wnd.Resized()

	for i, db := range dw.Buffers {
		if db.DumpCmd != "" {
			NewJob(db.DumpDir, db.DumpCmd, "", &ExecContext{buf: buffers[i]}, false, nil)
		}
	}

	for i := range buffers {
		if buffers[i] == nil {
			FsRemoveBuffer(i)
		}
		if buffers[i].RefCount == 0 {
			FsRemoveBuffer(i)
			buffers[i] = nil
		}
	}

	return true
}

func setDumpTitle() {
	b := filepath.Base(AutoDumpPath)
	b = b[:len(b)-len(".dump")]
	Wnd.wnd.SetTitle("Yacco " + b)
}
