package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/aarzilli/yacco/buf"
	"github.com/aarzilli/yacco/config"
	"github.com/aarzilli/yacco/hl"
	"github.com/aarzilli/yacco/util"
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

	for i := range Wnd.cols.cols {
		for j := range Wnd.cols.cols[i].editors {
			FsRemoveEditor(Wnd.cols.cols[i].editors[j].edid)
		}
	}

	h := Wnd.cols.cols[0].contentArea()
	Wnd.cols.cols = Wnd.cols.cols[0:0]

	cdIntl(dw.Wd)

	buffers := make([]*buf.Buffer, len(dw.Buffers))
	for i, db := range dw.Buffers {
		b, err := buf.NewBuffer(db.Dir, db.Name, true, Wnd.Prop["indentchar"], hl.New(config.LanguageRules, db.Name))
		if err != nil {
			b, _ = buf.NewBuffer(dw.Wd, "+CouldntLoad", true, Wnd.Prop["indentchar"], hl.NilHighlighter)
		}
		b.Props = db.Props
		if db.Text != "" {
			b.Replace([]rune(db.Text), &util.Sel{0, b.Size()}, true, nil, util.EO_KBD)
		}
		buffers[i] = b
	}

	for _, dc := range dw.Columns {
		col := Wnd.cols.AddAfter(NewCol(&Wnd, Wnd.cols.r), -1, 0.4)

		col.tagbuf.Replace([]rune(dc.TagText), &util.Sel{0, col.tagbuf.Size()}, true, nil, util.EO_MOUSE)

		for _, de := range dc.Editors {
			b := buffers[de.Id]
			ed := NewEditor(b)
			switch de.Font {
			case "main":
				ed.sfr.Fr.Font = config.MainFont
			case "alt":
				ed.sfr.Fr.Font = config.AltFont
			}
			col.AddAfter(ed, -1, -1, true)

			ed.tagbuf.Replace([]rune(de.TagText), &util.Sel{ed.tagbuf.EditableStart, ed.tagbuf.Size()}, true, nil, util.EO_MOUSE)
			if de.SelS != 0 {
				ed.sfr.Fr.Sel.S = de.SelS
				ed.sfr.Fr.Sel.E = de.SelS
			}
		}
		for i, de := range dc.Editors {
			col.editors[i].size = int((de.Frac / 10.0) * float64(h))
		}
	}

	for i, dc := range dw.Columns {
		Wnd.cols.cols[i].frac = dc.Frac
	}

	Wnd.GenTag()
	Wnd.tagbuf.Replace([]rune(dw.TagText), &util.Sel{Wnd.tagbuf.EditableStart, Wnd.tagbuf.Size()}, true, nil, util.EO_MOUSE)
	Wnd.BufferRefresh()
	Wnd.tagfr.Redraw(true, nil)
	Wnd.RedrawHard()

	for i, db := range dw.Buffers {
		if db.DumpCmd != "" {
			NewJob(db.DumpDir, db.DumpCmd, "", &ExecContext{buf: buffers[i]}, false, false, nil)
		}
	}

	for i := range Wnd.cols.cols {
		for j := range Wnd.cols.cols[i].editors {
			Wnd.cols.cols[i].editors[j].BufferRefreshEx(true, true)
		}
	}

	return true
}

func setDumpTitle() {
	b := filepath.Base(AutoDumpPath)
	b = b[:len(b)-len(".dump")]
	Wnd.SetTitle("Yacco " + b)
}

func historyAdd(path string) {
	if !FirstOpenFile {
		return
	}
	if AutoDumpPath != "" {
		return
	}
	FirstOpenFile = false

	histpath := os.ExpandEnv("$HOME/.config/yacco/history")
	buf, _ := ioutil.ReadFile(histpath)
	oldpaths := strings.Split(string(buf), "\n")
	paths := make([]string, 0, len(oldpaths))
	for _, p := range oldpaths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		paths = append(paths, p)
	}

	found := false
	for i := range paths {
		if paths[i] == path {
			paths = append(paths[:i], paths[i+1:]...)
			paths = append(paths, path)
			found = true
			break
		}
	}
	if !found {
		paths = append(paths, path)
	}
	if len(paths) > 12 {
		paths = paths[:len(paths)-12]
	}

	ioutil.WriteFile(histpath, []byte(strings.Join(paths, "\n")+"\n"), 0666)
}
