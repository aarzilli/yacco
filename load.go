package main

import (
	"fmt"
	"regexp"
	"strings"
	"path/filepath"
	"yacco/util"
	"yacco/config"
	"yacco/buf"
	"yacco/edit"
)

type LoadRule struct {
	BufRe *regexp.Regexp
	Re *regexp.Regexp
	Action string
}

var LoadRules []LoadRule

func LoadInit() {
	LoadRules = []LoadRule{}
	for _, rule := range config.LoadRules {
		if (rule.Action[0] != 'L') && (rule.Action[0] != 'X') {
			panic(fmt.Errorf("Actions must start with X or L in: %s", rule.Action))
		}
		LoadRules = append(LoadRules, LoadRule{ BufRe: regexp.MustCompile(rule.BufRe), Re: regexp.MustCompile(rule.Re), Action: rule.Action })
	}
}

func Load(ec ExecContext, origin int) {
	//println("in load")
	if ec.buf == nil {
		return
	}
	for _, rule := range LoadRules {
		path := filepath.Join(ec.buf.Dir, ec.buf.Name)
		if !rule.BufRe.MatchString(path) {
			continue
		}
		start := ec.fr.Sels[2].S
		rr := ec.buf.ReaderFrom(ec.fr.Sels[2].S, ec.fr.Sels[2].E)
		for {
			matches := rule.Re.FindReaderSubmatchIndex(rr)
			//println("match:", matches, ec.fr.Sels[2].S, ec.fr.Sels[2].E)
			if matches == nil {
				break
			}
			s := matches[0] + start
			e := matches[1] + start

			//println("match:", s, e, origin)

			if (s <= origin) && (origin <= e) {
				strmatches := []string{}
				for i := 0; 2*i+1 < len(matches); i++ {
					s := matches[2*i] + start
					e := matches[2*i+1] + start
					strmatches = append(strmatches, string(buf.ToRunes(ec.buf.SelectionX(util.Sel{s, e }))))
				}
				//println("Match:", strmatches[0])
				if rule.Exec(ec, strmatches, s, e) {
					return
				}
			}

			start = s+1
			if start > origin {
				break
			}
			rr = ec.buf.ReaderFrom(start, ec.fr.Sels[2].E)
		}
	}
}

func expandMatches(str string, matches []string) string {
	out := []byte{}
	sub := false
	for i := range str {
		if !sub {
			if str[i] == '$' {
				sub = true
			} else {
				out = append(out, str[i])
			}
		} else {
			if (str[i] >= '0') && (str[i] <= '9') {
				d := int(str[i] - '0')
				if d >= len(matches) {
					out = append(out, '$')
					out = append(out, str[i])
				} else {
					out = append(out, matches[d]...)
				}
			} else {
				out = append(out, '$')
				out = append(out, str[i])
			}
			sub = false
		}
	}
	return string(out)
}

func (rule *LoadRule) Exec(ec ExecContext, matches []string, s, e int) bool {
	action := rule.Action[1:]

	switch rule.Action[0] {
	case 'X':
		expaction := expandMatches(action, matches)
		Exec(ec, expaction)
		return true
	case 'L':
		v := strings.SplitN(action, ":", 2)
		path := expandMatches(v[0], matches)
		addrExpr := ""
		if len(v) > 1 {
			addrExpr = expandMatches(v[1], matches)
		}
		newed, err := EditFind(path, false)
		if err != nil {
			return false
		}
		if newed == nil {
			return false
		}
		ec.fr.Sels[2] = util.Sel{ s, e }
		ec.br.BufferRefresh(ec.ontag)
		if addrExpr != "" {
			newed.sfr.Fr.Sels[0] = edit.AddrEval(addrExpr, newed.bodybuf, newed.sfr.Fr.Sels[0])
			newed.BufferRefresh(false)
		}
		return true
	}
	return false
}


