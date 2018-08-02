package config

import (
	"yacco/hl"
	"yacco/util"

	"golang.org/x/image/font"
)

var TheColorScheme = AcmeColorScheme

var DefaultWindowTag = []rune("Newcol Getall Putall Jobs Exit | ")
var DefaultColumnTag = []rune("New Cut Paste Sort Zerox Delcol ")
var DefaultEditorTag = " Del"

var StartupWidth = 640
var ScrollWidth = 10
var StartupHeight = 480
var ComplMaxX = 1024
var ComplMaxY = 1024

var MainFontSize int
var MainFont, TagFont, AltFont, ComplFont font.Face

var EditErrorTrace = false

var EnableHighlighting = true
var ServeTCP = false
var HideHidden = true

var FontSizeChange = 0

var Templates []string

const DefaultLookFileExt = ",c,cc,cpp,h,py,txt,pl,tcl,java,js,html,go,clj,jsp"

var LoadRules = []util.LoadRule{}

var LanguageRules = []hl.LanguageRules{
	// Go
	hl.LanguageRules{
		NameRe: `\.go$`,
		RegionMatches: []hl.RegionMatch{
			hl.StringRegion("`", "`", 0),
			hl.StringRegion("\"", "\"", '\\'),
			hl.StringRegion("'", "'", '\\'),
			hl.CommentRegion("/*", "*/", 0),
			hl.CommentRegion("//", "\n", 0),
			hl.RegexpRegion(`^(func|type)\s+(\([^\)]+\)\s+)?`, `\W`, 0, hl.RMT_HEADER),
		},
	},

	// C / C++ / Java / js
	hl.LanguageRules{
		NameRe: `\.(?:c|java|cpp|h|js)$`,
		RegionMatches: []hl.RegionMatch{
			hl.StringRegion("\"", "\"", '\\'),
			hl.StringRegion("'", "'", '\\'),
			hl.CommentRegion("/*", "*/", 0),
			hl.CommentRegion("//", "\n", 0),
		},
	},

	// Python
	hl.LanguageRules{
		NameRe: `\.py$`,
		RegionMatches: []hl.RegionMatch{
			hl.StringRegion(`"""`, `"""`, 0),
			hl.StringRegion("\"", "\"", '\\'),
			hl.StringRegion("'", "'", '\\'),
			hl.CommentRegion("#", "\n", 0),
		},
	},
}
