package visit

import (
	"sort"

	"golang.org/x/tools/go/packages"
)

// Packages returns a packages iterator for the pkgs
func Packages(pkgs []*packages.Package) *PackagesIterator {
	return &PackagesIterator{
		fringeStack: []*[]*packages.Package{&pkgs},
		seen:        make(map[*packages.Package]bool),
	}
}

// PackagesIterator will visit a package and its imports in pre-order.
// After aving visited all the imports of a package it will return nil as the current package.
// For example given this tree:
//
//              a
//             / \
//            b   c
//           / \
//          d   e
//
// It will return:
// a, b, d, e, nil, c, nil
//
// Packages that are imported by multiple packages are only returned once.
type PackagesIterator struct {
	fringeStack []*[]*packages.Package
	stack       []*packages.Package
	cur         *packages.Package
	seen        map[*packages.Package]bool
}

// Moves to the next package in the sequence.
func (it *PackagesIterator) Next() bool {
	for {
		if len(it.fringeStack) == 0 {
			return false
		}
		fringe := it.fringeStack[len(it.fringeStack)-1]
		if fringe != nil && len(*fringe) != 0 {
			it.cur = (*fringe)[0]
			(*fringe) = (*fringe)[1:]
			if !it.seen[it.cur] || it.cur == nil {
				it.seen[it.cur] = true
				it.visitCur()
				return true
			}
		} else {
			it.fringeStack = it.fringeStack[:len(it.fringeStack)-1]
		}
	}
}

func (it *PackagesIterator) visitCur() {
	if it.cur == nil {
		it.fringeStack = append(it.fringeStack, nil)
		return
	}
	paths := make([]string, 0, len(it.cur.Imports))
	for path := range it.cur.Imports {
		paths = append(paths, path)
	}
	curfringe := make([]*packages.Package, 0, len(it.cur.Imports)+1)
	sort.Strings(paths)
	for _, path := range paths {
		curfringe = append(curfringe, it.cur.Imports[path])
	}
	curfringe = append(curfringe, nil)
	it.fringeStack = append(it.fringeStack, &curfringe)
}

// Pkg returns the current package.
func (it *PackagesIterator) Pkg() *packages.Package {
	return it.cur
}

// Path returns a path that can be used to find the current package starting
// from the root of the iteration.
// For example if Path returns [a, b, c]
// Then 'a' is one of the packages passed to visit.Packages, a imports b, b
// imports c and, finally, c imports the current package.
func (it *PackagesIterator) Path() []*packages.Package {
	return it.stack
}

// SkipChildren will skip visiting the children of the current package.
func (it *PackagesIterator) SkipChildren() {
	fringe := it.fringeStack[len(it.fringeStack)-1]
	if fringe != nil && len(*fringe) != 0 {
		(*fringe) = nil
	}
}
