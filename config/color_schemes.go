package config

import (
	"image"
	"image/color"
)

type ColorScheme struct {
	WindowBG  image.Uniform
	TooltipBG image.Uniform

	TopBorder  image.Uniform
	VertBorder image.Uniform
	Scrollbar  image.Uniform

	EditorPlain []image.Uniform
	EditorSel1  []image.Uniform
	EditorSel2  []image.Uniform
	EditorSel3  []image.Uniform

	EditorMatchingParenthesis []image.Uniform

	Compl []image.Uniform

	TagPlain []image.Uniform
	TagSel1  []image.Uniform
	TagSel2  []image.Uniform
	TagSel3  []image.Uniform

	TagMatchingParenthesis []image.Uniform

	HandleFG         image.Uniform
	HandleModifiedFG image.Uniform
	HandleSpecialFG  image.Uniform
	HandleBG         image.Uniform
}

var ColorSchemeMap = map[string]*ColorScheme{}

func init() {
	ColorSchemeMap["standard"] = &AcmeColorScheme
	ColorSchemeMap["e"] = &AcmeEveningColorScheme
	ColorSchemeMap["evening"] = &AcmeEveningColorScheme
	ColorSchemeMap["e2"] = &AcmeEvening2ColorScheme
	ColorSchemeMap["evening2"] = &AcmeEvening2ColorScheme
	ColorSchemeMap["m"] = &AcmeMidnightColorScheme
	ColorSchemeMap["midnight"] = &AcmeMidnightColorScheme
	ColorSchemeMap["bw"] = &AcmeBWColorScheme
	ColorSchemeMap["zb"] = &ZenburnColorScheme
	ColorSchemeMap["atom"] = &AtomColorScheme
	ColorSchemeMap["tan"] = &TanColorScheme
	ColorSchemeMap["4"] = &C4ColorScheme
	ColorSchemeMap["4c"] = &C4CColorScheme
	ColorSchemeMap["4cr"] = &C4CRColorScheme
	ColorSchemeMap["g"] = &GruvboxColorScheme
}

func c(r, g, b uint8) image.Uniform {
	return *image.NewUniform(color.RGBA{r, g, b, 0xff})
}

func cc(x uint64) image.Uniform {
	return *image.NewUniform(color.RGBA{uint8((x >> 16) & 0xff), uint8((x >> 8) & 0xff), uint8(x & 0xff), 0xff})
}

var col2sel = c(0xAA, 0x00, 0x00)
var col3sel = c(0x00, 0x66, 0x00)
var bluebg = c(234, 0xff, 0xff)
var yellowbg = c(0xff, 0xff, 234)
var darkergreen = c(0x24, 0x49, 0x24)

func mix(color1 color.RGBA, color3 color.RGBA) image.Uniform {
	var color2 color.RGBA
	color2.R = uint8(float32(color3.R)*0.75) + uint8(float32(color1.R)*0.25)
	color2.G = uint8(float32(color3.G)*0.75) + uint8(float32(color1.G)*0.25)
	color2.B = uint8(float32(color3.B)*0.75) + uint8(float32(color1.B)*0.25)
	color2.A = 0xff
	return *image.NewUniform(color2)
}

var blahcol = c(0x78, 0x00, 0x3e)

var AcmeColorScheme = ColorScheme{
	WindowBG: *image.White,

	TopBorder: *image.Black, VertBorder: *image.Black,
	Scrollbar: *image.NewUniform(color.RGBA{153, 153, 76, 0xff}),

	EditorPlain: []image.Uniform{yellowbg, *image.Black, darkergreen, *DDarkblue},
	EditorSel1:  []image.Uniform{*DDarkyellow, *image.Black, darkergreen, *DDarkblue},
	EditorSel2:  []image.Uniform{col2sel, yellowbg},
	EditorSel3:  []image.Uniform{col3sel, yellowbg},

	EditorMatchingParenthesis: []image.Uniform{*image.Black, yellowbg},

	Compl: []image.Uniform{bluebg, *image.Black},

	TagPlain:               []image.Uniform{bluebg, *image.Black},
	TagSel1:                []image.Uniform{*DPalegreygreen, *image.Black},
	TagSel2:                []image.Uniform{col2sel, bluebg},
	TagSel3:                []image.Uniform{col3sel, bluebg},
	TagMatchingParenthesis: []image.Uniform{*image.Black, bluebg},

	HandleFG:         bluebg,
	HandleModifiedFG: *DMedblue,
	HandleSpecialFG:  *DMedgreen,
	HandleBG:         *DPurpleblue,
}

