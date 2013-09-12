package main

import (
	"fmt"
	"path/filepath"
	"yacco/buf"
	"yacco/util"
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

func EditFind(rel2dir, path string, warp bool, create bool) (*Editor, error) {
	abspath := util.ResolvePath(rel2dir, path)

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
		Wnd.cols.AddAfter(NewCol(Wnd.wnd, Wnd.cols.r), -1, 0.4)
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

	if len(col.editors) <= 0 {
		col.AddAfter(ed, -1, 0.5)
	} else {
		emptyed := col.editors[0]
		biged := col.editors[0]
		lh := int(biged.sfr.Fr.Font.LineHeight())

		for _, ced := range col.editors {
			if ced.Height() >= biged.Height()-lh {
				biged = ced
			}

			if (ced.Height() - ced.UsedHeight()) >= (emptyed.Height() - emptyed.UsedHeight() - lh) {
				emptyed = ced
			}
		}

		el := (emptyed.Height() - emptyed.UsedHeight()) / lh
		bl := (biged.Height() / lh)
		if (el > 15) || ((el > 3) && (el > bl/2)) {
			f := float32(emptyed.UsedHeight()) / float32(emptyed.Height())
			col.AddAfter(ed, col.IndexOf(emptyed), f)
		} else {
			col.AddAfter(ed, col.IndexOf(biged), 0.5)
		}

	}

	Wnd.cols.RecalcRects()
	col.Redraw()
	Wnd.wnd.FlushImage()
	if warp {
		ed.Warp()
	}
}

func Warnfull(bufname, msg string, clear bool) {
	ed, err := EditFind(Wnd.tagbuf.Dir, bufname, false, true)
	if err != nil {
		fmt.Printf("Warn: %s (additionally error %s while displaying this warning)\n", msg, err.Error())
	} else {
		if clear {
			ed.sfr.Fr.Sels[0].S = 0
		} else {
			ed.sfr.Fr.Sels[0].S = ed.bodybuf.Size()
		}
		ed.sfr.Fr.Sels[0].E = ed.bodybuf.Size()

		ed.bodybuf.Replace([]rune(msg), &ed.sfr.Fr.Sels[0], true, nil, 0, true)
		ed.BufferRefresh(false)
	}
}

// Warn user about error
func Warn(msg string) {
	Warnfull("+Error", msg, false)
}

func Warndir(dir, msg string) {
	Warnfull(filepath.Join(dir, "+Error"), msg, false)
}
