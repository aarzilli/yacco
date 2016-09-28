package otat

import (
	"fmt"
)

func (m *Machine) Reset(in []rune) {
	m.runes = in
	m.runeit = nil
	m.resetCommon()
}

func (m *Machine) ResetIterator(in func() (rune, bool)) {
	m.runes = nil
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
		fmt.Printf("= INIT %d %d %d =\n", len(m.runes), len(m.backtrack), len(m.lookahead))
	}
	for i := len(m.backtrack) + 1; i < len(m.window); i++ {
		var ok bool
		m.window[i], ok = m.rune0()
		if ok {
			m.load++
		}
	}
	if debugProcess {
		fmt.Printf("= END INIT =\n")
	}
}

func (m *Machine) rune0() (Index, bool) {
	var ch rune
	var ok bool
	if m.runes != nil {
		ok = len(m.runes) > 0
		if ok {
			ch = m.runes[0]
			m.runes = m.runes[1:]
		}
	} else {
		ch, ok = m.runeit()
	}

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

func (m *Machine) Glyph() rune {
	if m.dummy {
		return rune(*m.input)
	}
	return -rune(*m.input)
}
