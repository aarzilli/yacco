package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func getcmtprefix(text string) (prefix, rest string) {
	i := 0
	for i < len(text) {
		if text[i] != ' ' && text[i] != '\t' {
			break
		}
		i++
	}

	if i >= len(text) {
		return "", text
	}

	var commentChar byte

	if ch := text[i]; ch == '/' || ch == '#' || ch == ';' || ch == '%' {
		commentChar = ch
	} else {
		return text[:i], text[i:]
	}

	for i < len(text) {
		if text[i] != commentChar {
			break
		}
		i++
	}

	if i < len(text) && text[i] == ' ' {
		i++
	}

	return text[:i], text[i:]
}

func getitemprefix(text string) (prefix, rest string) {
	if pfx := numlistprefix(text); pfx != "" {
		return pfx, text[len(pfx):]
	}
	i := 0
	for i < len(text) {
		if text[i] != ' ' && text[i] != '\t' {
			break
		}
		i++
	}

	if i >= len(text) {
		return "", text
	}

	if text[i] != '*' && text[i] != '-' {
		return "", text
	}

	i++

	if i >= len(text) {
		return "", text
	}

	if text[i] != ' ' && text[i] != '\t' {
		return "", text
	}

	return text[:i+1], text[i+1:]
}

func getprefix(text string) (prefix0, prefix1, rest string) {
	cmtprefix, rest := getcmtprefix(text)
	itemprefix, rest := getitemprefix(rest)
	itemprefix1 := []byte(itemprefix)
	for i := range itemprefix1 {
		if itemprefix1[i] != ' ' && itemprefix1[i] != '\t' {
			itemprefix1[i] = ' '
		}
	}
	return cmtprefix + itemprefix, cmtprefix + string(itemprefix1), rest
}

func splittext(text string, prefixlen0, prefixlen1, n int) []string {
	b := []byte(text)
	lastSpace := -1
	lineStart := 0
	prefixlen := prefixlen0
	for i := 0; i < len(b); i++ {
		if b[i] == ' ' || b[i] == '\t' {
			lastSpace = i
		}
		if (i-lineStart)+prefixlen > n && lastSpace >= 0 {
			b[lastSpace] = '\n'
			lineStart = lastSpace + 1
			lastSpace = -1
			prefixlen = prefixlen1
		}
	}
	return strings.Split(string(b), "\n")
}

func flush(w io.Writer, in string, sz int) {
	prefix0, prefix1, t := getprefix(in)
	v := splittext(t, len(prefix0), len(prefix1), sz)
	prefix := prefix0
	for _, line := range v {
		fmt.Fprintf(w, "%s%s\n", prefix, line)
		prefix = prefix1
	}
}

func numlistprefix(line string) string {
	var i int
	for i = range line {
		if line[i] != ' ' {
			break
		}
	}
	found := false
	for ; i < len(line); i++ {
		if line[i] < '0' || line[i] > '9' {
			break
		}
		found = true
	}
	if !found || i >= len(line) || i+1 >= len(line) {
		return ""
	}
	if line[i] == '.' || line[i] == ')' {
		for i++; i < len(line); i++ {
			if line[i] != ' ' {
				return line[:i]
			}
		}
		return ""
	}
	return ""
}

func reflow(in string, sz int) string {
	lines := strings.Split(in, "\n")
	outlines := []string{}
	line := []string{}
	for i := 0; i < len(lines); i++ {
		line = append(line, strings.TrimRight(lines[i], " "))
		if len(line) > 1 {
			cmt, _ := getcmtprefix(line[0])
			if cmt != "" && strings.HasPrefix(line[len(line)-1], cmt) {
				line[len(line)-1] = line[len(line)-1][len(cmt):]
			}
		}
		cur := line[len(line)-1]
		_, cur = getcmtprefix(cur)
		if cur == "" || cur[len(cur)-1] == '.' || cur[len(cur)-1] == ',' || cur[len(cur)-1] == ':' || cur[len(cur)-1] == ';' {
			outlines = append(outlines, strings.Join(line, " "))
			line = line[:0]
			continue
		}
		if i+1 < len(lines) && len(lines[i+1]) > 0 && len(line) > 0 {
			cmt, _ := getcmtprefix(line[0])
			next := lines[i+1]
			if cmt != "" && strings.HasPrefix(next, cmt) {
				next = next[len(cmt):]
			}
			ch, _ := utf8.DecodeRuneInString(next)
			if (!unicode.IsLetter(ch) && !unicode.IsNumber(ch)) || numlistprefix(next) != "" {
				outlines = append(outlines, strings.Join(line, " "))
				line = line[:0]
				continue
			}
		}
	}
	if len(line) > 0 {
		outlines = append(outlines, strings.Join(line, " "))
	}

	out := new(bytes.Buffer)
	for _, line := range outlines {
		flush(out, line, sz)
	}
	outbuf := out.String()
	return outbuf[:len(outbuf)-1]
}

func main() {
	sz := 75
	if len(os.Args) > 2 {
		n, _ := strconv.Atoi(os.Args[1])
		sz = int(n)
	}
	buf, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
	os.Stdout.Write([]byte(reflow(string(buf), sz)))
}
