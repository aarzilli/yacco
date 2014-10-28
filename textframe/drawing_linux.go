// +build linux

package textframe

import (
	"github.com/skelterjohn/go.wde/xgb"
	"image"
	"image/draw"
)

func GetOptimizedDrawing(img draw.Image) DrawingFuncs {
	_, ok := img.(*xgb.Image)
	if !ok {
		return defaultDrawingFuncs
	} else {
		return xgbDrawingFuncs
	}
}

const m = 1<<16 - 1

var xgbDrawingFuncs = DrawingFuncs{
	DrawFillSrc: func(gdst draw.Image, r image.Rectangle, src *image.Uniform) {
		if r.Empty() {
			return
		}
		dst := gdst.(*xgb.Image)
		sr, sg, sb, sa := src.RGBA()
		// The built-in copy function is faster than a straightforward for loop to fill the destination with
		// the color, but copy requires a slice source. We therefore use a for loop to fill the first row, and
		// then use the first row as the slice source for the remaining rows.
		i0 := dst.PixOffset(r.Min.X, r.Min.Y)
		i1 := i0 + r.Dx()*4
		for i := i0; i < i1; i += 4 {
			dst.Pix[i+0] = uint8(sb >> 8)
			dst.Pix[i+1] = uint8(sg >> 8)
			dst.Pix[i+2] = uint8(sr >> 8)
			dst.Pix[i+3] = uint8(sa >> 8)
		}
		firstRow := dst.Pix[i0:i1]
		for y := r.Min.Y + 1; y < r.Max.Y; y++ {
			i0 += dst.Stride
			i1 += dst.Stride
			copy(dst.Pix[i0:i1], firstRow)
		}
	},

	DrawCopy: func(gdst draw.Image, r image.Rectangle, gsrc draw.Image, sp image.Point) {
		if r.Empty() {
			return
		}
		dst := gdst.(*xgb.Image)
		src := gsrc.(*xgb.Image)

		// Copyied from drawCopySrc
		n, dy := 4*r.Dx(), r.Dy()
		d0 := dst.PixOffset(r.Min.X, r.Min.Y)
		s0 := src.PixOffset(sp.X, sp.Y)
		var ddelta, sdelta int
		if r.Min.Y <= sp.Y {
			ddelta = dst.Stride
			sdelta = src.Stride
		} else {
			// If the source start point is higher than the destination start point, then we compose the rows
			// in bottom-up order instead of top-down. Unlike the drawCopyOver function, we don't have to
			// check the x co-ordinates because the built-in copy function can handle overlapping slices.
			d0 += (dy - 1) * dst.Stride
			s0 += (dy - 1) * src.Stride
			ddelta = -dst.Stride
			sdelta = -src.Stride
		}
		for ; dy > 0; dy-- {
			copy(dst.Pix[d0:d0+n], src.Pix[s0:s0+n])
			d0 += ddelta
			s0 += sdelta
		}
	},

	DrawGlyphOver: func(b draw.Image, r image.Rectangle, src *image.Uniform, mask *image.Alpha, mp image.Point) {
		if r.Empty() {
			return
		}
		dst := b.(*xgb.Image)
		i0 := dst.PixOffset(r.Min.X, r.Min.Y)
		i1 := i0 + r.Dx()*4
		mi0 := mask.PixOffset(mp.X, mp.Y)
		sr, sg, sb, sa := src.RGBA()
		for y, my := r.Min.Y, mp.Y; y != r.Max.Y; y, my = y+1, my+1 {
			for i, mi := i0, mi0; i < i1; i, mi = i+4, mi+1 {
				ma := uint32(mask.Pix[mi])
				if ma == 0 {
					continue
				}
				ma |= ma << 8

				db := uint32(dst.Pix[i+0])
				dg := uint32(dst.Pix[i+1])
				dr := uint32(dst.Pix[i+2])
				da := uint32(dst.Pix[i+3])

				// The 0x101 is here for the same reason as in drawRGBA.
				a := (m - (sa * ma / m)) * 0x101

				dst.Pix[i+0] = uint8((db*a + sb*ma) / m >> 8)
				dst.Pix[i+1] = uint8((dg*a + sg*ma) / m >> 8)
				dst.Pix[i+2] = uint8((dr*a + sr*ma) / m >> 8)
				dst.Pix[i+3] = uint8((da*a + sa*ma) / m >> 8)
			}
			i0 += dst.Stride
			i1 += dst.Stride
			mi0 += mask.Stride
		}
	},
}
