package main

import (
	"fmt"
	"io"
	"os"
	"runtime/pprof"
	"yacco/buf"
)

type memdebugsup struct {
	tot uintptr
	w   io.Writer
}

func (sup *memdebugsup) add(name string, x uintptr) {
	sup.tot += x
	fmt.Fprintf(sup.w, "%s: %s\n", name, humanBytes(x))
}

func memdebug(w io.Writer) {
	fmt.Fprintf(w, "WINDOW:\n")
	var sup = memdebugsup{0, w}
	sup.add("\tShiny buffer", uintptr(len(Wnd.wndb.RGBA().Pix)*2))
	sup.add("\tSecond buffer", uintptr(len(Wnd.img.Pix)))
	var wordsz uintptr
	for i := range Wnd.Words {
		wordsz += uintptr(len(Wnd.Words[i]))
	}
	sup.add("\tWords", wordsz)
	sup.add("\tInvalid Rectangles", uintptr(cap(Wnd.invalidRects)*8*4))
	fmt.Fprintf(w, "EDITORS:\n")
	buffers := map[*buf.Buffer]struct{}{}
	for i, col := range Wnd.cols.cols {
		for j, ed := range col.editors {
			sup.add(fmt.Sprintf("\teditor (%d,%d) tag frame glyphs", i, j), ed.tagfr.BytesSize())
			sup.add(fmt.Sprintf("\teditor (%d,%d) body frame glyphs", i, j), ed.sfr.Fr.BytesSize())
			bs := ed.tagbuf.BytesSize()
			sup.add(fmt.Sprintf("\teditor (%d,%d) tag buffer", i, j), bs.GapUsed+bs.GapGap+bs.Words+bs.Undo)
			fmt.Fprintf(w, "\t\tsize of gap %s+%s, words %s, undolist %s\n", humanBytes(bs.GapUsed), humanBytes(bs.GapGap), humanBytes(bs.Words), humanBytes(bs.Undo))
			if _, ok := buffers[ed.bodybuf]; !ok {
				buffers[ed.bodybuf] = struct{}{}
				bs := ed.bodybuf.BytesSize()
				sup.add(fmt.Sprintf("\teditor (%d,%d) body buffer %s", i, j, ed.bodybuf.ShortName()), bs.GapUsed+bs.GapGap+bs.Words+bs.Undo)
				fmt.Fprintf(w, "\t\tsize of gap %s+%s, words %s, undolist %s\n", humanBytes(bs.GapUsed), humanBytes(bs.GapGap), humanBytes(bs.Words), humanBytes(bs.Undo))
			} else {
				fmt.Fprintf(w, "\teditor (%d,%d) body buffer skipped\n", i, j)
			}
		}
	}
	fmt.Fprintf(w, "BUFFERS:\n")
	fmt.Fprintf(w, "TOTAL: %s\n", humanBytes(sup.tot))
	if *memprofileFlag != "" {
		f, err := os.Create(*memprofileFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not create memory profile file: %v", err)
			return
		}
		pprof.WriteHeapProfile(f)
		f.Close()
		return
	}
}

func humanBytes(sz uintptr) string {
	switch {
	case sz > 1024*1024:
		return fmt.Sprintf("%0.03gMB", float64(sz)/float64(1024*1024))
	case sz > 1024:
		return fmt.Sprintf("%0.03gkB", float64(sz)/float64(1024))
	default:
		return fmt.Sprintf("%dB", sz)
	}
}
