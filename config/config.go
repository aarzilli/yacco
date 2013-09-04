package config

import (
	"yacco/util"
)

var TheColorScheme = AcmeColorScheme

var DefaultWindowTag = []rune("Newcol Putall Dump Exit | ")
var DefaultColumnTag = []rune("New Cut Paste Sort Zerox Delcol | ")
var DefaultEditorTag = " Del"

var MainFont = util.MustNewFont(72, 16, 1.0, "$HOME/.config/yacco/luxisr.ttf:$HOME/.config/yacco/DejaVuSans.ttf")
var TagFont = util.MustNewFont(72, 16, 0.9, "$HOME/.config/yacco/luxisr.ttf:$HOME/.config/yacco/DejaVuSans.ttf")
var AltFont = util.MustNewFont(72, 16, 1.0, "$HOME/.config/yacco/luximr.ttf")
var ComplFont = TagFont

var EditErrorTrace = false

var TabElasticity = 4
var EnableHighlighting = true
