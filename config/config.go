package config

import (
	"yacco/util"
)

var TheColorScheme = AcmeColorScheme

var DefaultWindowTag = []rune("Newcol Getall Putall Jobs Exit | ")
var DefaultColumnTag = []rune("New Cut Paste Sort Zerox Delcol ")
var DefaultEditorTag = " Del"
var ColorEnabled = true

var StartupWidth = 640
var ScrollWidth = 10
var StartupHeight = 480

var MainFont = util.MustNewFont(72, 16, 1.0, true, "$HOME/.config/yacco/luxisr.ttf:$HOME/.config/yacco/DejaVuSans.ttf")
var MainFont2 = util.MustNewFont(72, 16, 1.0, true, "$HOME/.config/yacco/luxisr.ttf:$HOME/.config/yacco/DejaVuSans.ttf")
var TagFont = util.MustNewFont(72, 16, 0.9, true, "$HOME/.config/yacco/luxisr.ttf:$HOME/.config/yacco/DejaVuSans.ttf")
var AltFont = util.MustNewFont(72, 16, 1.0, true, "$HOME/.config/yacco/luximr.ttf")
var ComplFont = util.MustNewFont(72, 16, 1.0, true, "$HOME/.config/yacco/luxisr.ttf:$HOME/.config/yacco/DejaVuSans.ttf") // do not ever use fractional line spacing for multiline textframes

var EditErrorTrace = false

var EnableHighlighting = true
var ServeTCP = false
var HideHidden = true
var QuoteHack = true
