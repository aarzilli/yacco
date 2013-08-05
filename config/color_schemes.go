package config

import (
	"image"
	"image/color"
)

type ColorScheme struct {
	WindowBG image.Uniform
	TooltipBG image.Uniform

	Border image.Uniform
	Scrollbar image.Uniform

	EditorPlain []image.Uniform
	EditorSel1 []image.Uniform
	EditorSel2 []image.Uniform
	EditorSel3 []image.Uniform
	EditorMatchingParenthesis []image.Uniform

	Compl []image.Uniform

	TagPlain []image.Uniform
	TagSel1 []image.Uniform
	TagSel2 []image.Uniform
	TagSel3 []image.Uniform
	TagMatchingParenthesis []image.Uniform

	HandleFG image.Uniform
	HandleModifiedFG image.Uniform
	HandleSpecialFG image.Uniform
	HandleBG image.Uniform
}

var col2sel = *image.NewUniform(color.RGBA{0xAA, 0x00, 0x00, 0xFF})
var col3sel = *image.NewUniform(color.RGBA{0x00, 0x66, 0x00, 0xFF})
var bluebg = *image.NewUniform(color.RGBA{234, 0xff, 0xff, 0xff})
var yellowbg = *image.NewUniform(color.RGBA{0xff, 0xff, 234, 0xff})
var darkergreen = *image.NewUniform(color.RGBA{0x24, 0x49, 0x24, 0xff})

var acmeColorScheme = ColorScheme{
	WindowBG: *image.White,

	Border: *image.Black,
	Scrollbar: *image.NewUniform(color.RGBA{ 153, 153, 76, 0xff }),

	EditorPlain: []image.Uniform{ yellowbg, *image.Black, darkergreen, *DDarkblue },
	EditorSel1: []image.Uniform{ *DDarkyellow, *image.Black, darkergreen, *DDarkblue },
	EditorSel2: []image.Uniform{ col2sel, yellowbg, yellowbg, yellowbg },
	EditorSel3: []image.Uniform{ col3sel, yellowbg, yellowbg, yellowbg },
	EditorMatchingParenthesis: []image.Uniform{ *image.Black, yellowbg, yellowbg, yellowbg },
	Compl: []image.Uniform{ bluebg, *image.Black },

	TagPlain: []image.Uniform{  bluebg, *image.Black },
	TagSel1: []image.Uniform{ *DPalegreygreen, *image.Black },
	TagSel2: []image.Uniform{ col2sel, bluebg },
	TagSel3: []image.Uniform{ col3sel, bluebg },
	TagMatchingParenthesis: []image.Uniform{ *image.Black, bluebg },

	HandleFG:  bluebg,
	HandleModifiedFG: *DMedblue,
	HandleSpecialFG: *DMedgreen,
	HandleBG: *DPurpleblue,
}
