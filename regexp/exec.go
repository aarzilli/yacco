package regexp

import (
	"fmt"
)

var rxDebug = false

type threadlet struct {
	pc   int
	save []int
}

type threadlist struct {
	rx      *Regex
	set     []bool
	threads []threadlet
}

func (t *threadlet) spawn(pc int) threadlet {
	save := make([]int, len(t.save))
	copy(save, t.save)
	return threadlet{pc: pc, save: save}
}

func (rx *Regex) newThreadlist() *threadlist {
	return &threadlist{
		rx:      rx,
		set:     make([]bool, len(*rx)),
		threads: make([]threadlet, 0, len(*rx)),
	}
}

func (tl *threadlist) addthread(t threadlet, b Matchable, start, end, i int) {
	if ok := tl.set[t.pc]; ok {
		return
	}
	tl.set[t.pc] = true

	if rxDebug {
		fmt.Printf("\taddthread %d\n", t.pc)
	}

	switch ix := (*tl.rx)[t.pc]; ix.op {
	case RX_ASSERT:
		ok := ix.check(b, start, end, i)
		if ok {
			t.pc++
			if rxDebug {
				fmt.Printf("\t\t-> %d\n", t.pc+1)
			}
			tl.addthread(t, b, start, end, i)
		} else {
			if rxDebug {
				fmt.Printf("\t\t-> assert failed\n")
			}
		}

	case RX_SAVE:
		t.save[ix.no] = i
		t.pc++
		if rxDebug {
			fmt.Printf("\t\t-> %d\n", t.pc)
		}
		tl.addthread(t, b, start, end, i)

	case RX_JMP:
		t.pc = ix.L[0]
		if rxDebug {
			fmt.Printf("\t\t-> %d\n", t.pc)
		}
		tl.addthread(t, b, start, end, i)

	case RX_SPLIT:
		if rxDebug {
			fmt.Printf("\t\t-> ")
			for _, j := range ix.L {
				fmt.Printf("%d, ", j)
			}
			fmt.Printf("\n")
		}
		for _, j := range ix.L {
			tl.addthread(t.spawn(j), b, start, end, i)
		}

	default:
		tl.threads = append(tl.threads, t)
	}
}

func (tl *threadlist) reset() {
	for i := range tl.set {
		tl.set[i] = false
	}
	tl.threads = tl.threads[:0]
}

func (rx *Regex) Match(b Matchable, start, end int, dir int) []int {
	if len(*rx) <= 0 {
		return []int{start, start}
	}

	if dir == 0 {
		dir = 1
	}

	if rxDebug {
		fmt.Printf("CODE:\n%s\n", rx.String())
	}

	ssz := 2
	for _, ix := range *rx {
		if (ix.op == RX_SAVE) && (ix.no >= ssz) {
			ssz = ix.no + 1
		}
	}

	save := make([]int, ssz)
	for i := range save {
		save[i] = -1
	}
	matched := false

	clist := rx.newThreadlist()
	nlist := rx.newThreadlist()

	fsave := make([]int, ssz)
	for i := range fsave {
		fsave[i] = -1
	}

	clist.addthread(threadlet{0, fsave}, b, start, end, start)

	for i := start; ; i += dir {
		if len(clist.threads) == 0 {
			break
		}

		var ch rune
		if i >= b.Size() {
			ch = 0
		} else if i < 0 {
			ch = 0
		} else {
			ch = b.At(i)
		}

		if dir >= 0 {
			if i > b.Size() {
				break
			}
			if (end >= 0) && (i > end) {
				break
			}
		} else {
			if i < -1 {
				break
			}
			if (end >= 0) && (i <= end) {
				break
			}
		}

		if rxDebug {
			fmt.Printf("At: %d (%d) threads: ", i, ch)
			for j := range clist.threads {
				fmt.Printf("%d, ", clist.threads[j].pc)
			}
			fmt.Printf("\n")
		}

	threadletLoop:
		for j := 0; j < len(clist.threads); j++ {
			switch ix := (*rx)[clist.threads[j].pc]; ix.op {
			case RX_CHAR:
				if rxDebug {
					fmt.Printf("\t%d -> compare %d %d\n", clist.threads[j].pc, ix.c, ch)
				}
				if ix.c != ch {
					break
				}
				nlist.addthread(clist.threads[j].spawn(clist.threads[j].pc+1), b, start, end, i+dir)

			case RX_CLASS:
				if ch == 0 {
					break
				}

				ok := false
				if ix.special != nil {
					for _, f := range ix.special {
						if f(ch) {
							ok = true
							break
						}
					}
				}
				if !ok && (ix.set != nil) {
					_, ok = ix.set[ch]
				}
				if ix.inv {
					ok = !ok
				}

				if rxDebug {
					fmt.Printf("\t%d -> class %s %v\n", clist.threads[j].pc, ix.cname, ok)
				}

				if !ok {
					break
				}
				nlist.addthread(clist.threads[j].spawn(clist.threads[j].pc+1), b, start, end, i+dir)

			case RX_MATCH:
				if rxDebug {
					fmt.Printf("\t%d -> match! (wiping low priority threads)\n", clist.threads[j].pc)
				}
				copy(save, clist.threads[j].save)
				matched = true
				break threadletLoop // lower priority threads discarded

			// jmp, split, save, assert handled by addthread
			default:
				panic(fmt.Errorf("Unhandled: %d %s", j, ix.String()))
			}
		}

		tlist := clist
		clist = nlist
		nlist = tlist
		nlist.reset()
	}

	if matched {
		return save
	} else {
		return nil
	}
}
