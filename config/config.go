package config

import (
	"yacco/util"
)

var TheColorScheme = AcmeColorScheme

var DefaultWindowTag = []rune("Newcol Putall Dump Exit | ")
var DefaultColumnTag = []rune("New Cut Paste Sort Zerox Delcol | ")
var DefaultEditorTag = " Del"

var MainFont = util.MustNewFontFromBytes(72, 16, 1.0, [][]byte{luxibytes})
var TagFont = util.MustNewFontFromBytes(72, 14, 1.0, [][]byte{luxibytes})
var AltFont = util.MustNewFontFromBytes(72, 16, 1.0, [][]byte{luximonobytes})
var ComplFont = util.MustNewFontFromBytes(72, 14, 1.0, [][]byte{luxibytes})

var EditErrorTrace = true

var TabElasticity = 4
var EnableHighlighting = true
