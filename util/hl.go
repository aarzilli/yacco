package util

import (
	"regexp"
)

type RegionMatchType int

const (
	RMT_STRING  = RegionMatchType(2)
	RMT_COMMENT = RegionMatchType(3)
)

type RegionMatch struct {
	NameRe     *regexp.Regexp
	StartDelim []rune
	EndDelim   []rune
	Escape     rune
	Type       RegionMatchType
}
