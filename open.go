package main

import (
	"os"
	"fmt"
	"path/filepath"
	"yacco/buf"
)

func editOpen(path string, create bool) (*Editor, error) {
	//println("editOpen call:", path)
	dir := filepath.Dir(path)
	name := filepath.Base(path)

	b, err := buf.NewBuffer(dir, name, create, Wnd.Prop["indentchar"])
	if err != nil {
		return nil, err
	}
	return NewEditor(b, true), nil
}

func resolvePath(rel2dir, path string) string {
	var abspath = path
	if len(path) > 0 {
		switch path[0] {
		case '/':
			var err error
			abspath, err = filepath.Abs(path)
			if err != nil {
				return path
			}
		case '~':
			var err error
			home := os.Getenv("HOME")
			abspath = filepath.Join(home, path[1:])
			abspath, err = filepath.Abs(abspath)
			if err != nil {
				return path
			}
		default:
			var err error
			abspath = filepath.Join(rel2dir, path)
			abspath, err = filepath.Abs(abspath)
			if err != nil {
				return path
			}
		}
	}
	
	return abspath
}

func EditFind(rel2dir, path string, warp bool, create bool) (*Editor, error) {
	abspath := resolvePath(rel2dir, path)
	
	//println("Relative", abspath, rel2dir)
	dir := filepath.Dir(abspath)
	name := filepath.Base(abspath)

	for _, col := range Wnd.cols.cols {
		for _, ed := range col.editors {
			if (ed.bodybuf.Name == name) && (ed.bodybuf.Dir == dir) {
				if ed.frac < 0.5 {
					Wnd.GrowEditor(col, ed, nil)
				}
				if warp {
					ed.Warp()
				}
				return ed, nil
			}
		}
	}

	return HeuristicOpen(abspath, true, create)
}

func HeuristicOpen(path string, warp bool, create bool) (*Editor, error) {
	ed, err := editOpen(path, create)
	if err != nil {
		return nil, err
	}
	
	HeuristicPlaceEditor(ed, warp)

	return ed, nil
}

func HeuristicPlaceEditor(ed *Editor, warp bool) {
	if len(Wnd.cols.cols) == 0 {
		Wnd.cols.AddAfter(-1)
	}

	var col *Col = nil

	if ed.bodybuf.Name[0] == '+' {
		col = Wnd.cols.cols[len(Wnd.cols.cols)-1]
	} else {
		if activeEditor != nil {
			activeCol := activeEditor.Column()
			if activeCol != nil {
				col = activeCol
			}
		}

		if col == nil {
			col = Wnd.cols.cols[0]
		}
	}

	var ted *Editor = nil
	for _, ced := range col.editors {
		if ted == nil {
			ted = ced
		} else {
			if ced.Height() >= ted.Height() - int(ted.sfr.Fr.Font.LineHeight()) {
				ted = ced
			}
		}
	}

	col.AddAfter(ed, col.IndexOf(ted), 0.5)
	Wnd.cols.RecalcRects()
	col.Redraw()
	Wnd.wnd.FlushImage()
	if warp {
		ed.Warp()
	}
}


func Warnfull(bufname, msg string) {
	ed, err := EditFind(Wnd.tagbuf.Dir, bufname, false, true)
	if err != nil {
		fmt.Printf("Warn: %s (additionally error %s while displaying this warning)\n", msg, err.Error())
	} else {
		ed.sfr.Fr.Sels[0].S = ed.bodybuf.Size()
		ed.sfr.Fr.Sels[0].E = ed.bodybuf.Size()

		ed.bodybuf.Replace([]rune(msg), &ed.sfr.Fr.Sels[0], ed.sfr.Fr.Sels, true, nil, 0, true)
		ed.BufferRefresh(false)
	}
}

// Warn user about error
func Warn(msg string) {
	Warnfull("+Error", msg)
}

func Warndir(dir, msg string) {
	Warnfull(filepath.Join(dir, "+Error"), msg)
}
