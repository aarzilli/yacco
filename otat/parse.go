package otat

import (
	"fmt"
)

func parse(ttf []byte, offset int) (m *Machine, err error) {
	if len(ttf)-offset < 12 {
		err = FormatError("TTF data is too short")
		return
	}
	originalOffset := offset
	magic, offset := u32(ttf, offset), offset+4
	switch magic {
	case 0x00010000:
		// No-op.
	case 0x74746366: // "ttcf" as a big-endian uint32.
		if originalOffset != 0 {
			err = FormatError("recursive TTC")
			return
		}
		ttcVersion, offset := u32(ttf, offset), offset+4
		if ttcVersion != 0x00010000 {
			// TODO: support TTC version 2.0, once I have such a .ttc file to test with.
			err = FormatError("bad TTC version")
			return
		}
		numFonts, offset := int(u32(ttf, offset)), offset+4
		if numFonts <= 0 {
			err = FormatError("bad number of TTC fonts")
			return
		}
		if len(ttf[offset:])/4 < numFonts {
			err = FormatError("TTC offset table is too short")
			return
		}
		// TODO: provide an API to select which font in a TrueType collection to return,
		// not just the first one. This may require an API to parse a TTC's name tables,
		// so users of this package can select the font in a TTC by name.
		offset = int(u32(ttf, offset))
		if offset <= 0 || offset > len(ttf) {
			err = FormatError("bad TTC offset")
			return
		}
		return parse(ttf, offset)
	default:
		err = FormatError("bad TTF version")
		return
	}
	n, offset := int(u16(ttf, offset)), offset+2
	if len(ttf) < 16*n+12 {
		err = FormatError("TTF data is too short")
		return
	}
	m = new(Machine)
	// Assign the table slices.
	for i := 0; i < n; i++ {
		x := 16*i + 12
		switch string(ttf[x : x+4]) {
		case "cmap":
			m.cmap, err = readTable(ttf, ttf[x+8:x+16])
		case "GSUB":
			m.gsub, err = readTable(ttf, ttf[x+8:x+16])
		}
		if err != nil {
			return
		}
	}
	if m.cmap == nil || m.gsub == nil {
		return nil, nil
	}
	// Parse and sanity-check the TTF data.
	if err = m.parseCmap(); err != nil {
		return
	}
	if err = m.parseGsub(); err != nil {
		return
	}
	return
}

func (m *Machine) parseCmap() error {
	const (
		cmapFormat4         = 4
		cmapFormat12        = 12
		languageIndependent = 0
	)

	offset, _, err := parseSubtables(m.cmap, "cmap", 4, 8, nil)
	if err != nil {
		return err
	}
	offset = int(u32(m.cmap, offset+4))
	if offset <= 0 || offset > len(m.cmap) {
		return FormatError("bad cmap offset")
	}

	cmapFormat := u16(m.cmap, offset)
	switch cmapFormat {
	case cmapFormat4:
		language := u16(m.cmap, offset+4)
		if language != languageIndependent {
			return UnsupportedError(fmt.Sprintf("language: %d", language))
		}
		segCountX2 := int(u16(m.cmap, offset+6))
		if segCountX2%2 == 1 {
			return FormatError(fmt.Sprintf("bad segCountX2: %d", segCountX2))
		}
		segCount := segCountX2 / 2
		offset += 14
		m.cm = make([]cm, segCount)
		for i := 0; i < segCount; i++ {
			m.cm[i].end = uint32(u16(m.cmap, offset))
			offset += 2
		}
		offset += 2
		for i := 0; i < segCount; i++ {
			m.cm[i].start = uint32(u16(m.cmap, offset))
			offset += 2
		}
		for i := 0; i < segCount; i++ {
			m.cm[i].delta = uint32(u16(m.cmap, offset))
			offset += 2
		}
		for i := 0; i < segCount; i++ {
			m.cm[i].offset = uint32(u16(m.cmap, offset))
			offset += 2
		}
		m.cmapIndexes = m.cmap[offset:]
		return nil

	case cmapFormat12:
		if u16(m.cmap, offset+2) != 0 {
			return FormatError(fmt.Sprintf("cmap format: % x", m.cmap[offset:offset+4]))
		}
		length := u32(m.cmap, offset+4)
		language := u32(m.cmap, offset+8)
		if language != languageIndependent {
			return UnsupportedError(fmt.Sprintf("language: %d", language))
		}
		nGroups := u32(m.cmap, offset+12)
		if length != 12*nGroups+16 {
			return FormatError("inconsistent cmap length")
		}
		offset += 16
		m.cm = make([]cm, nGroups)
		for i := uint32(0); i < nGroups; i++ {
			m.cm[i].start = u32(m.cmap, offset+0)
			m.cm[i].end = u32(m.cmap, offset+4)
			m.cm[i].delta = u32(m.cmap, offset+8) - m.cm[i].start
			offset += 12
		}
		return nil
	}
	return UnsupportedError(fmt.Sprintf("cmap format: %d", cmapFormat))
}

