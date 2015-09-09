package util

import (
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	"gopcf"
	"image"
	"io/ioutil"
	"os"
	"strings"
)

type Font struct {
	fonts       []*truetype.Font
	cs          []*freetype.Context
	pfonts      []*pcf.Pcf
	dpi         float64
	Size        float64
	spacing     float64
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
			f.cs[i].SetHinting(font.HintingFull)
		}
	}
}

func FloatToFixed(x float64) fixed.Int26_6 {
	n := int(x)
	frac := int(0x3f * (x - float64(n)))
	return fixed.Int26_6(n<<6 + frac)
}

func FixedToInt(x fixed.Int26_6) int {
	return int(x >> 6)
}

func (f *Font) LineHeight() int32 {
	if f.fonts[0] != nil {
		bounds := f.Bounds()
		return int32(float64(FixedToInt(bounds.Max.Y-bounds.Min.Y)) * f.spacing)
	} else {
		return int32(f.pfonts[0].LineAdvance() + 1) // <- is this wrong?
	}
}

/*
func (f *Font) SpacingFix(h int32) float64 {
	return math.Floor(float64(h) * f.spacing)
}*/

func (f *Font) Spacing() float64 {
	return f.spacing
}

func (f *Font) LineHeightRaster() fixed.Int26_6 {
	if f.fonts[0] != nil {
		bounds := f.Bounds()
		return fixed.I(int(float64(FixedToInt(bounds.Max.Y-bounds.Min.Y)) * f.spacing))
	} else {
		return fixed.I(f.pfonts[0].LineAdvance())
	}
}

func (f *Font) Bounds() fixed.Rectangle26_6 {
	if f.fonts[0] != nil {
		return f.fonts[0].Bounds(FloatToFixed(f.Size))
	} else {
		mb := f.pfonts[0].Accelerators.Maxbounds
		return fixed.Rectangle26_6{Min: fixed.Point26_6{fixed.I(0), -fixed.I(int(mb.CharacterDescent))}, Max: fixed.Point26_6{fixed.I(int(mb.CharacterWidth)), fixed.I(int(mb.CharacterAscent))}}
	}
}

/*
func (f *Font) GlyphWidth(fontIdx int, idx truetype.Index) raster.Fix32 {
	if f.fonts[fontIdx] != nil {
		return raster.Fix32(f.fonts[fontIdx].HMetric(f.cs[fontIdx].Scale, idx).AdvanceWidth) << 2
	} else {
		return raster.Fix32(f.pfonts[fontIdx].Advance(int(idx))) << 8
	}
}*/

func (f *Font) GlyphKerning(fontIdx int, pidx, idx truetype.Index) fixed.Int26_6 {
	if f.fonts[fontIdx] != nil {
		return f.fonts[fontIdx].Kern(f.cs[fontIdx].Scale, pidx, idx)
	} else {
		return 0
	}
}

func (f *Font) Glyph(fontIdx int, idx truetype.Index, p fixed.Point26_6) (width fixed.Int26_6, mask *image.Alpha, glyphRect image.Rectangle, err error) {
	var offset image.Point
	if f.fonts[fontIdx] != nil {
		width, mask, offset, err = f.cs[fontIdx].Glyph(idx, p)
		if err != nil {
			return
		}
	} else {
		width = fixed.I(f.pfonts[fontIdx].Advance(int(idx)))
		mask, offset = f.pfonts[fontIdx].Glyph(int(idx), image.Point{FixedToInt(p.X), FixedToInt(p.Y)})
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
