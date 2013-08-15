package textframe

import (
	"code.google.com/p/freetype-go/freetype/raster"
	"image"
	"yacco/config"
)

func (fr *Frame) elasticTabs(padding, tabWidth, bottom, leftMargin, rightMargin, lh raster.Fix32) (limit image.Point) {
	minTabWidth := tabWidth
	maxTabWidth := tabWidth * raster.Fix32(config.TabElasticity)

	fieldWidths := [][]raster.Fix32{}

	/* Calculating field widths */
	curField := 0
	curIndent := 0
	bol := true
	fieldWidth := raster.Fix32(0)
	invln := false

	set := func() {
		for curIndent >= len(fieldWidths) {
			fieldWidths = append(fieldWidths, make([]raster.Fix32, 0))
		}

		if curField >= len(fieldWidths[curIndent]) {
			fieldWidths[curIndent] = append(fieldWidths[curIndent], minTabWidth)
		}

		fieldWidth += padding

		if fieldWidth > maxTabWidth {
			invln = true
		}

		if !invln && (fieldWidth > fieldWidths[curIndent][curField]) {
			fieldWidths[curIndent][curField] = fieldWidth
		}
	}

	for _, g := range fr.glyphs {
		switch g.r {
		case '\t':
			if bol {
				curIndent++
				fieldWidth = 0
			} else {
				set()
				fieldWidth = 0
				curField++
			}

		case '\n':
			set()
			fieldWidth = 0
			curField = 0
			curIndent = 0
			bol = true
			invln = false

		default:
			bol = false
			fieldWidth += g.kerning + g.width
		}
	}

	set()

	colwrap := -1

	if (len(fieldWidths) == 1) && (fr.Hackflags&HF_COLUMNIZE != 0) {
		var n int

		for n = len(fieldWidths[0]); n > 0; n-- {
			newFieldWidth := make([]raster.Fix32, n)

			for i := range newFieldWidth {
				newFieldWidth[i] = 0
			}

			for i := 0; i < len(fieldWidths[0]); i++ {
				j := i % n
				if fieldWidths[0][i] > newFieldWidth[j] {
					newFieldWidth[j] = fieldWidths[0][i]
				}
			}

			totalWidth := leftMargin
			for i := range newFieldWidth {
				totalWidth += newFieldWidth[i]
			}

			if totalWidth <= rightMargin {
				colwrap = n
				fieldWidths[0] = newFieldWidth
				break
			}
		}
	}

	fr.ins = fr.initialInsPoint()

	/* Reflowing glyphs to respect new field widths */
	bol = true
	curIndent = 0
	curField = 0
	margin := leftMargin
	for i := range fr.glyphs {
		g := &fr.glyphs[i]

		switch g.r {
		case '\n':
			g.p = fr.ins
			g.width = raster.Fix32(fr.R.Max.X<<8) - fr.ins.X - fr.margin
			fr.ins.X = leftMargin
			fr.ins.Y += lh
			curField = 0
			curIndent = 0
			bol = true
			margin = leftMargin

		case '\t':
			//TODO: if columnizing replace appropriate '\t' with newline behaviour
			g.p = fr.ins
			if bol {
				curIndent++
				fr.ins.X += g.width
				margin = fr.ins.X
				break
			}
			if curIndent >= len(fieldWidths) {
				curField++
				fr.ins.X += g.width
				break
			}

			ts := margin
			found := false
			for i := range fieldWidths[curIndent] {
				ts += fieldWidths[curIndent][i]
				if ts >= fr.ins.X+padding {
					g.width = ts - fr.ins.X
					fr.ins.X = ts
					found = true
					curField = i
					break
				}
			}

			if (colwrap > 0) && (curField+1 >= colwrap) {
				g.p = fr.ins
				g.width = raster.Fix32(fr.R.Max.X<<8) - fr.ins.X - fr.margin
				fr.ins.X = leftMargin
				fr.ins.Y += lh
				curField = 0
				curIndent = 0
				bol = true
				margin = leftMargin
				break
			}

			if !found {
				toNextCell := tabWidth - ((fr.ins.X - leftMargin) % tabWidth)
				if toNextCell <= padding/2 {
					toNextCell += tabWidth
				}

				g.width = toNextCell
				fr.ins.X += g.width
			}

		default:
			fr.ins.X += g.kerning

			if fr.Hackflags&HF_TRUNCATE == 0 {
				if fr.ins.X+g.width > rightMargin {
					fr.ins.X = leftMargin
					fr.ins.Y += lh
				}
			}

			g.p = fr.ins
			fr.ins.X += g.width
			bol = false
		}

		if int(fr.ins.X>>8) > limit.X {
			limit.X = int(fr.ins.X >> 8)
		}

		if int(fr.ins.Y>>8) > limit.Y {
			limit.Y = int(fr.ins.Y >> 8)
		}
	}

	fr.Limit = limit
	return
}
