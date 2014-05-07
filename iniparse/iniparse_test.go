package iniparse

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

type Pair struct {
	Iv int
	Sv string
}

type Conf struct {
	Core struct {
		Avar  int
		Avar2 string
		V     []int
		V2    []string
		Avar3 string
	}
	Amap  map[string]*Pair
	Apair *Pair
	Amap2 map[string]*Pair
}

func pairtest(t *testing.T, pname string, v, p *Pair) {
	if v.Iv != p.Iv {
		t.Fatalf("%s: mismatched Iv (%d %d)", pname, v.Iv, p.Iv)
	}

	if v.Sv != p.Sv {
		t.Fatalf("%s: mismatched Sv (%s %s)", pname, v.Sv, p.Sv)
	}
}

func maptest(t *testing.T, m map[string]*Pair, mname, key string, p *Pair) {
	v, ok := m[key]
	if !ok {
		t.Fatalf("%s: no %s key", mname, key)
	}

	pairtest(t, fmt.Sprintf("%s[%s]", mname, key), v, p)
}

func pairParserFn(path string, lineno int, lines []string) (interface{}, error) {
	var line = ""
	for i := range lines {
		x := strings.TrimSpace(lines[i])
		if x == "" {
			continue
		}
		if line != "" {
			return nil, fmt.Errorf("%s:%d: Expected only one line", path, lineno)
		}
		line = x
	}

	v := strings.Split(line, "/")
	if len(v) != 2 {
		return nil, fmt.Errorf("%s:%d: Malformed line: %s\n", path, lineno, line)
	}

	r := &Pair{}
	i, err := strconv.ParseInt(v[0], 10, 64)
	r.Iv = int(i)
	if err != nil {
		return nil, fmt.Errorf("%s:%d: %v\n", path, lineno, err)
	}
	r.Sv = v[1]
	return r, nil
}

func TestParse(t *testing.T) {
	confstr := `
	# a comment
[core]
Avar=1
Avar2=test
V=1
V=2
V=3
V2=some
V2=other # comment
V2=test
Avar3=this is a "test\n" end ; other comment

[amap "blah"]
Iv=1
Sv=one

[amap "bloh"]
Iv=2
Sv=two

[apair]
3/three

[amap2 "blee"]
4/four

[amap2 "bloo"]
5/five
`

	var confout Conf
	u := NewUnmarshaller()
	u.AddSpecialUnmarshaller("apair", pairParserFn)
	u.AddSpecialUnmarshaller("amap2", pairParserFn)
	err := u.Unmarshal([]byte(confstr), &confout)
	if err != nil {
		t.Fatalf("Error: %v\n", err)
	}

	if confout.Core.Avar != 1 {
		t.Fatalf("Avar not 1 (%d)", confout.Core.Avar)
	}

	if confout.Core.Avar2 != "test" {
		t.Fatalf("Avar2 not test (%s)", confout.Core.Avar2)
	}

	if (len(confout.Core.V) != 3) || (confout.Core.V[0] != 1) || (confout.Core.V[1] != 2) || (confout.Core.V[2] != 3) {
		t.Fatalf("V has the wrong value %v", confout.Core.V)
	}

	if (len(confout.Core.V2) != 3) || (confout.Core.V2[0] != "some") || (confout.Core.V2[1] != "other") || (confout.Core.V2[2] != "test") {
		t.Fatalf("V2 has the wrong value %v", confout.Core.V2)
	}

	if confout.Core.Avar3 != "this is a test\n end" {
		t.Fatalf("Avar3 has the wrong value '%s'", confout.Core.Avar3)
	}

	if len(confout.Amap) != 2 {
		t.Fatalf("Amap: wrong number of keys %d", len(confout.Amap))
	}

	maptest(t, confout.Amap, "Amap", "blah", &Pair{Iv: 1, Sv: "one"})
	maptest(t, confout.Amap, "Amap", "bloh", &Pair{Iv: 2, Sv: "two"})
	pairtest(t, "Apair", confout.Apair, &Pair{Iv: 3, Sv: "three"})
	maptest(t, confout.Amap2, "Amap2", "blee", &Pair{Iv: 4, Sv: "four"})
	maptest(t, confout.Amap2, "Amap2", "bloo", &Pair{Iv: 5, Sv: "five"})
}

/*
TESTS:
- a map with special parser
*/
