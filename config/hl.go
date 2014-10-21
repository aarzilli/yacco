package config

import (
	"yacco/util"
)

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
		StartDelim: []rune("\"\"\""), EndDelim: []rune("\"\"\""), Escape: '\\', Type: util.RMT_STRING,
	},
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
