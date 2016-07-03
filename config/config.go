package config

import (
	"yacco/util"
)

var TheColorScheme = AcmeColorScheme

var DefaultWindowTag = []rune("Newcol Getall Putall Jobs Exit | ")
var DefaultColumnTag = []rune("New Cut Paste Sort Zerox Delcol ")
var DefaultEditorTag = " Del"

var StartupWidth = 640
var ScrollWidth = 10
var StartupHeight = 480

var MainFontSize int
var MainFont = util.MustNewFont(72, 16, 1.0, true, "$HOME/.config/yacco/luxisr.ttf:$HOME/.config/yacco/DejaVuSans.ttf")
var TagFont = util.MustNewFont(72, 16, 0.9, true, "$HOME/.config/yacco/luxisr.ttf:$HOME/.config/yacco/DejaVuSans.ttf")
var AltFont = util.MustNewFont(72, 16, 1.0, true, "$HOME/.config/yacco/luximr.ttf")
var ComplFont = util.MustNewFont(72, 16, 1.0, true, "$HOME/.config/yacco/luxisr.ttf:$HOME/.config/yacco/DejaVuSans.ttf") // do not ever use fractional line spacing for multiline textframes

var EditErrorTrace = false

var EnableHighlighting = true
var ServeTCP = false
var HideHidden = true

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

var RegionMatches = []util.RegionMatch{
	util.RegionMatch{
		NameRe:     `\.go$`,
		StartDelim: []rune{'`'}, EndDelim: []rune{'`'}, Escape: 0, Type: util.RMT_STRING,
	},
	util.RegionMatch{
		NameRe:     `\.go$`,
		StartDelim: []rune{'"'}, EndDelim: []rune{'"'}, Escape: '\\', Type: util.RMT_STRING,
	},
	util.RegionMatch{
		NameRe:     `\.go$`,
		StartDelim: []rune{'\''}, EndDelim: []rune{'\''}, Escape: '\\', Type: util.RMT_STRING,
	},
	util.RegionMatch{
		NameRe:     `\.go$`,
		StartDelim: []rune("/*"), EndDelim: []rune("*/"), Escape: rune(0), Type: util.RMT_COMMENT,
	},
	util.RegionMatch{
		NameRe:     `\.go$`,
		StartDelim: []rune("//"), EndDelim: []rune{'\n'}, Escape: rune(0), Type: util.RMT_COMMENT,
	},

	// C / C++ / Java / js
	util.RegionMatch{
		NameRe:     `\.(?:c|java|cpp|h|js)$`,
		StartDelim: []rune{'"'}, EndDelim: []rune{'"'}, Escape: '\\', Type: util.RMT_STRING,
	},
	util.RegionMatch{
		NameRe:     `\.(?:c|java|cpp|h|js)$`,
		StartDelim: []rune{'\''}, EndDelim: []rune{'\''}, Escape: '\\', Type: util.RMT_STRING,
	},
	util.RegionMatch{
		NameRe:     `\.(?:c|java|cpp|h|js)$`,
		StartDelim: []rune("/*"), EndDelim: []rune("*/"), Escape: rune(0), Type: util.RMT_COMMENT,
	},
	util.RegionMatch{
		NameRe:     `\.(?:c|java|cpp|h|js)$`,
		StartDelim: []rune("//"), EndDelim: []rune{'\n'}, Escape: rune(0), Type: util.RMT_COMMENT,
	},

	// Python
	util.RegionMatch{
		NameRe:     `\.py$`,
		StartDelim: []rune{'"'}, EndDelim: []rune{'"'}, Escape: '\\', Type: util.RMT_STRING,
	},
	util.RegionMatch{
		NameRe:     `\.py$`,
		StartDelim: []rune{'\''}, EndDelim: []rune{'\''}, Escape: '\\', Type: util.RMT_STRING,
	},
	util.RegionMatch{
		NameRe:     `\.py$`,
		StartDelim: []rune{'`'}, EndDelim: []rune{'`'}, Escape: '\\', Type: util.RMT_STRING,
	},
	util.RegionMatch{
		NameRe:     `\.py$`,
		StartDelim: []rune{'#'}, EndDelim: []rune{'\n'}, Escape: rune(0), Type: util.RMT_COMMENT,
	},
}

