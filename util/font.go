package util

import (
	"code.google.com/p/freetype-go/freetype"
	"code.google.com/p/freetype-go/freetype/raster"
	"code.google.com/p/freetype-go/freetype/truetype"
	"gopcf"
	"image"
	"io/ioutil"
	"math"
	"os"
	"strings"
)

type Font struct {
	fonts   []*truetype.Font
	cs      []*freetype.Context
	pfonts  []*pcf.Pcf
	dpi     float64
	Size    float64
	spacing float64
	fullHinting bool
}

// Reads a Font: fontPath is a ':' separated list of ttf or pcf font files (they will be used to search characters)
func NewFont(dpi, size, lineSpacing float64, fullHinting bool, fontPath string) (*Font, error) {
	fontPathV := strings.Split(fontPath, ":")
	rf := &Font{make([]*truetype.Font, 0, len(fontPathV)), make([]*freetype.Context, len(fontPathV)), make([]*pcf.Pcf, 0, len(fontPathV)), dpi, size, lineSpacing, fullHinting}
	for _, fontfile := range fontPathV {
		if strings.HasSuffix(fontfile, ".ttf") {
			fontBytes, err := ioutil.ReadFile(os.ExpandEnv(fontfile))
			if err != nil {
				return nil, err
			}
			parsedfont, err := freetype.ParseFont(fontBytes)
			if err != nil {
				return nil, err
			}
			rf.fonts = append(rf.fonts, parsedfont)
			rf.pfonts = append(rf.pfonts, nil)
		} else {
			font, err := pcf.ReadPath(os.ExpandEnv(fontfile))
			if err != nil {
				println("check")
				return nil, err
			}
			rf.fonts = append(rf.fonts, nil)
			rf.pfonts = append(rf.pfonts, font)
		}
	}
	rf.createContexts()
	return rf, nil
}

func MustNewFont(dpi, size, lineSpacing float64, fullHinting bool, fontPath string) *Font {
	r, err := NewFont(dpi, size, lineSpacing, fullHinting, fontPath)
	if err != nil {
		panic(err)
	}
	return r
}

func NewFontFromBytes(dpi, size, lineSpacing float64, fullHinting bool, fontBytes [][]byte) (*Font, error) {
	rf := &Font{make([]*truetype.Font, 0, len(fontBytes)), make([]*freetype.Context, len(fontBytes)), make([]*pcf.Pcf, 0, len(fontBytes)), dpi, size, lineSpacing, fullHinting}
	for _, aFontBytes := range fontBytes {
		parsedfont, err := freetype.ParseFont(aFontBytes)
		if err != nil {
			return nil, err
		}
		rf.fonts = append(rf.fonts, parsedfont)
		rf.pfonts = append(rf.pfonts, nil)
	}
	rf.createContexts()
	return rf, nil
}

func MustNewFontFromBytes(dpi, size, lineSpacing float64, fullHinting bool, fontBytes [][]byte) *Font {
	f, err := NewFontFromBytes(dpi, size, lineSpacing, fullHinting, fontBytes)
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
		if f.fullHinting {
			f.cs[i].SetHinting(freetype.FullHinting)
		}
	}
}

func (f *Font) LineHeight() int32 {
	if f.fonts[0] != nil {
		bounds := f.fonts[0].Bounds(int32(f.Size))
		return bounds.YMax - bounds.YMin
	} else {
		return int32(f.pfonts[0].LineAdvance() + 1)
	}
}

func (f *Font) SpacingFix(h int32) float64 {
	return math.Floor(float64(h) * f.spacing)
}

func (f *Font) LineHeightRaster() raster.Fix32 {
	if f.fonts[0] != nil {
		return f.cs[0].PointToFix32(f.SpacingFix(f.LineHeight()))
	} else {
		return raster.Fix32(f.pfonts[0].LineAdvance() << 8)
	}
}

func (f *Font) Bounds() truetype.Bounds {
	if f.fonts[0] != nil {
		return f.fonts[0].Bounds(int32(f.Size))
	} else {
		mb := f.pfonts[0].Accelerators.Maxbounds
		return truetype.Bounds{0, -int32(mb.CharacterDescent), int32(mb.CharacterWidth), int32(mb.CharacterAscent)}
	}
}

func (f *Font) GlyphWidth(fontIdx int, idx truetype.Index) raster.Fix32 {
	if f.fonts[fontIdx] != nil {
		return raster.Fix32(f.fonts[fontIdx].HMetric(f.cs[fontIdx].Scale, idx).AdvanceWidth) << 2
	} else {
		return raster.Fix32(f.pfonts[fontIdx].Advance(int(idx))) << 8
	}
}

func (f *Font) GlyphKerning(fontIdx int, pidx, idx truetype.Index) raster.Fix32 {
	if f.fonts[fontIdx] != nil {
		return raster.Fix32(f.fonts[fontIdx].Kerning(f.cs[fontIdx].Scale, pidx, idx)) << 2
	} else {
		return 0
	}
}

func (f *Font) Glyph(fontIdx int, idx truetype.Index, p raster.Point) (mask *image.Alpha, glyphRect image.Rectangle, err error) {
	var offset image.Point
	if f.fonts[fontIdx] != nil {
		_, mask, offset, err = f.cs[fontIdx].Glyph(idx, p)
		if err != nil {
			return
		}
	} else {
		mask, offset = f.pfonts[fontIdx].Glyph(int(idx), image.Point{int(p.X >> 8), int(p.Y >> 8)})
	}
	glyphRect = mask.Bounds().Add(offset)
	return
}

func (f *Font) Index(x rune) (fontIdx int, idx truetype.Index) {
	var font *truetype.Font
	for fontIdx, font = range f.fonts {
		if font != nil {
			idx = font.Index(x)
		} else {
			idx = truetype.Index(f.pfonts[fontIdx].GetIndex(x))
		}
		if idx != 0 {
			return
		}
	}
	fontIdx = 0
	idx = 0
	return
}