var yellowsilver = mix(color.RGBA{0xEE, 0xEE, 0x9E, 0xFF}, color.RGBA{0xcc, 0xcc, 0xcc, 0xff})
var redsilver = mix(color.RGBA{0xff, 0x00, 0x00, 0xff}, color.RGBA{0xcc, 0xcc, 0xcc, 0xff})
var greensilver = mix(color.RGBA{0x00, 0xff, 0x00, 0xff}, color.RGBA{0xcc, 0xcc, 0xcc, 0xff})
var stratostundora = mix(color.RGBA{0x00, 0x00, 0x44, 0xff}, color.RGBA{0x44, 0x44, 0x44, 0xff})
var stratostundora2 = mix(stratostundora.At(0, 0).(color.RGBA), color.RGBA{0x00, 0x00, 0x00, 0xff})

var AcmeEveningColorScheme = ColorScheme{
	WindowBG: *image.Black,

	TopBorder: *image.Black, VertBorder: *image.Black,
	Scrollbar: *image.NewUniform(color.RGBA{153, 153, 76, 0xff}),

	EditorPlain: []image.Uniform{*image.Black, *image.White, *DGreygreen, *DPalegreyblue},
	EditorSel1:  []image.Uniform{yellowsilver, *image.Black},
	EditorSel2:  []image.Uniform{redsilver, *image.Black},
	EditorSel3:  []image.Uniform{greensilver, *image.Black},

	EditorMatchingParenthesis: []image.Uniform{*image.White, *image.Black},

	TagPlain: []image.Uniform{stratostundora, *image.White},
	TagSel1:  []image.Uniform{*DPurpleblue, *image.Black},
	TagSel2:  []image.Uniform{*DPurpleblue, *image.Black},
	TagSel3:  []image.Uniform{*DPurpleblue, *image.Black},

	TagMatchingParenthesis: []image.Uniform{*image.White, stratostundora},

	Compl: []image.Uniform{stratostundora, *image.White},

	HandleFG:         stratostundora,
	HandleModifiedFG: *DMedblue,
	HandleSpecialFG:  *DMedgreen,
	HandleBG:         *DPurpleblue,
}

var AcmeEvening2ColorScheme = ColorScheme{
	WindowBG: stratostundora2,

	TopBorder: *image.Black, VertBorder: *image.Black,
	Scrollbar: *image.NewUniform(color.RGBA{153, 153, 76, 0xff}),

	EditorPlain: []image.Uniform{stratostundora2, *image.White, *DGreygreen, *DPalegreyblue},
	EditorSel1:  []image.Uniform{yellowsilver, *image.Black, *image.Black, *image.Black},
	EditorSel2:  []image.Uniform{redsilver, *image.Black},
	EditorSel3:  []image.Uniform{greensilver, *image.Black},

	EditorMatchingParenthesis: []image.Uniform{*image.White, *image.Black},

	TagPlain: []image.Uniform{stratostundora, *image.White},
	TagSel1:  []image.Uniform{*DPurpleblue, *image.Black},
	TagSel2:  []image.Uniform{*DPurpleblue, *image.Black},
	TagSel3:  []image.Uniform{*DPurpleblue, *image.Black},

	TagMatchingParenthesis: []image.Uniform{*image.White, stratostundora},

	Compl: []image.Uniform{stratostundora, *image.White},

	HandleFG:         stratostundora,
	HandleModifiedFG: *DMedblue,
	HandleSpecialFG:  *DMedgreen,
	HandleBG:         *DPurpleblue,
}

