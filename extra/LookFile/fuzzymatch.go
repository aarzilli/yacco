package main

import (
	"strings"
	"unicode"
)

/*
Algorithm described here:
https://blog.forrestthewoods.com/reverse-engineering-sublime-text-s-fuzzy-match-4cffeed33fdb#.c1i15w2h6 https://www.reddit.com/r/programming/comments/4cfz8r/reverse_engineering_sublime_texts_fuzzy_match/
*/

func fuzzyMatch(needle, haystack string) (bool, int) {
	exact := false
	for _, ch := range needle {
		if unicode.IsUpper(ch) {
			exact = true
			break
		}
	}
	if !exact {
		needle = strings.ToLower(needle)
		haystack = strings.ToLower(haystack)
	}
	ok, score := bestFuzzyMatch([]rune(needle), []rune(haystack), true)
	if len(needle) == len(haystack) {
		score += 10
	}
	return ok, score
}

func bestFuzzyMatch(needle, haystack []rune, first bool) (bool, int) {
	const separatorBonus = 10
	const startPenalty = -3
	const startPenaltyMax = -9
	const consecutiveBonus = 5

	issep := func(ch rune) bool {
		return ch == ' ' || ch == '_' || ch == '-' || ch == '/' || ch == '.'
	}

	if len(needle) == 0 {
		return true, 0
	}
	aftersep := first
	afterlower := first

	match := false
	max := 0

	for i := range haystack {
		score := 0

		if haystack[i] == needle[0] {
			if aftersep || (afterlower && unicode.IsUpper(haystack[i])) {
				score += separatorBonus
			}

			if first {
				p := startPenalty * i
				if p < startPenaltyMax {
					p = startPenaltyMax
				}
				score += p
			} else {
				score -= i
				if i == 0 {
					score += consecutiveBonus
				}
			}

			ok, subscore := bestFuzzyMatch(needle[1:], haystack[i+1:], false)
			if ok {
				match = true
				if score+subscore > max {
					max = score + subscore
				}
			}
		}

		aftersep = issep(haystack[i])
		afterlower = unicode.IsLower(haystack[i])
	}

	return match, max
}
