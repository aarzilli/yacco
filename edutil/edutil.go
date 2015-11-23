package edutil

import (
	"yacco/buf"
	"yacco/textframe"
	"yacco/util"
)

type HighlightFn func(buf *buf.Buffer, end int)

func MakeExpandSelectionFn(buf *buf.Buffer) func(kind, start, end int) (int, int) {
	return func(kind, start, end int) (int, int) {
		return expandSelectionBuf(buf, kind, start, end)
	}
}

func expandSelectionBuf(buf *buf.Buffer, kind, start, end int) (rstart, rend int) {
	switch kind {
	default:
		fallthrough
	case 1:
		return start, end
	case 2:
		return buf.Towd(start, -1, false), buf.Towd(end, +1, true)
	case 3:
		return buf.Tonl(start-1, -1), buf.Tonl(end, +1)
	}
}

func Scrollfn(buf *buf.Buffer, top *util.Sel, sfr *textframe.ScrollFrame, sd, sl int, hlf HighlightFn) {
	buf.Rdlock()
	defer buf.Rdunlock()

	DoHighlightingConsistency(buf, top, sfr, hlf)

	sz := buf.Size()

	switch {
	case sd == 0:
		top.E = buf.Tonl(sl, -1)

		sz := buf.Size()

		a, b := buf.Selection(util.Sel{top.E, sz})
		sfr.Fr.Clear()
		sfr.Fr.InsertColor(a)
		sfr.Fr.InsertColor(b)

	case sd > 0:
		n := sfr.Fr.PushUp(sl, true)
		top.E = sfr.Fr.Top
		a, b := buf.Selection(util.Sel{top.E + n, sz})
		sfr.Fr.InsertColor(a)
		sfr.Fr.InsertColor(b)

	case sd < 0:
		nt := top.E
		for i := 0; i < sl; i++ {
			nt = buf.Tonl(nt-2, -1)
		}

		a, b := buf.Selection(util.Sel{nt, top.E})

		if len(a)+len(b) == 0 {
			return
		}

		sfr.Fr.PushDown(sl, a, b)
		top.E = sfr.Fr.Top
	}

	sfr.Set(top.E, sz)
	sfr.Redraw(true, nil)
}

func MakeScrollfn(buf *buf.Buffer, top *util.Sel, sfr *textframe.ScrollFrame, hlf HighlightFn) func(sd, sl int) {
	return func(sd, sl int) {
		Scrollfn(buf, top, sfr, sd, sl, hlf)
	}
}

func DoHighlightingConsistency(buf *buf.Buffer, top *util.Sel, sfr *textframe.ScrollFrame, hlf HighlightFn) {
	sz := sfr.Fr.Size() * 5
	if sz < 100 {
		sz = 100
	}
	end := top.E + sz
	if end <= buf.HlGood {
		return
	}

	hlf(buf, end)
}
