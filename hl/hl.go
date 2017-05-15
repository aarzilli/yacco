package hl

import (
	"regexp"
	"sort"

	yregexp "yacco/regexp"
)

type Highlighter interface {
	Highlight(start, end int, buf yregexp.Matchable, outbuf []uint8) []uint8
	Toregend(start int, buf yregexp.Matchable) int
	Alter(idx int)
}

var _ Highlighter = &nilHighlighter{}
var _ Highlighter = &Syncs{}
var _ Highlighter = &Fixed{}

// Implementation of Highlihgter that does nothing
type nilHighlighter struct {
}

var NilHighlighter = &nilHighlighter{}

func (hl *nilHighlighter) Highlight(start, end int, buf yregexp.Matchable, outbuf []uint8) []uint8 {
	if cap(outbuf) > end-start {
		outbuf = outbuf[:end-start]
	} else {
		outbuf = make([]byte, end-start)
	}

	for i := range outbuf {
		outbuf[i] = 1
	}
	return outbuf
}

func (hl *nilHighlighter) Alter(int) {
}

func (hl *nilHighlighter) Toregend(start int, buf yregexp.Matchable) int {
	return -1
}

// Implementation of Highlighter based on synchronization points
type Syncs struct {
	matches  []RegionMatch
	syncs    []sync
	tempSync sync
}

type sync struct {
	index int
	state int
}

func New(rules []LanguageRules, name string) Highlighter {
	var matches []RegionMatch = nil
	for i := range rules {
		if rules[i].re == nil {
			rules[i].re = regexp.MustCompile(rules[i].NameRe)
		}
		if rules[i].re.MatchString(name) {
			matches = rules[i].RegionMatches
		}
	}
	if matches == nil {
		return NilHighlighter
	}
	return &Syncs{matches: matches, tempSync: sync{0, 0}, syncs: []sync{{0, 0}}}
}

func (syncs *Syncs) find(idx int) int {
	return sort.Search(len(syncs.syncs), func(i int) bool { return syncs.syncs[i].index >= idx }) - 1
}

func (syncs *Syncs) Alter(idx int) {
	if len(syncs.syncs) == 0 {
		return
	}
	if idx < 0 {
		idx = 0
	}
	i := syncs.find(idx)
	if i < 0 {
		i = 0
	}
	syncs.syncs = syncs.syncs[:i]
	if syncs.tempSync.index >= idx {
		syncs.tempSync = sync{0, 0}
	}
	if len(syncs.syncs) == 0 {
		syncs.syncs = append(syncs.syncs, sync{0, 0})
	}
}

func matchDelim(delim []rune, start int, buf yregexp.Matchable) bool {
	for j := range delim {
		if delim[j] != buf.At(start+j) {
			return false
		}
	}
	return true
}

func (syncs *Syncs) scan(sy sync, buf yregexp.Matchable) (sync, RegionMatchType) {
	if sy.state == 0 {
		for i, m := range syncs.matches {
			if m.StartDelim != nil {
				if matchDelim(m.StartDelim, sy.index, buf) {
					return sync{sy.index + len(m.StartDelim), i + 1}, m.DelimType
				}
			} else {
				if match := m.StartRegexp.Match(buf, sy.index, buf.Size(), +1); match != nil {
					return sync{match[1], i + 1}, m.DelimType
				}
			}
		}

		return sync{sy.index + 1, 0}, 1
	}

	i := sy.state - 1
	m := syncs.matches[i]

	if m.EndDelim != nil {
		if matchDelim(m.EndDelim, sy.index, buf) {
			return sync{sy.index + len(m.EndDelim), 0}, m.DelimType
		}
	} else {
		if match := m.EndRegexp.Match(buf, sy.index, buf.Size(), +1); match != nil {
			return sync{match[1], 0}, m.DelimType
		}
	}

	if buf.At(sy.index) == m.Escape {
		return sync{sy.index + 2, sy.state}, m.Type
	}

	return sync{sy.index + 1, sy.state}, m.Type
}

const syncInterval = 128

func (syncs *Syncs) syncFor(start int) (sy sync, atend bool) {
	if start == syncs.tempSync.index {
		sy = syncs.tempSync
		if len(syncs.syncs) == 0 || sy == syncs.syncs[len(syncs.syncs)-1] {
			atend = true
		}
	} else if len(syncs.syncs) == 0 {
		sy = sync{0, 0}
		atend = true
	} else {
		i := syncs.find(start)
		atend = (i == len(syncs.syncs)-1)
		if i >= 0 {
			sy = syncs.syncs[i]
		} else {
			sy = sync{0, 0}
		}
	}
	return
}

func (syncs *Syncs) Highlight(start, end int, buf yregexp.Matchable, outbuf []uint8) []uint8 {
	sy, atend := syncs.syncFor(start)

coloringLoop:
	for {
		nextSy, color := syncs.scan(sy, buf)
		for i := sy.index; i < nextSy.index; i++ {
			if i == start {
				syncs.tempSync = nextSy
			}
			if i >= end {
				break coloringLoop
			}
			if i >= start && i < end {
				outbuf = append(outbuf, uint8(color))
			}
		}
		sy = nextSy
		if atend {
			if len(syncs.syncs) == 0 || sy.index-syncs.syncs[len(syncs.syncs)-1].index > syncInterval {
				syncs.syncs = append(syncs.syncs, sy)
			}
		}
	}

	return outbuf
}

func (syncs *Syncs) Toregend(start int, buf yregexp.Matchable) int {
	sy, _ := syncs.syncFor(start - 1)
	var prevcolor, curcolor RegionMatchType
	for {
		if sy.index >= buf.Size() {
			return -1
		}
		nextSy, color := syncs.scan(sy, buf)
		for i := sy.index; i < nextSy.index; i++ {
			switch i {
			case start - 1:
				prevcolor = color
			case start:
				if color == prevcolor {
					return -1
				}
				curcolor = color
			}
			if curcolor != 0 && curcolor != color {
				return i - 1
			}
		}
		sy = nextSy
	}
	return start
}

func (syncs *Syncs) Size() int {
	return len(syncs.syncs)
}

type Fixed struct {
	color []uint8
}

func NewFixed(start int) *Fixed {
	r := &Fixed{make([]uint8, start)}
	for i := range r.color {
		r.color[i] = 1
	}
	return r
}

func (fhl *Fixed) Highlight(start int, end int, buf yregexp.Matchable, outbuf []uint8) []uint8 {
	if start < len(fhl.color) {
		e := end
		if e > len(fhl.color) {
			e = len(fhl.color)
		}
		outbuf = append(outbuf, fhl.color[start:e]...)
	}
	if d := (end - start) - len(outbuf); d > 0 {
		for i := 0; i < d; i++ {
			outbuf = append(outbuf, 1)
		}
	}
	return outbuf
}

func (fhl *Fixed) Toregend(start int, buf yregexp.Matchable) int {
	return -1
}

func (fhl *Fixed) Alter(idx int) {
	if idx < 0 {
		idx = 0
	}
	if idx < len(fhl.color) {
		fhl.color = fhl.color[:idx]
	}
	return
}

func (fhl *Fixed) Append(color []uint8) {
	fhl.color = append(fhl.color, color...)
}