var darkyellowgray = mix(color.RGBA{0xaa, 0xff, 0x55, 0xff}, color.RGBA{0x66, 0x66, 0x66, 0xff})
var harlequin = image.NewUniform(color.RGBA{0x44, 0xcc, 0x00, 0xff})
var darkbluegray = mix(color.RGBA{0x00, 0x00, 0x55, 0xff}, color.RGBA{0x22, 0x22, 0x22, 0xff})

var AcmeMidnightColorScheme = ColorScheme{
	WindowBG: *image.Black,

	TopBorder: *harlequin, VertBorder: *harlequin,
	Scrollbar: darkyellowgray,

	EditorPlain: []image.Uniform{*image.Black, *harlequin, *DGreygreen, *DPalegreyblue},
	EditorSel1:  []image.Uniform{*DDarkyellow, *image.Black},
	EditorSel2:  []image.Uniform{*DRed, *image.White},
	EditorSel3:  []image.Uniform{*DGreen, *image.White},

	EditorMatchingParenthesis: []image.Uniform{*harlequin, *image.Black},

	TagPlain: []image.Uniform{darkbluegray, *harlequin},
	TagSel1:  []image.Uniform{*DPurpleblue, *image.Black},
	TagSel2:  []image.Uniform{*DPurpleblue, *image.Black},
	TagSel3:  []image.Uniform{*DPurpleblue, *image.Black},

	TagMatchingParenthesis: []image.Uniform{*harlequin, *DDarkblue},

	Compl: []image.Uniform{darkbluegray, *harlequin},

	HandleFG:         darkbluegray,
	HandleModifiedFG: *DPalegreyblue,
	HandleSpecialFG:  *DMedgreen,
	HandleBG:         *DPurpleblue,
}

var dustygray = image.NewUniform(color.RGBA{0x99, 0x99, 0x99, 0xff})

var AcmeBWColorScheme = ColorScheme{
	WindowBG: *image.White,

	TopBorder: *image.Black, VertBorder: *image.Black,
	Scrollbar: *image.NewUniform(color.RGBA{0xaa, 0xaa, 0xaa, 0xff}),

	EditorPlain: []image.Uniform{*image.White, *image.Black},
	EditorSel1:  []image.Uniform{*image.Black, *image.White},
	EditorSel2:  []image.Uniform{*DRed, *image.White},
	EditorSel3:  []image.Uniform{*dustygray, *image.White},

	EditorMatchingParenthesis: []image.Uniform{*image.Black, *image.White},

	TagPlain: []image.Uniform{*image.White, *image.Black},
	TagSel1:  []image.Uniform{*image.Black, *image.White},
	TagSel2:  []image.Uniform{*DRed, *image.White},
	TagSel3:  []image.Uniform{*dustygray, *image.White},

	TagMatchingParenthesis: []image.Uniform{*image.Black, *image.White},

	Compl: []image.Uniform{*image.White, *image.Black},

	HandleFG:         *image.White,
	HandleModifiedFG: *image.NewUniform(color.RGBA{0x33, 0x33, 0x33, 0xff}),
	HandleSpecialFG:  *DGreen,
	HandleBG:         *image.Black,
}

var zbbord = c(18, 16, 15)
var zbtagbg = c(0x15, 0x12, 0x10)
var zbtagfg = c(0x8a, 0x77, 0x6a)
var zbedbg = c(0x18, 0x15, 0x12)
var zbedfg = c(0xbe, 0xa4, 0x92)

