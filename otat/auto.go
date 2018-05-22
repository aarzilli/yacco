package otat

type autoligature struct {
	seq string
	out string
}

var autoligatures = []autoligature{
	{":=", "≔"},
	{"!=", "≠"},
	{"<=", "≤"},
	{">=", "≥"},
	{"...", "…"},
	{"<-", "←"},
	{"->", "→"},
}

func (m *Machine) setupAuto() (maxbacktrack, maxlookahead int) {
	unknownIdx := m.index(rune(0xfffd))
	zerowidthIdx := m.index(rune(0x200b))
	if zerowidthIdx == unknownIdx {
		return
	}
	var lookup lookup
	lookup.typ = 1000
	for _, autoligature := range autoligatures {
		substrune := []rune(autoligature.out)[0]
		idx := m.index(substrune)
		if idx == unknownIdx {
			continue
		}
		//fmt.Printf("%s -> %d\n", autoligature.out, idx)
		seq := []rune(autoligature.seq)
		if len(seq)-1 > maxlookahead {
			maxlookahead = len(seq) - 1
		}
		lt := lookupTable{}
		lt.cov = m.makeCoverage(seq[0])
		for _, ch := range seq[1:] {
			lt.lookaheadCov = append(lt.lookaheadCov, m.makeCoverage(ch))
		}
		lt.substitute = []Index{idx}
		lt.substrune = substrune
		lookup.tables = append(lookup.tables, lt)
	}

	m.lookups = append(m.lookups, &lookup)

	return maxbacktrack, maxlookahead
}

func (m *Machine) makeCoverage(ch rune) coverage {
	idx := m.index(ch)
	return coverage{
		sparse: []Index{idx},
	}
}
