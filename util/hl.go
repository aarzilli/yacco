package util

type RegionMatchType int

const (
	RMT_STRING  = RegionMatchType(2)
	RMT_COMMENT = RegionMatchType(3)
)

type RegionMatch struct {
	NameRe     string
	StartDelim []rune
	EndDelim   []rune
	Escape     rune
	Type       RegionMatchType
}