var ZenburnColorScheme = ColorScheme{
	WindowBG: zbbord,

	TopBorder: zbbord, VertBorder: zbbord,
	Scrollbar: zbbord,

	EditorPlain: []image.Uniform{zbedbg, zbedfg, darkergreen, bluebg},
	EditorSel1:  []image.Uniform{*image.NewUniform(color.RGBA{0x8a, 0x77, 0x6a, 0xFF}), *image.NewUniform(color.RGBA{0x22, 0x22, 0x22, 0xFF}), darkergreen, *DDarkblue},
	EditorSel2:  []image.Uniform{redsilver, zbedbg},
	EditorSel3:  []image.Uniform{greensilver, zbedbg},

	EditorMatchingParenthesis: []image.Uniform{zbedfg, zbedbg},

	Compl: []image.Uniform{zbtagbg, zbtagfg},

	TagPlain: []image.Uniform{zbtagbg, zbtagfg},
	TagSel1:  []image.Uniform{*image.NewUniform(color.RGBA{0x8a, 0x77, 0x6a, 0xFF}), *image.NewUniform(color.RGBA{0x22, 0x22, 0x22, 0xFF})},
	TagSel2:  []image.Uniform{col2sel, bluebg},
	TagSel3:  []image.Uniform{col3sel, bluebg},

	TagMatchingParenthesis: []image.Uniform{zbtagfg, zbtagbg},

	HandleFG:         zbtagbg,
	HandleModifiedFG: *image.NewUniform(color.RGBA{0x9f, 0x71, 0x55, 0xFF}),
	HandleSpecialFG:  *DMedgreen,
	HandleBG:         zbbord,
}

var atombg = c(40, 44, 52)
var atomcmtfg = c(92, 99, 112)
var atomstrfg = c(152, 195, 121)
var atomnormfg = c(206, 209, 214)
var atomtagbg = c(0xaa, 0xaa, 0xaa)
var atomtagfg = c(0x00, 0x00, 0x00)
var atomwinbg = c(45, 45, 45)
var atomselbg = c(62, 68, 81)
var atomtagselbg = c(135, 135, 135)

var AtomColorScheme = ColorScheme{
	WindowBG: atomwinbg,

	TopBorder: atomtagfg, VertBorder: atomnormfg,
	Scrollbar: c(53, 59, 69),

	EditorPlain: []image.Uniform{atombg, atomnormfg, atomstrfg, atomcmtfg},
	EditorSel1:  []image.Uniform{atomselbg, atomnormfg, atomstrfg, atomcmtfg},
	EditorSel2:  []image.Uniform{atomselbg, atomnormfg},
	EditorSel3:  []image.Uniform{atomselbg, atomnormfg},

	EditorMatchingParenthesis: []image.Uniform{atomnormfg, atombg, atombg, atombg},

	Compl: []image.Uniform{atomtagbg, atomtagfg},

	TagPlain: []image.Uniform{atomtagbg, atomtagfg},
	TagSel1:  []image.Uniform{atomtagselbg, atomtagfg},
	TagSel2:  []image.Uniform{atomtagselbg, atomtagfg},
	TagSel3:  []image.Uniform{atomtagselbg, atomtagfg},

	TagMatchingParenthesis: []image.Uniform{atomtagselbg, atomtagfg},

	HandleFG:         atomtagbg,
	HandleModifiedFG: atomtagselbg,
	HandleSpecialFG:  *DMedgreen,
	HandleBG:         atomwinbg,
}

var tanbg = c(0xcb, 0x97, 0x62)
var tanscroll = c(0xe4, 0xc7, 0x78)
var tannormfg = c(0x29, 0x2a, 0x2d)
var tantagbg = c(0x38, 0x3b, 0x41)
var tantagfg = c(0xe4, 0xc7, 0x78)

var TanColorScheme = ColorScheme{
	WindowBG: tanbg,

	TopBorder: *image.Black, VertBorder: *image.Black,
	Scrollbar: tanscroll,

	EditorPlain: []image.Uniform{tanbg, tannormfg, darkergreen, *DDarkblue},
	EditorSel1:  []image.Uniform{tanscroll, tannormfg},
	EditorSel2:  []image.Uniform{tanscroll, tannormfg},
	EditorSel3:  []image.Uniform{tanscroll, tannormfg},

	EditorMatchingParenthesis: []image.Uniform{tanscroll, tannormfg},

	Compl: []image.Uniform{*image.White, *image.Black},

	TagPlain: []image.Uniform{tantagbg, tantagfg},
	TagSel1:  []image.Uniform{tantagfg, tantagbg},
	TagSel2:  []image.Uniform{tantagfg, tantagbg},
	TagSel3:  []image.Uniform{tantagfg, tantagbg},

	TagMatchingParenthesis: []image.Uniform{tantagfg, tantagbg},

	HandleFG:         tantagbg,
	HandleModifiedFG: c(224, 108, 107),
	HandleSpecialFG:  *DMedgreen,
	HandleBG:         c(0x72, 0x78, 0x80),
}

