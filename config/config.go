package config

import (
	"yacco/hl"
	"yacco/util"
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
var MainFont = util.MustNewFont(72, 16, 1.0, true, "$HOME/.config/yacco/luxisr.ttf:$HOME/.config/yacco/DejaVuSans.ttf")
var TagFont = util.MustNewFont(72, 16, 0.9, true, "$HOME/.config/yacco/luxisr.ttf:$HOME/.config/yacco/DejaVuSans.ttf")
var AltFont = util.MustNewFont(72, 16, 1.0, true, "$HOME/.config/yacco/luximr.ttf")
var ComplFont = util.MustNewFont(72, 16, 1.0, true, "$HOME/.config/yacco/luxisr.ttf:$HOME/.config/yacco/DejaVuSans.ttf") // do not ever use fractional line spacing for multiline textframes

var EditErrorTrace = false

var EnableHighlighting = true
var ServeTCP = false
var HideHidden = true

const DefaultLookFileExt = ",c,cc,cpp,h,py,txt,pl,tcl,java,js,html,go,clj,jsp"

var LoadRules = []util.LoadRule{
	util.LoadRule{BufRe: `.`, Re: `https?://\S+`, Action: "Xxdg-open $0"},
	util.LoadRule{BufRe: `.`, Re: `:([^ ]+)`, Action: "L:$1"},
	util.LoadRule{BufRe: `.`, Re: `([^:\s\(\)]+):(\d+):(\d+)`, Action: "L$1:$2-+#$3-#1"},
	util.LoadRule{BufRe: `.`, Re: `([^:\s\(\)]+):(\d+)`, Action: "L$1:$2"},
	util.LoadRule{BufRe: `.`, Re: `File "(.+?)", line (\d+)`, Action: "L$1:$2"},
	util.LoadRule{BufRe: `.`, Re: `at (\S+) line (\d+)`, Action: "L$1:$2"},
	util.LoadRule{BufRe: `.`, Re: `in (\S+) on line (\d+)`, Action: "L$1:$2"},
	util.LoadRule{BufRe: `.`, Re: `([^:\s\(\)]+):\[(\d+),(\d+)\]`, Action: "L$1:$2-+#$3"},
	util.LoadRule{BufRe: `.`, Re: `([^:\s\(\)]+):\t?/(.*)/`, Action: "L$1:/$2/"},
	util.LoadRule{BufRe: `.`, Re: `[^:\s\(\)]+`, Action: "L$0"},
	util.LoadRule{BufRe: `.`, Re: `\S+`, Action: "L$0"},
	util.LoadRule{BufRe: `.`, Re: `\w+`, Action: "XLook $l0"},
	util.LoadRule{BufRe: `.`, Re: `.+`, Action: "XLook $l0"},
}

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
