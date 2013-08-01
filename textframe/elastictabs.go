package textframe

import (
	"image"
	"yacco/config"
	"code.google.com/p/freetype-go/freetype/raster"
)

func (fr *Frame) elasticTabs(padding, tabWidth, bottom, leftMargin, rightMargin, lh raster.Fix32) (limit image.Point) {
	minTabWidth := tabWidth
	maxTabWidth := tabWidth * raster.Fix32(config.TabElasticity)
	
	fieldWidths := []raster.Fix32{}
	
	/* Calculating field widths */
	curField := 0
	fieldWidth := raster.Fix32(0)
	
	set := func() {
		if curField >= len(fieldWidths) {
			fieldWidths = append(fieldWidths, minTabWidth)
		}
		
		fieldWidth += padding

		if (fieldWidth < maxTabWidth) && (fieldWidth > fieldWidths[curField]) {
			fieldWidths[curField] = fieldWidth
		}
	}
	
	for _, g := range fr.glyphs {
		switch g.r {
		case '\t':
			set()
			fieldWidth = 0
			curField++
			
		case '\n':
			set()
			fieldWidth = 0
			curField = 0
		
		default:
			fieldWidth += g.kerning + g.width
		}
	}
	
	set()
	
	print("Widths:")
	for _, w := range fieldWidths {
		print(" ", w)
	}
	println()
		
	fr.ins = fr.initialInsPoint()
	
	/* Reflowing glyphs to respect new field widths */
	for _, g := range fr.glyphs {
		if fr.ins.Y < raster.Fix32(fr.R.Max.Y << 8) {
			//println("lastFull is: ", len(fr.glyphs), fr.ins.Y + lh, raster.Fix32(fr.R.Max.Y << 8))
			fr.lastFull = len(fr.glyphs)
		}

		switch g.r {
		case '\n':
			println("Newline")
			g.p = fr.ins
			g.width = raster.Fix32(fr.R.Max.X<<8) - fr.ins.X - fr.margin
			fr.ins.X = leftMargin
			fr.ins.Y += lh
			curField = 0
			
		case '\t':
			g.p = fr.ins
			ts := raster.Fix32(0)
			found := false
			println("Finding next tab for:", fr.ins.X + padding)
			for i := range fieldWidths {
				ts += fieldWidths[i]
				if ts > fr.ins.X + padding {
					println("Width change from", g.width, "to", ts-fr.ins.X)
					g.width = ts - fr.ins.X
					println("Advancing to", ts)
					fr.ins.X = ts
					found = true
					break
				}
			}
			if !found {
				println("Not found")
				g.width = tabWidth
				fr.ins.X += g.width
			}
			curField++
			
		default:
			fr.ins.X += g.kerning
			
			if fr.Hackflags & HF_TRUNCATE == 0 {
				if fr.ins.X + g.width > rightMargin {
					fr.ins.X = leftMargin
					fr.ins.Y += lh
				}
			}
			
			g.p = fr.ins
			fr.ins.X += g.width
		}
		
		if int(fr.ins.X >> 8) > limit.X {
			limit.X = int(fr.ins.X >> 8)
		}

		if int(fr.ins.Y >> 8) > limit.Y {
			limit.Y = int(fr.ins.Y >> 8)
		}
	}
	
	println("Done")
	
	if fr.ins.Y < raster.Fix32(fr.R.Max.Y << 8) {
		//println("lastFull is: ", len(fr.glyphs), fr.ins.Y + lh, raster.Fix32(fr.R.Max.Y << 8))
		fr.lastFull = len(fr.glyphs)
	}
	fr.Limit = limit
	return
}