// parseSubtables returns the offset and platformID of the best subtable in
// table, where best favors a Unicode cmap encoding, and failing that, a
// Microsoft cmap encoding. offset is the offset of the first subtable in
// table, and size is the size of each subtable.
//
// If pred is non-nil, then only subtables that satisfy that predicate will be
// considered.
func parseSubtables(table []byte, name string, offset, size int, pred func([]byte) bool) (
	bestOffset int, bestPID uint32, retErr error) {

	if len(table) < 4 {
		return 0, 0, FormatError(name + " too short")
	}
	nSubtables := int(u16(table, 2))
	if len(table) < size*nSubtables+offset {
		return 0, 0, FormatError(name + " too short")
	}
	ok := false
	for i := 0; i < nSubtables; i, offset = i+1, offset+size {
		if pred != nil && !pred(table[offset:]) {
			continue
		}
		// We read the 16-bit Platform ID and 16-bit Platform Specific ID as a single uint32.
		// All values are big-endian.
		pidPsid := u32(table, offset)
		// We prefer the Unicode cmap encoding. Failing to find that, we fall
		// back onto the Microsoft cmap encoding.
		if pidPsid == unicodeEncoding {
			bestOffset, bestPID, ok = offset, pidPsid>>16, true
			break

		} else if pidPsid == microsoftSymbolEncoding ||
			pidPsid == microsoftUCS2Encoding ||
			pidPsid == microsoftUCS4Encoding {

			bestOffset, bestPID, ok = offset, pidPsid>>16, true
			// We don't break out of the for loop, so that Unicode can override Microsoft.
		}
	}
	if !ok {
		return 0, 0, UnsupportedError(name + " encoding")
	}
	return bestOffset, bestPID, nil
}

func (m *Machine) parseGsub() error {
	scriptListOff := u16(m.gsub, 4)
	featureListOff := u16(m.gsub, 6)
	lookupListOff := u16(m.gsub, 8)

	m.allookups = parseLookupList(m.gsub[lookupListOff:])
	m.allfeatures = parseFeatureList(m.gsub[featureListOff:])
	m.allscripts = parseScriptList(m.gsub[scriptListOff:])

	return nil
}

func parseScriptList(list []byte) []script {
	count := u16(list, 0)
	r := make([]script, count)
	off := 2
	for i := range r {
		r[i].tag = string(list[off : off+4])
		scriptOff := u16(list, off+4)
		off += 6

		defaultLangSysOff := u16(list, int(scriptOff)) + scriptOff

		featcount := u16(list, int(defaultLangSysOff+4))
		r[i].features = make([]uint16, featcount)

		for j := range r[i].features {
			r[i].features[j] = u16(list, int(defaultLangSysOff+6+(uint16(j)*2)))
		}
	}
	return r
}

func parseFeatureList(list []byte) []feature {
	count := u16(list, 0)
	r := make([]feature, count)

	off := 2
	for i := range r {
		r[i].tag = string(list[off : off+4])
		featureOff := u16(list, off+4)

		off += 6

		lookupcount := u16(list, int(featureOff+2))
		r[i].lookups = make([]uint16, lookupcount)

		for j := range r[i].lookups {
			r[i].lookups[j] = u16(list, int(featureOff+4+(uint16(j)*2)))
		}
	}
	return r
}

func parseLookupList(list []byte) []lookup {
	count := u16(list, 0)
	r := make([]lookup, count)
	var subtables []uint16
	for i := range r {
		lookupOff := u16(list, 2+(i*2))

		r[i].id = i
		r[i].typ = u16(list, int(lookupOff))
		r[i].flag = u16(list, int(lookupOff+2))
		subtableCount := u16(list, int(lookupOff+4))

		subtables = subtables[:0]
		for j := 0; j < int(subtableCount); j++ {
			subtables = append(subtables, u16(list, int(lookupOff+6+(uint16(j)*2))))
		}

		switch r[i].typ {
		case 1: // single substitution
			r[i].tables = make([]lookupTable, len(subtables))
			for j := range subtables {
				r[i].tables[j].parseType1(list[lookupOff+subtables[j]:])
			}
		case 6: // chaining context substitution
			r[i].tables = make([]lookupTable, len(subtables))
			for j := range subtables {
				r[i].tables[j].parseType6(list[lookupOff+subtables[j]:])
			}
		default:
			// unsupported subtable
		}
	}
	return r
}