var c4bg = c(0x0a, 0x0d, 0x12)
var c4scroll = c(0x32, 0x5b, 0x65)
var c4normfg = c(0xb4, 0xb4, 0xb4)
var c4tagbg = c(0x32, 0x5b, 0x65)
var c4tagfg = c(0x0a, 0x0d, 0x12)
var c4comment = c(0x40, 0x97, 0x97)
var c4string = c(0x97, 0xa0, 0x6a)
var c4sel1 = c(0x1c, 0x83, 0x74)
var c4sel2 = c(0x83, 0x74, 0x1C)
var c4sel3 = c(0x83, 0x1C, 0x5E)

var C4ColorScheme = ColorScheme{
	WindowBG: c4bg,

	TopBorder: *image.Black, VertBorder: c4scroll,
	Scrollbar: c4scroll,

	EditorPlain: []image.Uniform{c4bg, c4normfg, c4string, c4comment},
	EditorSel1:  []image.Uniform{c4sel1, c4normfg, c4string, c4comment},
	EditorSel2:  []image.Uniform{c4sel2, c4normfg, c4string, c4comment},
	EditorSel3:  []image.Uniform{c4sel3, c4normfg, c4string, c4comment},

	EditorMatchingParenthesis: []image.Uniform{c4normfg, c4bg, c4bg, c4bg},

	Compl: []image.Uniform{c4normfg, c4bg},

	TagPlain: []image.Uniform{c4tagbg, c4tagfg},
	TagSel1:  []image.Uniform{c4tagfg, c4tagbg},
	TagSel2:  []image.Uniform{c4sel2, c4tagfg},
	TagSel3:  []image.Uniform{c4sel3, c4tagfg},

	TagMatchingParenthesis: []image.Uniform{c4tagfg, c4tagbg},

	HandleFG:         c4bg,
	HandleModifiedFG: c(0x42, 0x65, 0x32),
	HandleSpecialFG:  c(0xF5, 0x2B, 0x00),
	HandleBG:         c(0x72, 0x78, 0x80),
}

var c4cbg = cc(0xf0f0f0)
var c4cscroll = cc(0x9f9f9f)
var c4cnormfg = *image.Black
var c4ctagbg = cc(0x5f5f5f)
var c4ctagfg = cc(0xf0f0f0)
var c4ccomment = c(0x40, 0x97, 0x97)
var c4cstring = cc(0x6d583d)

//var c4csel1 = cc(0xb27606)
var c4csel1 = cc(0x0b44f9)
var c4csel1fg = cc(0x8dc1ff)
var c4csel2 = c(0x83, 0x74, 0x1C)
var c4csel3 = c(0x83, 0x1C, 0x5E)

var C4CColorScheme = ColorScheme{
	WindowBG: c4cbg,

	TopBorder: *image.Black, VertBorder: c4cscroll,
	Scrollbar: c4cscroll,

	EditorPlain: []image.Uniform{c4cbg, c4cnormfg, c4cstring, c4ccomment, c4csel1},
	EditorSel1:  []image.Uniform{c4csel1, c4csel1fg, c4csel1fg, c4ccomment},
	EditorSel2:  []image.Uniform{c4csel2, c4cbg, c4cbg, c4cbg},
	EditorSel3:  []image.Uniform{c4csel3, c4cbg, c4cbg, c4cbg},

	EditorMatchingParenthesis: []image.Uniform{c4cnormfg, c4cbg, c4cbg, c4cbg},

	Compl: []image.Uniform{c4cnormfg, c4cbg},

	TagPlain: []image.Uniform{c4ctagbg, c4ctagfg},
	TagSel1:  []image.Uniform{c4ctagfg, c4ctagbg},
	TagSel2:  []image.Uniform{c4csel2, c4ctagfg},
	TagSel3:  []image.Uniform{c4csel3, c4ctagfg},

	TagMatchingParenthesis: []image.Uniform{c4ctagfg, c4ctagbg},

	HandleFG:         c4ctagbg,
	HandleModifiedFG: c4cbg,
	HandleSpecialFG:  c(0xF5, 0x2B, 0x00),
	HandleBG:         c(0x72, 0x78, 0x80),
}

