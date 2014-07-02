package config

import (
	"regexp"
	"yacco/util"
)

var RegionMatches = []util.RegionMatch{
	util.RegionMatch{
		NameRe: nil,
	},

	// C / C++ / Java / Go / js
	util.RegionMatch{
		NameRe:     regexp.MustCompile(`\.go$`),
		StartDelim: []rune{'`'}, EndDelim: []rune{'`'}, Escape: 0, Type: util.RMT_STRING,
	},
	util.RegionMatch{
		NameRe:     regexp.MustCompile(`\.(?:c|java|cpp|h|go|js)$`),
		StartDelim: []rune{'"'}, EndDelim: []rune{'"'}, Escape: '\\', Type: util.RMT_STRING,
	},
	util.RegionMatch{
		NameRe:     regexp.MustCompile(`\.(?:c|java|cpp|h|go|js)$`),
		StartDelim: []rune{'\''}, EndDelim: []rune{'\''}, Escape: '\\', Type: util.RMT_STRING,
	},
	util.RegionMatch{
		NameRe:     regexp.MustCompile(`\.(?:c|java|cpp|h|go|js)$`),
		StartDelim: []rune("/*"), EndDelim: []rune("*/"), Escape: rune(0), Type: util.RMT_COMMENT,
	},
	util.RegionMatch{
		NameRe:     regexp.MustCompile(`\.(?:c|java|cpp|h|go|js)$`),
		StartDelim: []rune("//"), EndDelim: []rune{'\n'}, Escape: rune(0), Type: util.RMT_COMMENT,
	},

	// Python
	util.RegionMatch{
		NameRe:     regexp.MustCompile(`\.py$`),
		StartDelim: []rune("\"\"\""), EndDelim: []rune("\"\"\""), Escape: '\\', Type: util.RMT_STRING,
	},
	util.RegionMatch{
		NameRe:     regexp.MustCompile(`\.py$`),
		StartDelim: []rune{'"'}, EndDelim: []rune{'"'}, Escape: '\\', Type: util.RMT_STRING,
	},
	util.RegionMatch{
		NameRe:     regexp.MustCompile(`\.py$`),
		StartDelim: []rune{'\''}, EndDelim: []rune{'\''}, Escape: '\\', Type: util.RMT_STRING,
	},
	util.RegionMatch{
		NameRe:     regexp.MustCompile(`\.py$`),
		StartDelim: []rune{'`'}, EndDelim: []rune{'`'}, Escape: '\\', Type: util.RMT_STRING,
	},
	util.RegionMatch{
		NameRe:     regexp.MustCompile(`\.py$`),
		StartDelim: []rune{'#'}, EndDelim: []rune{'\n'}, Escape: rune(0), Type: util.RMT_COMMENT,
	},
}
