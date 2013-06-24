package main

import (
	"fmt"
	"os"
	"image"
	"path/filepath"
	"yacco/util"
	"yacco/buf"
)

func EditOpen(path string) (*Editor, error) {
	abspath, err := filepath.Abs(path)
	util.Must(err, "Error parsing argument")
	dir := filepath.Dir(abspath)
	name := filepath.Base(abspath)

	if fi, err := os.Stat(abspath); err == nil {
		if fi.Size() > 10 * 1024 * 1024 {
			return nil, fmt.Errorf("Refusing to open file larger than 10MB")
		}
	}

	b, err := buf.NewBuffer(dir, name)
	if err != nil {
		return nil, fmt.Errorf("Could not open file: %s\n", abspath)
	}
	return NewEditor(b), nil
}

func EditFind(path string) (*Editor, error) {
	abspath, err := filepath.Abs(path)
	util.Must(err, "Error parsing argument")
	dir := filepath.Dir(abspath)
	name := filepath.Base(abspath)

	for _, col := range wnd.cols.cols {
		for _, ed := range col.editors {
			if (ed.bodybuf.Name == name) && (ed.bodybuf.Dir == dir) {
				if ed.frac < 0.5 {
					wnd.GrowEditor(col, ed)
				}
				wnd.wnd.WarpMouse(ed.sfr.Fr.R.Min.Add(image.Point{ 2, 2}))
				return ed, nil
			}
		}
	}

	return HeuristicOpen(path, true)
}

func HeuristicOpen(path string, warp bool) (*Editor, error) {
	ed, err := EditOpen(path)
	if err != nil {
		return nil, err
	}

	if len(wnd.cols.cols) == 0 {
		wnd.cols.AddAfter(-1)
	}

	var col *Col = nil

	if filepath.Base(path)[0] == '+' {
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
	wnd.cols.RecalcRects()
	col.Redraw()
	wnd.wnd.FlushImage()
	wnd.wnd.WarpMouse(ed.sfr.Fr.R.Min.Add(image.Point{ 2, 2}))

	return ed, nil
}

func Warnfull(bufname, msg string) {
	ed, err := EditFind(bufname)
	if err != nil {
		fmt.Printf("Warn: %s (additionally error %s while displaying this warning)\n", msg, err.Error())
	} else {
		ed.sfr.Fr.Sels[0].S = 0
		ed.sfr.Fr.Sels[0].E = ed.bodybuf.Size()

		ed.bodybuf.Replace([]rune(msg), &ed.sfr.Fr.Sels[0], ed.sfr.Fr.Sels, true)
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
