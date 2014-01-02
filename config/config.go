package config

import (
	"yacco/util"
)

var TheColorScheme = AcmeColorScheme

var DefaultWindowTag = []rune("Newcol Getall Jobs Exit | ")
var DefaultColumnTag = []rune("New Cut Paste Sort Zerox Delcol | ")
var DefaultEditorTag = " Del Look"

var MainFont = util.MustNewFont(72, 16, 1.0, "$HOME/.config/yacco/luxisr.ttf:$HOME/.config/yacco/DejaVuSans.ttf")
var TagFont = util.MustNewFont(72, 16, 0.9, "$HOME/.config/yacco/luxisr.ttf:$HOME/.config/yacco/DejaVuSans.ttf")
var AltFont = util.MustNewFont(72, 16, 1.0, "$HOME/.config/yacco/luximr.ttf")
var ComplFont = util.MustNewFont(72, 16, 1.0, "$HOME/.config/yacco/luxisr.ttf:$HOME/.config/yacco/DejaVuSans.ttf") // do not ever use fractional line spacing for multiline textframes

var EditErrorTrace = false

var EnableHighlighting = true
var ServeTCP = false
var HideHidden = true
