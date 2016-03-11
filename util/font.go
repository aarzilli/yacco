package util

import (
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	"image"
	"io/ioutil"
	"os"
	"strings"
)

type Font struct {
	fonts       []*truetype.Font
	cs          []*freetype.Context
	dpi         float64
	Size        float64
	lineextra   float64
	fullHinting bool
}

// Reads a Font: fontPath is a ':' separated list of ttf or pcf font files (they will be used to search characters)
func NewFont(dpi, size, lineSpacing float64, fullHinting bool, fontPath string) (*Font, error) {
	fontPathV := strings.Split(fontPath, ":")
	rf := &Font{make([]*truetype.Font, 0, len(fontPathV)), make([]*freetype.Context, len(fontPathV)), dpi, size, lineSpacing, fullHinting}
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

func MustNewFont(dpi, size, lineSpacing float64, fullHinting bool, fontPath string) *Font {
	r, err := NewFont(dpi, size, lineSpacing, fullHinting, fontPath)
	if err != nil {
		panic(err)
	}
	return r
}

func NewFontFromBytes(dpi, size, lineSpacing float64, fullHinting bool, fontBytes [][]byte) (*Font, error) {
	rf := &Font{make([]*truetype.Font, 0, len(fontBytes)), make([]*freetype.Context, len(fontBytes)), dpi, size, lineSpacing, fullHinting}
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
	return int32(f.Size + f.lineextra)
}

func (f *Font) LineHeightRaster() fixed.Int26_6 {
	return fixed.I(int(f.Size + f.lineextra))
}

func (f *Font) Bounds() fixed.Rectangle26_6 {
	return f.fonts[0].Bounds(FloatToFixed(f.Size))
}

func (f *Font) GlyphKerning(fontIdx uint16, pidx, idx truetype.Index) fixed.Int26_6 {
	return f.fonts[fontIdx].Kern(f.cs[fontIdx].Scale, pidx, idx)
}

func (f *Font) Glyph(fontIdx uint16, idx truetype.Index, p fixed.Point26_6) (width fixed.Int26_6, mask *image.Alpha, glyphRect image.Rectangle, err error) {
	var offset image.Point
	width, mask, offset, err = f.cs[fontIdx].Glyph(idx, p)
	if err != nil {
		return
	}
	glyphRect = mask.Bounds().Add(offset)
	return
}

func (f *Font) Index(x rune) (uint16, truetype.Index) {
	for fontIdx, font := range f.fonts {
		idx := font.Index(x)
		if idx != 0 {
			return uint16(fontIdx), idx
		}
	}
	return 0, 0
}
