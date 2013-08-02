package textframe

import (
	"image"
	"image/draw"
)

type DrawingFuncs struct {
	DrawFillSrc   func(draw.Image, image.Rectangle, *image.Uniform)
	DrawGlyphOver func(draw.Image, image.Rectangle, *image.Uniform, *image.Alpha, image.Point)
}

var defaultDrawingFuncs = DrawingFuncs{
	DrawFillSrc: func(b draw.Image, r image.Rectangle, color *image.Uniform) {
		if r.Empty() {
			return
		}
		draw.Draw(b, r, color, image.ZP, draw.Over)
	},

	DrawGlyphOver: func(b draw.Image, r image.Rectangle, color *image.Uniform, mask *image.Alpha, mp image.Point) {
		if r.Empty() {
			return
		}
		draw.DrawMask(b, r, color, image.ZP, mask, mp, draw.Over)
	},
}
