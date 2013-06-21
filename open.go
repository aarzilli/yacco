package main

import (
	"image"
	"path/filepath"
	"yacco/util"
	"yacco/buf"
)

func EditOpen(path string) *Editor {
	abspath, err := filepath.Abs(path)
	util.Must(err, "Error parsing argument")
	dir := filepath.Dir(abspath)
	name := filepath.Base(abspath)
	return NewEditor(buf.NewBuffer(dir, name))
}

func EditFind(path string) *Editor {
	abspath, err := filepath.Abs(path)
	util.Must(err, "Error parsing argument")
	dir := filepath.Dir(abspath)
	name := filepath.Base(abspath)

	for _, col := range wnd.cols.cols {
		for _, ed := range col.editors {
			if (ed.bodybuf.Name == name) && (ed.bodybuf.Dir == dir) {
				if ed.frac < 0.5 {
					wnd.GrowEditor(col, ed)
					wnd.wnd.WarpMouse(ed.sfr.Fr.R.Min.Add(image.Point{ 2, 2}))
					return ed
				}
			}
		}
	}

	return HeuristicOpen(path, true)
}

func HeuristicOpen(path string, warp bool) *Editor {
	ed := EditOpen(path)

	if len(wnd.cols.cols) == 0 {
		wnd.cols.AddAfter(-1)
	}

	var col *Col = nil


	if path[0] == '+' {
		col = wnd.cols.cols[len(wnd.cols.cols)-1]
	} else {
		if activeEditor != nil {
			activeCol := activeEditor.Column()
			if activeCol != nil {
				col = activeCol
			}
		}

		if col == nil {
			col = wnd.cols.cols[0]
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
	wnd.cols.RecalcRects(wnd.cols.b)
	col.Redraw()
	wnd.wnd.FlushImage()
	wnd.wnd.WarpMouse(ed.sfr.Fr.R.Min.Add(image.Point{ 2, 2}))

	return ed
}

// Warn user about error
func Warn(msg string) {
	ed := EditFind("+Error")

	ed.sfr.Fr.Sels[0].S = 0
	ed.sfr.Fr.Sels[0].E = ed.bodybuf.Size()

	ed.bodybuf.Replace([]rune(msg), &ed.sfr.Fr.Sels[0], ed.sfr.Fr.Sels)
	ed.BufferRefresh(false)
}
