package util

import (
	"bytes"
	"testing"
)

func arrayOut(v []string) string {
	var buf bytes.Buffer

	for i := range v {
		buf.WriteByte('<')
		buf.WriteString(v[i])
		buf.WriteString(">, ")
	}

	return string(buf.Bytes())
}

func splitIs(t *testing.T, in string, tgt []string) {
	out := quotedSplit(in)
	if len(out) != len(tgt) {
		t.Fatalf("Parsing of <%s> failed: tgt: [%s] out: [%s]\n", in, arrayOut(tgt), arrayOut(out))
	}

	for i := range out {
		if out[i] != tgt[i] {
			t.Fatalf("Parsing of <%s> failed: tgt: [%s] out: [%s]\n", in, arrayOut(tgt), arrayOut(out))
		}
	}
}

func TestQuotedSplit(t *testing.T) {
	splitIs(t, "something something", []string{"something", "something"})
	splitIs(t, "something \"something, else\"", []string{"something", "something, else"})
	splitIs(t, "something      	'something \\- else'", []string{"something", "something - else"})
	splitIs(t, "something \"\\\"something\\\"\"", []string{"something", "\"something\""})
}
