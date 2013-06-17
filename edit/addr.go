package edit

import (
	"fmt"
	"yacco/util"
)

type Addr interface {
	Empty() bool
	String() string
	Eval(sel *util.Sel)
}

type AddrOp struct {
	Op string
	Lh Addr
	Rh Addr
}

func (e *AddrOp) Empty() bool {
	return false
}

func (e *AddrOp) String() string {
	return fmt.Sprintf("Op<%s %s %s>", e.Lh.String(), e.Op, e.Rh.String())
}

func (e *AddrOp) Eval(sel *util.Sel) {
	//TODO
}

type addrEmpty struct {
}

func (e *addrEmpty) Empty() bool {
	return true
}

func (e *addrEmpty) String() string {
	return "·"
}

func (e *addrEmpty) Eval(sel *util.Sel) {
}

type AddrBase struct {
	Batype string
	Value string
	Dir int
}

func (e *AddrBase) Empty() bool {
	return false
}

func (e *AddrBase) String() string{
	dirch := ""
	if e.Dir > 0 {
		dirch = "+"
	} else if e.Dir < 0 {
		dirch = "-"
	}
	return fmt.Sprintf("%s%s%s", dirch, e.Batype, e.Value)
}

func (e *AddrBase) Eval(sel *util.Sel) {
	switch e.Batype {
	case ".":
		// Nothing to do

	case "":
		//TODO: by line

	case "#w":
		//TODO: by word

	case "#":
		//TODO: by character

	case "$":
		//TODO: end of file

	case "?":
		//TODO: regexp backwards

	case "/":
		//TODO: regexp forward

	}
}

type AddrList struct {
	addrs []Addr
}

func (e *AddrList) Empty() bool {
	return false
}

func (e *AddrList) String() string{
	s := "List<"
	for _, addr := range e.addrs {
		s += addr.String() + " "
	}
	s += ">"
	return s
}

func (e *AddrList) Eval(sel *util.Sel) {
	//TODO
}