var c4crgb = cc(0xd6d6d6)
var c4crdarkbg = cc(0x606060)

var C4CRColorScheme = ColorScheme{
	WindowBG: cc(0xd6d6d6),

	TopBorder: *image.Black, VertBorder: c4crdarkbg,
	Scrollbar: c4crdarkbg,

	EditorPlain: []image.Uniform{cc(0xd6d6d6), *image.Black, cc(0x9a0000), cc(0x005800), c4csel1},
	EditorSel1:  []image.Uniform{cc(0x0044ff), c4crgb, c4crgb, c4crgb},
	EditorSel2:  []image.Uniform{cc(0x525252), c4crgb, c4crgb, c4crgb},
	EditorSel3:  []image.Uniform{cc(0x00b8b8), c4crgb, c4crgb, c4crgb},

	EditorMatchingParenthesis: []image.Uniform{*image.Black, c4crgb, c4crgb, c4crgb},

	Compl: []image.Uniform{c4cnormfg, c4crgb},

	TagPlain: []image.Uniform{c4crdarkbg, c4ctagfg},
	TagSel1:  []image.Uniform{c4ctagfg, c4crdarkbg},
	TagSel2:  []image.Uniform{cc(0x525252), c4ctagfg},
	TagSel3:  []image.Uniform{cc(0x00b8b8), c4ctagfg},

	TagMatchingParenthesis: []image.Uniform{c4ctagfg, c4crdarkbg},

	HandleFG:         c4crdarkbg,
	HandleModifiedFG: cc(0x0044ff),
	HandleSpecialFG:  c(0xF5, 0x2B, 0x00),
	HandleBG:         c(0x72, 0x78, 0x80),
}

var GruvboxColorScheme = ColorScheme{
	WindowBG: cc(0xfbf1c7),

	TopBorder: *image.Black, VertBorder: cc(0xa89984),
	Scrollbar: cc(0x928374),

	EditorPlain: []image.Uniform{cc(0xfbf1c7), cc(0x282828), cc(0xcc241d), cc(0x458588), cc(0xd654d0e)},
	EditorSel1:  []image.Uniform{cc(0x076678), cc(0xfbf1c7), cc(0xfbf1c7), cc(0xfbf1c7)},
	EditorSel2:  []image.Uniform{cc(0x79740e), cc(0xfbf1c7), cc(0xfbf1c7), cc(0xfbf1c7)},
	EditorSel3:  []image.Uniform{cc(0x9d0006), cc(0xfbf1c7), cc(0xfbf1c7), cc(0xfbf1c7)},

	EditorMatchingParenthesis: []image.Uniform{cc(0x282828), cc(0xfbf1c7), cc(0xfbf1c7), cc(0xfbf1c7)},

	Compl: []image.Uniform{cc(0x282828), cc(0xfbf1c7)},

	TagPlain: []image.Uniform{cc(0x427b58), cc(0xfbf1c7)},
	TagSel1:  []image.Uniform{cc(0x076678), cc(0xfbf1c7)},
	TagSel2:  []image.Uniform{cc(0x076678), cc(0xfbf1c7)},
	TagSel3:  []image.Uniform{cc(0x076678), cc(0xfbf1c7)},

	TagMatchingParenthesis: []image.Uniform{cc(0x076678), cc(0xfbf1c7)},

	HandleFG:         cc(0x427b58),
	HandleModifiedFG: cc(0xd79921),
	HandleSpecialFG:  cc(0xfabd2f),
	HandleBG:         cc(0xfbf1c7),
}
