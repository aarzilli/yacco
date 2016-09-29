package otat

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

const debugLookupBuild = false
const debugProcess = false

type Index uint16

type Machine struct {
	dummy            bool
	cmap, gsub []byte
	cmapIndexes      []byte
	cm               []cm

	// Values from GDEF, GPOS and GSUB sections
	allookups                       []lookup
	allfeatures                     []feature
	allscripts                      []script

	// selected lookups
	prevlookup *lookup
	lookups    []*lookup

	// input window
	window, backtrack, lookahead []Index
	input                        *Index
	load                         int // number of true character in the input window, excluding backtrack characters

	// full input
	runes  []rune
	runeit func() (rune, bool)
}

const (
	// A 32-bit encoding consists of a most-significant 16-bit Platform ID and a
	// least-significant 16-bit Platform Specific ID. The magic numbers are
	// specified at https://www.microsoft.com/typography/otspec/name.htm
	unicodeEncoding         = 0x00000003 // PID = 0 (Unicode), PSID = 3 (Unicode 2.0)
	microsoftSymbolEncoding = 0x00030000 // PID = 3 (Microsoft), PSID = 0 (Symbol)
	microsoftUCS2Encoding   = 0x00030001 // PID = 3 (Microsoft), PSID = 1 (UCS-2)
	microsoftUCS4Encoding   = 0x0003000a // PID = 3 (Microsoft), PSID = 10 (UCS-4)
)

// A cm holds a parsed cmap entry.
type cm struct {
	start, end, delta, offset uint32
}

type script struct {
	tag      string
	features []uint16
}

type feature struct {
	tag     string
	lookups []uint16
}

type lookup struct {
	id        int
	typ, flag uint16
	tables    []lookupTable
}

type lookupTable struct {
	used bool // used at least once, for debugging

	cov coverage

	// single substitution (type 1)
	delta      int16
	substitute []Index

	// contextual substitution (type 6)
	backtrackCov, inputCov, lookaheadCov []coverage
	substSeqIdx                          []uint16
	substLookup                          []uint16
	slp                                  *lookupTable
}

type coverage struct {
	sparse      []Index
	rangeStart  []Index
	rangeEnd    []Index
	rangeCovIdx []uint16
}

type ligature struct {
	ligGlyph   Index
	components []Index
}

type classRange struct {
	Start uint16
	End   uint16
	Class uint16
}

const DefaultFeatures = "calt,case,zero"

// Script selects the default script to use.
// Features is a comma separated list of features to enable.
// If script is the empty string "DFLT", "dflt" and "latn" will
// be used.
// If features is the empty string the features specified in
// DefaultFeatures will be used, if no features should be enabled use
// "none"
func New(ttf []byte, script string, features string) (*Machine, []string, error) {
	m, err := parse(ttf, 0)
	if err != nil {
		return nil, nil, err
	}
	if m == nil {
		return Dummy(), nil, nil
	}

	if features == "" {
		features = DefaultFeatures
	}
	wantedFeatures := strings.Split(features, ",")

	var availableFeatures []uint16
scriptSearch:
	for _, tag := range []string{script, "DFLT", "dflt", "latn"} {
		for _, cur := range m.allscripts {
			if cur.tag == tag {
				availableFeatures = cur.features
				break scriptSearch
			}
		}
	}

	availableFeaturesStr := make([]string, 0, len(availableFeatures))

	lookupIdx := []int{}

	for _, i := range availableFeatures {
		availableFeaturesStr = append(availableFeaturesStr, m.allfeatures[i].tag)
		for _, wantedFeature := range wantedFeatures {
			if m.allfeatures[i].tag == wantedFeature {
				for _, idx := range m.allfeatures[i].lookups {
					lookupIdx = append(lookupIdx, int(idx))
				}
				break
			}
		}
	}

	sort.Ints(lookupIdx)

	src, dst := 1, 1
	for src < len(lookupIdx) {
		if lookupIdx[src] != lookupIdx[dst-1] {
			lookupIdx[dst] = lookupIdx[src]
			dst++
			src++
		} else {
			src++
		}
	}
	if dst < len(lookupIdx) {
		lookupIdx = lookupIdx[:dst]
	}

	m.lookups = make([]*lookup, 0, len(lookupIdx))

	maxbacktrack := 0
	maxlookahead := 1

	for _, idx := range lookupIdx {
		lookup := &m.allookups[idx]
		switch lookup.typ {
		case 1:
			m.lookups = append(m.lookups, lookup)
		case 6:
			ok := true
			for j := range lookup.tables {
				lpt := &lookup.tables[j]
				if len(lpt.inputCov) != 1 {
					if debugLookupBuild {
						fmt.Printf("discarding lpt contextual (input len)\n")
					}
					ok = false
					break
				}
				if len(lpt.substLookup) > 1 {
					if debugLookupBuild {
						fmt.Printf("discarding lpt contextual (subst len)\n")
					}
					ok = false
					break
				}
				if len(lpt.substLookup) == 1 {
					slp := &m.allookups[lpt.substLookup[0]]
					if len(slp.tables) != 1 || slp.typ != 1 {
						if debugLookupBuild {
							fmt.Printf("discarding lpt contextual (subst type/tables)")

						}
						ok = false
						break
					}
					lpt.slp = &slp.tables[0]

				}
			}

			if !ok {
				continue
			}

			for j := range lookup.tables {
				lpt := &lookup.tables[j]
				if len(lpt.backtrackCov) > maxbacktrack {
					maxbacktrack = len(lpt.backtrackCov)
				}
				if len(lpt.lookaheadCov) > maxlookahead {
					maxlookahead = len(lpt.lookaheadCov)
				}
			}
			m.lookups = append(m.lookups, lookup)

		default:
			if debugLookupBuild {
				fmt.Printf("discarding lookup (unsupported type %d)\n", lookup.typ)
			}
			//discarded
		}
	}

	// free space
	m.allookups = nil
	m.allfeatures = nil
	m.allscripts = nil

	m.window = make([]Index, maxbacktrack+maxlookahead+1)
	m.backtrack = m.window[:maxbacktrack]
	m.input = &m.window[maxbacktrack]
	m.lookahead = m.window[maxbacktrack+1:]

	if debugLookupBuild {
		for _, lookup := range m.lookups {
			fmt.Println(lookup)
		}
	}

	return m, availableFeaturesStr, err
}

