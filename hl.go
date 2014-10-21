package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"yacco/buf"
	"yacco/config"
	"yacco/util"
)

const TraceHighlight = false
const TraceHighlightExtra = false

type hlMachine struct {
	nameRe *regexp.Regexp
	states []hlState
}

type hlState struct {
	mark      util.RegionMatchType
	trans     []hlTransition
	failTrans hlTransition
}

type hlTransition struct {
	match    rune
	mark     util.RegionMatchType // color to mark this matched character
	backMark int
	next     uint16 // next state
}

var hlMachines map[string]*hlMachine = nil

func (s *hlState) addTransition(match rune, mark util.RegionMatchType, backMark int, next uint16) {
	s.trans = append(s.trans, hlTransition{match: match, mark: mark, backMark: backMark, next: next})
}

func (m *hlMachine) addState(mark util.RegionMatchType, failTrans hlTransition, trans ...hlTransition) uint16 {
	m.states = append(m.states, hlState{mark, trans, failTrans})
	return uint16(len(m.states) - 1)
}

func (m *hlMachine) addProgression(startState, endState uint16, startColor, endColor util.RegionMatchType, seq []rune) {
	curState := startState
	for i := range seq {
		nextState := -1
		for j := range m.states[curState].trans {
			if m.states[curState].trans[j].match == seq[i] {
				nextState = int(m.states[curState].trans[j].next)
				break
			}
		}
		if i == len(seq)-1 {
			if nextState < 0 {
				m.states[curState].addTransition(seq[i], endColor, len(seq)-1, endState)
			} else {
				m.states[nextState].failTrans = hlTransition{match: 0, mark: endColor, backMark: len(seq) - 1, next: endState}
			}
		} else {
			if nextState < 0 {
				nextState = int(m.addState(startColor, hlTransition{0, 0, 0, startState}))
				m.states[curState].addTransition(seq[i], startColor, 0, uint16(nextState))
			}
			curState = uint16(nextState)
		}
	}
}

func (m *hlMachine) addEscape(state uint16, color util.RegionMatchType, r rune) {
	escapeState := m.addState(color, hlTransition{match: 0, mark: color, backMark: 0, next: state})
	m.states[state].addTransition(r, color, 0, escapeState)
}

func compileHl() {
	hlMachines = map[string]*hlMachine{}
	for _, r := range config.RegionMatches {
		if _, ok := hlMachines[r.NameRe]; !ok {
			hlMachines[r.NameRe] = &hlMachine{regexp.MustCompile(r.NameRe), []hlState{}}
			hlMachines[r.NameRe].addState(0x01, hlTransition{match: 0, mark: 0x01, backMark: 0, next: 0}) // state 0
		}
		m := hlMachines[r.NameRe]
		newState := m.addState(r.Type, hlTransition{0, r.Type, 0, 0})
		m.states[newState].failTrans.next = newState
		m.addProgression(0, newState, 0x01, r.Type, r.StartDelim)
		m.addProgression(newState, 0, r.Type, r.Type, r.EndDelim)
		if r.Escape != 0 {
			m.addEscape(newState, r.Type, r.Escape)
		}
	}
	for _, m := range hlMachines {
		if len(m.states) > 255 {
			panic(fmt.Errorf("Too many highlighting states"))
		}
	}
	if TraceHighlight {
		for name, m := range hlMachines {
			fmt.Printf("MACHINE: %s\n", name)
			for i := range m.states {
				for j := range m.states[i].trans {
					t := m.states[i].trans[j]
					normal := func() {
						fmt.Printf("\t%02d: %c -> %02d (mark: %d %d)\n", i, t.match, t.next, t.mark, t.backMark)
					}
					if t.match == 0 {
						if t.next == uint16(i) {
							fmt.Printf("\t%02d: ERROR %d\n", i, t.mark)
						} else if t.next == 0 && t.mark == 0 {
							fmt.Printf("\t%02d: ERROR\n", i)
						}
					} else {
						normal()
					}
				}
				if m.states[i].failTrans.next == uint16(i) {
					fmt.Printf("\t%02d: all %d\n", i, m.states[i].failTrans.mark)
				} else if m.states[i].failTrans.mark != 0 {
					fmt.Printf("\t%02d: all -> %d (%d)\n", i, m.states[i].failTrans.next, m.states[i].failTrans.mark)
				} else {
					fmt.Printf("\t%02d: fail %d\n", i, m.states[i].failTrans.next)
				}
			}
		}
	}
}

func Highlight(b *buf.Buffer, end int) {
	if !config.EnableHighlighting {
		return
	}

	if hlMachines == nil {
		compileHl()
	}

	if b.IsDir() {
		return
	}

	if (len(b.Name) == 0) || (b.Name[0] == '+') {
		return
	}

	if b.HlGood >= b.Size() {
		return
	}

	path := filepath.Join(b.Dir, b.Name)
	var m *hlMachine = nil
	found := false
	for _, m = range hlMachines {
		if m.nameRe.MatchString(path) {
			found = true
			break
		}
	}

	if !found {
		return
	}

	start := b.HlGood

	status := uint16(0)
	if start >= 0 {
		status = b.At(start).C >> 4
	}

	if end >= b.Size() {
		end = b.Size() - 1
	}

	if TraceHighlight {
		ch := rune(0)
		if start >= 0 {
			ch = b.At(start).R
		}
		fmt.Printf("Highlighting from %d to %d\n", b.HlGood, end)
		fmt.Printf("Starting status: %d starting character %c\n", status, ch)
	}

	for i := start + 1; i <= end; {
		s := m.states[status]
		r := b.At(i).R
		if TraceHighlightExtra {
			fmt.Printf("State %d char %c\n", status, r)
		}
		found := false
		for _, t := range s.trans {
			if t.match == r {
				if TraceHighlightExtra {
					fmt.Printf("\tTransition %c %d (%d) %d\n", t.match, t.mark, t.backMark, t.next)
				}
				found = true
				status = t.next
				b.At(i).C = (status << 4) + uint16(t.mark)
				for j := 0; j < t.backMark; j++ {
					s := b.At(i-j-1).C & 0xfff0
					b.At(i - j - 1).C = s + uint16(t.mark)
				}
				i++
				break
			}
		}
		if !found {
			if TraceHighlightExtra {
				fmt.Printf("\tTransition fallback %d %d\n", s.failTrans.mark, s.failTrans.next)
			}
			status = s.failTrans.next
			if s.failTrans.mark != 0 {
				b.At(i).C = (status << 4) + uint16(s.failTrans.mark)
				i++
			}
		}
	}
}
