package main

import (
	"yacco/util"
)

func elasticTabs(ed *Editor, ignoreOverlong bool) {
	tabWidth := ed.sfr.Fr.Measure([]rune("\t"))
	spaceWidth := ed.sfr.Fr.Measure([]rune(" ")) * 2

	s := 0
	f := 0
	tabs := []int{}
	invalid := false

	completeField := func(e int) {
		if invalid {
			return
		}

		sz := ed.sfr.Fr.Measure(ed.bodybuf.SelectionRunes(util.Sel{s, e})) + spaceWidth
		if !ignoreOverlong && (sz >= tabWidth*8) {
			invalid = true
			return
		}

		if f >= len(tabs) {
			nt := make([]int, f+1)
			for i := range nt {
				if i < len(tabs) {
					nt[i] = tabs[i]
				} else {
					nt[i] = 0
				}
			}
			tabs = nt
		}

		if sz > tabs[f] {
			tabs[f] = sz
		}

		s = e + 1
	}

	for i := 0; i < ed.bodybuf.Size(); i++ {
		switch ed.bodybuf.At(i).R {
		case '\t':
			completeField(i)
			f++
		case '\n':
			completeField(i)
			invalid = false
			f = 0
		}
	}

	for i := 1; i < len(tabs); i++ {
		tabs[i] += tabs[i-1]
	}

	if len(tabs) > 0 {
		ed.sfr.Fr.Tabs = tabs
	}
}
