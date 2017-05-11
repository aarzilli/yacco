package regexp

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Matchable interface {
	Size() int
	At(int) rune
}

type node interface {
	String() string
	Compile(pgm Regex, bw bool) Regex
}

type nodeChar struct {
	c rune
}

func (n *nodeChar) String() string {
	return fmt.Sprintf("char(%c)", n.c)
}

type nodeClass struct {
	name    string
	inv     bool
	set     map[rune]bool
	special []func(r rune) bool
}

func (n *nodeClass) String() string {
	return fmt.Sprintf("class(%s)", n.name)
}

type nodeGroup struct {
	nodes []node
}

func (n *nodeGroup) String() string {
	r := []string{}
	for i := range n.nodes {
		r = append(r, n.nodes[i].String())
	}
	return fmt.Sprintf("branch(%s)", strings.Join(r, " "))
}

type nodeRep struct {
	min    int // minimum number of repetitions (0 or 1)
	max    int // maximum number of repetitions (-1 for unbound or 1 for only one)
	greedy bool
	child  node
}

func (n *nodeRep) String() string {
	r := n.child.String()
	return fmt.Sprintf("rep(%d%dv %s)", n.min, n.max, n.greedy, r)
}

type nodeAssert struct {
	name  string
	check func(b Matchable, start, end, i int) bool
}

func (n *nodeAssert) String() string {
	return fmt.Sprintf("assert(%s)", n.name)
}

type nodeAlt struct {
	no       int // -1 is an unsaved group
	branches []node
}

func (n *nodeAlt) String() string {
	r := []string{}
	for i := range n.branches {
		r = append(r, n.branches[i].String())
	}
	return fmt.Sprintf("alt(%d %s)", n.no, strings.Join(r, " | "))
}

type parser struct {
	nextgroup int
}

type instrCode uint8

const (
	RX_CHAR = instrCode(iota)
	RX_CLASS
	RX_ASSERT
	RX_MATCH
	RX_JMP
	RX_SPLIT
	RX_SAVE
)

type instr struct {
	op      instrCode
	L       []int                                       // for RX_SPLIT and RX_JMP
	no      int                                         // for RX_SAVE
	c       rune                                        // for RX_CHAR
	cname   string                                      // for RX_CLASS / RX_ASSERT
	inv     bool                                        // for RX_CLASS
	set     map[rune]bool                               // for RX_CLASS
	special []func(rune) bool                           // for RX_CLASS
	check   func(buf Matchable, start, end, i int) bool // for RX_ASSERT
}

type Regex []instr

func (ix *instr) String() string {
	switch ix.op {
	case RX_CHAR:
		ch := "-"
		if unicode.IsPrint(ix.c) {
			ch = fmt.Sprintf("%c", ix.c)
		}
		return fmt.Sprintf("char %d %s", ix.c, ch)
	case RX_CLASS:
		return fmt.Sprintf("class %s", ix.cname)
	case RX_ASSERT:
		return fmt.Sprintf("assert %s", ix.cname)
	case RX_MATCH:
		return fmt.Sprintf("match")
	case RX_JMP:
		return fmt.Sprintf("jmp %d", ix.L[0])
	case RX_SPLIT:
		r := []string{}
		for i := range ix.L {
			r = append(r, strconv.Itoa(ix.L[i]))
		}
		return fmt.Sprintf("split %s", strings.Join(r, ", "))
	case RX_SAVE:
		return fmt.Sprintf("save %d", ix.no)
	default:
		return fmt.Sprintf("unkwn")
	}
}

func (rx *Regex) String() string {
	r := []byte("")
	for i := range *rx {
		r = append(r, []byte(fmt.Sprintf("%04d\t%s\n", i, (*rx)[i].String()))...)
	}
	return string(r)
}

type RuneArrayMatchable []rune

func (ram RuneArrayMatchable) At(i int) rune {
	return ram[i]
}

func (ram RuneArrayMatchable) Size() int {
	return len(ram)
}
