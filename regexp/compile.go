package regexp

import (
	"fmt"
)

/**
Compiles a regex
- rx: the regular expression string
- find: true if we want to do a find operation, false if we want to do a match operation
- bw: compile the regular expression backwards
*/
func Compile(rx string, find, bw bool) Regex {
	defer func() {
		if ierr := recover(); ierr != nil {
			err := ierr.(error)
			panic(fmt.Errorf("Error compiling %s: %s", rx, err.Error()))
		}
	}()
	pgm := []instr{}

	if find {
		nf := &nodeRep{min: 0, max: -1, greedy: false, child: &realDotClass}
		pgm = nf.Compile(pgm, bw)
	}

	var p parser
	ast := p.parseToplevel([]rune(rx))
	pgm = ast.Compile(pgm, bw)
	pgm = append(pgm, instr{op: RX_MATCH})
	return pgm
}

/*
Compiles a regex to do a non-contiguous search of string s
*/
func CompileFuzzySearch(s []rune) Regex {
	defer func() {
		if ierr := recover(); ierr != nil {
			err := ierr.(error)
			panic(fmt.Errorf("Error compiling %s for fuzzy match: %s", s, err.Error()))
		}
	}()

	nodes := make([]node, len(s)*2)

	for i := range s {
		nodes[i*2] = &nodeAlt{
			no:       i * 2,
			branches: []node{&nodeRep{min: 0, max: -1, greedy: false, child: &dotClass}},
		}
		nodes[i*2+1] = &nodeAlt{
			no:       i*2 + 1,
			branches: []node{&nodeChar{s[i]}},
		}
	}

	ast := &nodeAlt{
		no:       -1,
		branches: []node{&nodeGroup{nodes: nodes}},
	}

	pgm := []instr{}
	pgm = ast.Compile(pgm, false)
	pgm = append(pgm, instr{op: RX_MATCH})
	return pgm
}

func (n *nodeChar) Compile(pgm Regex, bw bool) Regex {
	return append(pgm, instr{op: RX_CHAR, c: n.c})
}

func (n *nodeClass) Compile(pgm Regex, bw bool) Regex {
	return append(pgm, instr{op: RX_CLASS, cname: n.name, inv: n.inv, set: n.set, special: n.special})
}

func (n *nodeAssert) Compile(pgm Regex, bw bool) Regex {
	return append(pgm, instr{op: RX_ASSERT, cname: n.name, check: n.check})
}

func (n *nodeGroup) Compile(pgm Regex, bw bool) Regex {
	if !bw {
		for i := range n.nodes {
			pgm = n.nodes[i].Compile(pgm, bw)
		}
	} else {
		for i := len(n.nodes) - 1; i >= 0; i-- {
			pgm = n.nodes[i].Compile(pgm, bw)
		}
	}
	return pgm
}

func (n *nodeRep) Compile(pgm Regex, bw bool) Regex {
	if (n.min == 1) && (n.max <= 0) { // +
		topl := len(pgm)
		pgm = n.child.Compile(pgm, bw)
		if n.greedy {
			pgm = append(pgm, instr{op: RX_SPLIT, L: []int{topl, len(pgm) + 1}})
		} else {
			pgm = append(pgm, instr{op: RX_SPLIT, L: []int{len(pgm) + 1, topl}})
		}
		return pgm
	}

	if (n.min == 0) && (n.max <= 0) { // *
		topl := len(pgm)
		pgm = append(pgm, instr{op: RX_SPLIT, L: []int{0, 0}})
		pgm = n.child.Compile(pgm, bw)
		pgm = append(pgm, instr{op: RX_JMP, L: []int{topl}})
		if n.greedy {
			pgm[topl].L[0] = topl + 1
			pgm[topl].L[1] = len(pgm)
		} else {
			pgm[topl].L[0] = len(pgm)
			pgm[topl].L[1] = topl + 1
		}
		return pgm
	}

	if (n.min == 0) && (n.max == 1) { // +
		topl := len(pgm)
		pgm = append(pgm, instr{op: RX_SPLIT, L: []int{0, 0}})
		pgm = n.child.Compile(pgm, bw)
		if n.greedy {
			pgm[topl].L[0] = topl + 1
			pgm[topl].L[1] = len(pgm)
		} else {
			pgm[topl].L[0] = len(pgm)
			pgm[topl].L[1] = topl + 1
		}
		return pgm
	}

	panic(fmt.Errorf("Unknown min/max combination for repeat node %d %d", n.min, n.max))
}

func (n *nodeAlt) Compile(pgm Regex, bw bool) Regex {
	if n.no >= 0 {
		pgm = append(pgm, instr{op: RX_SAVE, no: n.no * 2})
	}

	if len(n.branches) == 0 {
		if n.no >= 0 {
			pgm = append(pgm, instr{op: RX_SAVE, no: n.no*2 + 1})
		}
		return pgm
	}

	if len(n.branches) == 1 {
		pgm = n.branches[0].Compile(pgm, bw)
		if n.no >= 0 {
			pgm = append(pgm, instr{op: RX_SAVE, no: n.no*2 + 1})
		}
		return pgm
	}

	topl := len(pgm)
	pgm = append(pgm, instr{op: RX_SPLIT})

	Ls := []int{}
	for i := range n.branches {
		Ls = append(Ls, len(pgm))
		pgm = n.branches[i].Compile(pgm, bw)
		if i != len(n.branches)-1 {
			pgm = append(pgm, instr{op: RX_JMP, L: []int{0}})
		}
	}

	endl := len(pgm)

	pgm[topl].L = Ls
	for i := 1; i < len(n.branches); i++ {
		pgm[Ls[i]-1].L[0] = endl
	}

	if n.no >= 0 {
		pgm = append(pgm, instr{op: RX_SAVE, no: n.no*2 + 1})
	}

	return pgm
}
