package hl

import (
	"regexp"
	yregexp "yacco/regexp"
)

type RegionMatchType uint8

const (
	RMT_STRING RegionMatchType = iota + 2
	RMT_COMMENT
	RMT_HEADER
)

type LanguageRules struct {
	NameRe        string
	re            *regexp.Regexp
	RegionMatches []RegionMatch
}

// RegionMatch describes a syntax highlighting rule
type RegionMatch struct {
	StartDelim, EndDelim   []rune
	StartRegexp, EndRegexp yregexp.Regex
	Escape                 rune
	Type, DelimType        RegionMatchType
}

func StringRegion(start, end string, escape rune) RegionMatch {
	return RegionMatch{
		StartDelim: []rune(start),
		EndDelim:   []rune(end),
		Escape:     escape,
		Type:       RMT_STRING,
		DelimType:  RMT_STRING,
	}
}

func CommentRegion(start, end string, escape rune) RegionMatch {
	return RegionMatch{
		StartDelim: []rune(start),
		EndDelim:   []rune(end),
		Escape:     escape,
		Type:       RMT_COMMENT,
		DelimType:  RMT_COMMENT,
	}
}

func RegexpRegion(start, end string, escape rune, typ RegionMatchType) RegionMatch {
	return RegionMatch{
		StartRegexp: yregexp.Compile(start, false, false),
		EndRegexp:   yregexp.Compile(end, false, false),
		Escape:      escape,
		Type:        typ,
		DelimType:   1,
	}
}