func (lp *lookupTable) parseType1(subtable []byte) {
	format := u16(subtable, 0)
	covOff := u16(subtable, 2)
	lp.cov = parseCoverage(subtable[covOff:])
	switch format {
	case 1:
		lp.delta = int16(u16(subtable, 4))
	case 2:
		count := u16(subtable, 4)
		lp.substitute = make([]Index, count)
		for i := range lp.substitute {
			lp.substitute[i] = Index(u16(subtable, 6+(i*2)))
		}
	default:
		// not defined
	}
}

func (lp *lookupTable) parseType6(subtable []byte) {
	format := u16(subtable, 0)
	if format != 3 {
		// only support format 3
		return
	}
	backtrackCount := u16(subtable, 2)
	lp.backtrackCov = make([]coverage, backtrackCount)
	off := 4
	for i := range lp.backtrackCov {
		covOff := u16(subtable, off)
		off += 2
		lp.backtrackCov[i] = parseCoverage(subtable[covOff:])
	}

	inputCount := u16(subtable, off)
	off += 2
	lp.inputCov = make([]coverage, inputCount)
	for i := range lp.inputCov {
		covOff := u16(subtable, off)
		off += 2
		lp.inputCov[i] = parseCoverage(subtable[covOff:])
	}
	lp.cov = lp.inputCov[0]

	lookaheadCount := u16(subtable, off)
	off += 2
	lp.lookaheadCov = make([]coverage, lookaheadCount)
	for i := range lp.lookaheadCov {
		covOff := u16(subtable, off)
		off += 2
		lp.lookaheadCov[i] = parseCoverage(subtable[covOff:])
	}

	substCount := u16(subtable, off)
	off += 2
	lp.substSeqIdx = make([]uint16, substCount)
	lp.substLookup = make([]uint16, substCount)
	for i := range lp.substSeqIdx {
		lp.substSeqIdx[i] = u16(subtable, off+0)
		lp.substLookup[i] = u16(subtable, off+2)
		off += 4
	}
}

func parseCoverage(table []byte) (r coverage) {
	format := u16(table, 0)
	count := u16(table, 2)
	switch format {
	case 1:
		r.sparse = make([]Index, count)
		for i := range r.sparse {
			r.sparse[i] = Index(u16(table, 4+(i*2)))
		}
	case 2:
		r.rangeStart = make([]Index, count)
		r.rangeEnd = make([]Index, count)
		r.rangeCovIdx = make([]uint16, count)
		off := 4
		for i := range r.rangeStart {
			r.rangeStart[i] = Index(u16(table, off+0))
			r.rangeEnd[i] = Index(u16(table, off+2))
			r.rangeCovIdx[i] = u16(table, off+4)
			off += 6
		}
	default:
		// not defined
	}
	return
}

func parseClassDef(table []byte) ([]classRange, error) {
	format := u16(table, 0)
	if format != 2 {
		return nil, UnsupportedError(fmt.Sprintf("unsupported class def format %d\n", format))
	}
	classRangeCount := u16(table, 2)

	r := make([]classRange, classRangeCount)

	off := 4
	for i := range r {
		r[i].Start = u16(table, off+0)
		r[i].End = u16(table, off+2)
		r[i].Class = u16(table, off+4)
		off += 6
	}

	return r, nil
}

// A FormatError reports that the input is not a valid TrueType font.
type FormatError string

func (e FormatError) Error() string {
	return "freetype: invalid TrueType format: " + string(e)
}

// An UnsupportedError reports that the input uses a valid but unimplemented
// TrueType feature.
type UnsupportedError string

func (e UnsupportedError) Error() string {
	return "freetype: unsupported TrueType feature: " + string(e)
}

// u32 returns the big-endian uint32 at b[i:].
func u32(b []byte, i int) uint32 {
	return uint32(b[i])<<24 | uint32(b[i+1])<<16 | uint32(b[i+2])<<8 | uint32(b[i+3])
}

// u16 returns the big-endian uint16 at b[i:].
func u16(b []byte, i int) uint16 {
	return uint16(b[i])<<8 | uint16(b[i+1])
}

// readTable returns a slice of the TTF data given by a table's directory entry.
func readTable(ttf []byte, offsetLength []byte) ([]byte, error) {
	offset := int(u32(offsetLength, 0))
	if offset < 0 {
		return nil, FormatError(fmt.Sprintf("offset too large: %d", uint32(offset)))
	}
	length := int(u32(offsetLength, 4))
	if length < 0 {
		return nil, FormatError(fmt.Sprintf("length too large: %d", uint32(length)))
	}
	end := offset + length
	if end < 0 || end > len(ttf) {
		return nil, FormatError(fmt.Sprintf("offset + length too large: %d", uint32(offset)+uint32(length)))
	}
	return ttf[offset:end], nil
}
