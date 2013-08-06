package config

import (
	"image"
	"image/color"
)

type ColorScheme struct {
	WindowBG  image.Uniform
	TooltipBG image.Uniform

	Border    image.Uniform
	Scrollbar image.Uniform

	EditorPlain               []image.Uniform
	EditorSel1                []image.Uniform
	EditorSel2                []image.Uniform
	EditorSel3                []image.Uniform
	EditorMatchingParenthesis []image.Uniform

	Compl []image.Uniform

	TagPlain               []image.Uniform
	TagSel1                []image.Uniform
	TagSel2                []image.Uniform
	TagSel3                []image.Uniform
	TagMatchingParenthesis []image.Uniform

	HandleFG         image.Uniform
	HandleModifiedFG image.Uniform
	HandleSpecialFG  image.Uniform
	HandleBG         image.Uniform
}

var col2sel = *image.NewUniform(color.RGBA{0xAA, 0x00, 0x00, 0xFF})
var col3sel = *image.NewUniform(color.RGBA{0x00, 0x66, 0x00, 0xFF})
var bluebg = *image.NewUniform(color.RGBA{234, 0xff, 0xff, 0xff})
var yellowbg = *image.NewUniform(color.RGBA{0xff, 0xff, 234, 0xff})
var darkergreen = *image.NewUniform(color.RGBA{0x24, 0x49, 0x24, 0xff})

func mix(color1 color.RGBA, color3 color.RGBA) image.Uniform {
	var color2 color.RGBA
	color2.R = uint8(float32(color3.R)*0.75) + uint8(float32(color1.R)*0.25)
	color2.G = uint8(float32(color3.G)*0.75) + uint8(float32(color1.G)*0.25)
	color2.B = uint8(float32(color3.B)*0.75) + uint8(float32(color1.B)*0.25)
	color2.A = 0xff
	return *image.NewUniform(color2)
}

var AcmeColorScheme = ColorScheme{
	WindowBG: *image.White,

	Border:    *image.Black,
	Scrollbar: *image.NewUniform(color.RGBA{153, 153, 76, 0xff}),

	EditorPlain:               []image.Uniform{yellowbg, *image.Black, darkergreen, *DDarkblue},
	EditorSel1:                []image.Uniform{*DDarkyellow, *image.Black, darkergreen, *DDarkblue},
	EditorSel2:                []image.Uniform{col2sel, yellowbg, yellowbg, yellowbg},
	EditorSel3:                []image.Uniform{col3sel, yellowbg, yellowbg, yellowbg},
	EditorMatchingParenthesis: []image.Uniform{*image.Black, yellowbg, yellowbg, yellowbg},
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

var AcmeEveningColorScheme = ColorScheme{
	WindowBG: *image.Black,

	Border:    *image.White,
	Scrollbar: *image.NewUniform(color.RGBA{153, 153, 76, 0xff}),

	EditorPlain:               []image.Uniform{*image.Black, *image.White, *DGreygreen, *DPalegreyblue},
	EditorSel1:                []image.Uniform{yellowsilver, *image.Black, *image.Black, *image.Black},
	EditorSel2:                []image.Uniform{redsilver, *image.Black, *image.Black, *image.Black},
	EditorSel3:                []image.Uniform{greensilver, *image.Black, *image.Black, *image.Black},
	EditorMatchingParenthesis: []image.Uniform{*image.White, *image.Black, *image.Black, *image.Black},

	TagPlain:               []image.Uniform{stratostundora, *image.White},
	TagSel1:                []image.Uniform{*DPurpleblue, *image.Black},
	TagSel2:                []image.Uniform{*DPurpleblue, *image.Black},
	TagSel3:                []image.Uniform{*DPurpleblue, *image.Black},
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

	Border:    *harlequin,
	Scrollbar: darkyellowgray,

	EditorPlain:               []image.Uniform{*image.Black, *harlequin, *DGreygreen, *DPalegreyblue},
	EditorSel1:                []image.Uniform{*DDarkyellow, *image.Black, *image.Black, *image.Black},
	EditorSel2:                []image.Uniform{*DRed, *image.White, *image.White, *image.White},
	EditorSel3:                []image.Uniform{*DGreen, *image.White, *image.White, *image.White},
	EditorMatchingParenthesis: []image.Uniform{*harlequin, *image.Black, *image.Black, *image.Black},

	TagPlain:               []image.Uniform{darkbluegray, *harlequin},
	TagSel1:                []image.Uniform{*DPurpleblue, *image.Black},
	TagSel2:                []image.Uniform{*DPurpleblue, *image.Black},
	TagSel3:                []image.Uniform{*DPurpleblue, *image.Black},
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

	Border:    *image.Black,
	Scrollbar: *image.NewUniform(color.RGBA{0xaa, 0xaa, 0xaa, 0xff}),

	EditorPlain:               []image.Uniform{*image.White, *image.Black, *image.Black, *image.Black},
	EditorSel1:                []image.Uniform{*image.Black, *image.White, *image.White, *image.White},
	EditorSel2:                []image.Uniform{*DRed, *image.White, *image.White, *image.White},
	EditorSel3:                []image.Uniform{*dustygray, *image.White, *image.White, *image.White},
	EditorMatchingParenthesis: []image.Uniform{*image.Black, *image.White, *image.White, *image.White},

	TagPlain:               []image.Uniform{*image.White, *image.Black},
	TagSel1:                []image.Uniform{*image.Black, *image.White},
	TagSel2:                []image.Uniform{*DRed, *image.White},
	TagSel3:                []image.Uniform{*dustygray, *image.White},
	TagMatchingParenthesis: []image.Uniform{*image.Black, *image.White},

	Compl: []image.Uniform{*image.White, *image.Black},

	HandleFG:         *image.White,
	HandleModifiedFG: *image.NewUniform(color.RGBA{0x33, 0x33, 0x33, 0xff}),
	HandleSpecialFG:  *DGreen,
	HandleBG:         *image.Black,
}