func Dummy() *Machine {
	return &Machine{dummy: true}
}

func (lookup *lookup) String() string {
	out := ""
	for i := range lookup.tables {
		out += lookup.tables[i].String(lookup.id, lookup.typ)
	}
	return out
}

func (lp *lookupTable) String(id int, typ uint16) string {
	return lp.StringIndented(id, typ, "")
}

func (lp *lookupTable) StringIndented(id int, typ uint16, indent string) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s[%d] Lookup %d:\n", indent, id, typ)
	switch typ {
	case 1:
		it := lp.cov.Iterator()
		if lp.substitute == nil {
			fmt.Fprintf(&buf, "%s\t%s,delta %d\n", indent, lp.cov.Type(), lp.delta)
		} else {
			fmt.Fprintf(&buf, "%s\t%s,mapped\n", indent, lp.cov.Type())
		}
		for it.Next() {
			glyph := it.Glyph()
			i := it.Idx()
			var glyphout Index
			if lp.substitute == nil {
				glyphout = Index(int(glyph) + int(lp.delta))
			} else {
				glyphout = lp.substitute[i]
			}
			fmt.Fprintf(&buf, "%s\t%d → %d\n", indent, glyph, glyphout)
		}
	case 6:
		for i := len(lp.backtrackCov) - 1; i >= 0; i-- {
			fmt.Fprintf(&buf, "%s\tbacktrack %d: ", indent, i)
			it := lp.backtrackCov[i].Iterator()
			for it.Next() {
				glyph := it.Glyph()
				fmt.Fprintf(&buf, "%d, ", glyph)
			}
			fmt.Fprintf(&buf, "\n")
		}

		for i := 0; i < len(lp.inputCov); i++ {
			fmt.Fprintf(&buf, "%s\tinput %d: ", indent, i)
			it := lp.inputCov[i].Iterator()
			for it.Next() {
				glyph := it.Glyph()
				fmt.Fprintf(&buf, "%d, ", glyph)
			}
			fmt.Fprintf(&buf, "\n")
		}

		for i := 0; i < len(lp.lookaheadCov); i++ {
			fmt.Fprintf(&buf, "%s\tlookahead %d: ", indent, i)
			it := lp.lookaheadCov[i].Iterator()
			for it.Next() {
				glyph := it.Glyph()
				fmt.Fprintf(&buf, "%d, ", glyph)
			}
			fmt.Fprintf(&buf, "\n")
		}

		fmt.Fprintf(&buf, "%s\tsubstitutions %d\n", indent, len(lp.substSeqIdx))

		if lp.slp != nil && indent == "" {
			fmt.Fprintln(&buf, lp.slp.StringIndented(int(lp.substSeqIdx[0]), 1, "\t"))
		} else {
			for i := range lp.substSeqIdx {
				fmt.Fprintf(&buf, "%s\tsubst %d %d\n", indent, lp.substSeqIdx[i], lp.substLookup[i])
			}
		}
	default:
		fmt.Fprintf(&buf, "\t<unsupported>\n")
	}

	return buf.String()
}

type coverageIterator struct {
	cov       *coverage
	i         int
	nextGlyph Index
	nextIdx   uint16
}

func (cov *coverage) Type() string {
	if cov.sparse != nil {
		return "sparse"
	}
	return "ranges"
}

func (cov *coverage) Iterator() coverageIterator {
	return coverageIterator{cov, -1, 0, 0}
}

func (cov *coverage) Covers(in Index) int {
	if cov.sparse != nil {
		for idx, i := range cov.sparse {
			if i == in {
				return idx
			}
		}
		return -1
	}

	for i := range cov.rangeStart {
		if in >= cov.rangeStart[i] && in <= cov.rangeEnd[i] {
			return int(cov.rangeCovIdx[i]) + int(in-cov.rangeStart[i])
		}
	}
	return -1
}

func (it *coverageIterator) Next() bool {
	if it.cov.sparse != nil {
		it.i++
		if it.i >= len(it.cov.sparse) {
			return false
		}
		it.nextGlyph = it.cov.sparse[it.i]
		it.nextIdx = uint16(it.i)
		return true
	}

	if it.i < 0 {
		it.i++
		if it.i >= len(it.cov.rangeStart) {
			return false
		}
		it.nextGlyph = it.cov.rangeStart[it.i]
		it.nextIdx = it.cov.rangeCovIdx[it.i]
	} else {
		it.nextGlyph++
		it.nextIdx++
		if it.nextGlyph > it.cov.rangeEnd[it.i] {
			it.i++
			if it.i >= len(it.cov.rangeStart) {
				return false
			}
			it.nextGlyph = it.cov.rangeStart[it.i]
			it.nextIdx = it.cov.rangeCovIdx[it.i]
		}
	}
	return true
}

func (it *coverageIterator) Glyph() Index {
	return it.nextGlyph
}

func (it *coverageIterator) Idx() uint16 {
	return it.nextIdx
}
