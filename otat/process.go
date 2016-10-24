package otat

import (
	"fmt"
)

func (m *Machine) Reset(in func() (rune, bool)) {
	m.runeit = in
	m.resetCommon()
}

func (m *Machine) resetCommon() {
	if m.dummy {
		m.window = make([]Index, 1)
		m.input = &m.window[0]
		return
	}
	for i := range m.backtrack {
		m.backtrack[i] = 0
	}
	*m.input = 0
	m.load = 0
	if debugProcess {
		fmt.Printf("= INIT %d %d =\n", len(m.backtrack), len(m.lookahead))
	}
	for i := len(m.backtrack) + 1; i < len(m.window); i++ {
		var ok bool
		m.window[i], ok = m.rune0()
		if ok {
			m.load++
		}
	}
	m.curIdx = -1
	if debugProcess {
		fmt.Printf("= END INIT =\n")
	}
}

// Index returns a Font's index for the given rune.
func (m *Machine) index(x rune) Index {
	c := uint32(x)
	for i, j := 0, len(m.cm); i < j; {
		h := i + (j-i)/2
		cm := &m.cm[h]
		if c < cm.start {
			j = h
		} else if cm.end < c {
			i = h + 1
		} else if cm.offset == 0 {
			return Index(c + cm.delta)
		} else {
			offset := int(cm.offset) + 2*(h-len(m.cm)+int(c-cm.start))
			return Index(u16(m.cmapIndexes, offset))
		}
	}
	return 0
}

func (m *Machine) rune0() (Index, bool) {
	ch, ok := m.runeit()

	var r Index
	if ok {
		if m.lookups == nil {
			r = Index(ch)
		} else {
			r = m.index(ch)
		}
		if debugProcess {
			fmt.Printf("Read %q (%d) <%v>\n", string(ch), r, m.dummy)
		}
	}
	return r, ok

}

func (m *Machine) slideWindow() {
	for i := 1; i < len(m.window); i++ {
		m.window[i-1] = m.window[i]
	}
	var ok bool
	m.window[len(m.window)-1], ok = m.rune0()
	if !ok {
		m.load--
	}
}

func (m *Machine) Next() bool {
	m.curIdx++
	if m.dummy {
		var ok bool
		*m.input, ok = m.rune0()
		return ok
	}
	if m.load <= 0 {
		return false
	}
	m.slideWindow()
	if debugProcess {
		pid := -1
		if m.prevlookup != nil {
			pid = m.prevlookup.id
		}
		fmt.Printf("\tback: %v input %d lookahead: %v (load: %d) prevlookup: %d\n", m.backtrack, *m.input, m.lookahead, m.load, pid)
	}
	if m.prevlookup != nil {
		ok := m.prevlookup.apply(m)
		if !ok {
			m.prevlookup = nil
		}
	}
	if m.prevlookup == nil {
		for _, lp := range m.lookups {
			ok := lp.apply(m)
			if ok {
				m.prevlookup = lp
				break
			}
		}
	}
	return true
}

func (lookup *lookup) apply(m *Machine) bool {
	for _, lp := range lookup.tables {
		covered := lp.cov.Covers(*m.input)
		if covered < 0 {
			continue
		}
		switch lookup.typ {
		case 1:
			lp.used = true
			if lp.substitute == nil {
				*m.input = Index(int(*m.input) + int(lp.delta))
			} else {
				*m.input = lp.substitute[covered]
			}
			if debugProcess {
				fmt.Printf("\tfound matching single lookup %d (as index %d) → %d\n", lookup.id, covered, *m.input)
			}
			return true
		case 6:
			if lp.cover6(m.backtrack, m.lookahead) {
				lp.used = true
				if lp.slp != nil {
					lp.slp.used = true
					c := lp.slp.cov.Covers(*m.input)
					if lp.slp.substitute == nil {
						*m.input = Index(int(*m.input) + int(lp.slp.delta))
					} else {
						*m.input = lp.slp.substitute[c]
					}
				}
				if debugProcess {
					fmt.Printf("\tfound matching contextual lookup %d → %d\n", lookup.id, *m.input)
				}
				return true
			}
		}
	}
	return false
}

func (lp *lookupTable) cover6(backtrack, lookahead []Index) bool {
	for i := range lp.backtrackCov {
		if lp.backtrackCov[i].Covers(backtrack[len(backtrack)-1-i]) < 0 {
			return false
		}
	}
	for i := range lp.lookaheadCov {
		if lp.lookaheadCov[i].Covers(lookahead[i]) < 0 {
			return false
		}
	}
	return true
}

func (m *Machine) Glyph() (int, rune) {
	if m.dummy {
		return m.curIdx, rune(*m.input)
	}
	return m.curIdx, -rune(*m.input)
}
