package util

import (
	"code.google.com/p/freetype-go/freetype"
	"code.google.com/p/freetype-go/freetype/raster"
	"code.google.com/p/freetype-go/freetype/truetype"
	"io/ioutil"
	"image"
	"os"
	"strings"
)

type Font struct {
	fonts   []*truetype.Font
	cs       []*freetype.Context
	dpi     float64
	Size    float64
	Spacing float64
}

// Reads a Font: fontPath is a ':' separated list of ttf font files (they will be used to search characters)
func NewFont(dpi, size, lineSpacing float64, fontPath string) (*Font, error) {
	fontPathV := strings.Split(fontPath, ":")
	rf := &Font{make([]*truetype.Font, 0, len(fontPathV)), make([]*freetype.Context, len(fontPathV)), dpi, size, lineSpacing}
	for _, fontfile := range fontPathV {
		fontBytes, err := ioutil.ReadFile(os.ExpandEnv(fontfile))
		if err != nil {
			return nil, err
		}
		parsedfont, err := freetype.ParseFont(fontBytes)
		if err != nil {
			return nil, err
		}
		rf.fonts = append(rf.fonts, parsedfont)
	}
	rf.createContexts()
	return rf, nil
}

func MustNewFont(dpi, size, lineSpacing float64, fontPath string) *Font {
	r, err := NewFont(dpi, size, lineSpacing, fontPath)
	if err != nil {
		panic(err)
	}
	return r
}

func NewFontFromBytes(dpi, size, lineSpacing float64, fontBytes [][]byte) (*Font, error) {
	rf := &Font{make([]*truetype.Font, 0, len(fontBytes)), make([]*freetype.Context, len(fontBytes)), dpi, size, lineSpacing}
	for _, aFontBytes := range fontBytes {
		parsedfont, err := freetype.ParseFont(aFontBytes)
		if err != nil {
			return nil, err
		}
		rf.fonts = append(rf.fonts, parsedfont)
	}
	rf.createContexts()
	return rf, nil
}

func MustNewFontFromBytes(dpi, size, lineSpacing float64, fontBytes [][]byte) *Font {
	f, err := NewFontFromBytes(dpi, size, lineSpacing, fontBytes)
	if err != nil {
		panic(err)
	}
	return f
}

func (f *Font) createContexts() {
	for i, _ := range f.fonts {
		f.cs[i] = freetype.NewContext()
		f.cs[i].SetDPI(f.dpi)
		f.cs[i].SetFont(f.fonts[i])
		f.cs[i].SetFontSize(f.Size)
	}
}

func (f *Font) LineHeight() int32 {
	bounds := f.fonts[0].Bounds(int32(f.Size))
	return bounds.YMax - bounds.YMin
}

func (f *Font) LineHeightRaster() raster.Fix32 {
	return f.cs[0].PointToFix32(float64(f.LineHeight()) * f.Spacing)
}

func (f *Font) Bounds() truetype.Bounds {
	return f.fonts[0].Bounds(int32(f.Size))
}

func (f *Font) GlyphWidth(fontIdx int, idx truetype.Index) raster.Fix32 {
	return raster.Fix32(f.fonts[fontIdx].HMetric(f.cs[fontIdx].Scale, idx).AdvanceWidth) << 2
}

func (f *Font) GlyphKerning(fontIdx int, pidx, idx truetype.Index) raster.Fix32 {
	return raster.Fix32(f.fonts[fontIdx].Kerning(f.cs[fontIdx].Scale, pidx, idx)) << 2
}

func (f *Font) Glyph(fontIdx int, idx truetype.Index, p raster.Point) (mask *image.Alpha, glyphRect image.Rectangle, err error) {
	mask, offset, err := f.cs[fontIdx].Glyph(idx, p)
	if err != nil {	
		return
	}
	glyphRect = mask.Bounds().Add(offset)
	return
}

func (f *Font) Index(x rune) (fontIdx int, idx truetype.Index) {
	var font *truetype.Font
	for fontIdx, font = range f.fonts {
		idx = font.Index(x)
		if idx != 0 {
			return
		}
	}
	fontIdx = 0
	idx = 0
	return
}
