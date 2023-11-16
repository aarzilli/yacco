package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func TestReflow(t *testing.T) {
	const testdir = "testdata"
	fis, err := os.ReadDir(testdir)
	must(err)
	for _, fi := range fis {
		buf, _ := os.ReadFile(filepath.Join(testdir, fi.Name()))
		v := strings.Split(string(buf), "\n====\n")
		in := v[0]
		tgt := v[1]
		t.Run(fi.Name(), func(t *testing.T) { testHelper(t, fi.Name(), in, tgt) })
	}
}

func testHelper(t *testing.T, name, in, tgt string) {
	out := reflow(in, 75)
	if out != tgt {
		t.Errorf("wrong output for %s expected\n[%s]\ngot:\n[%s]\ninput:\n[%s]", name, tgt, out, in)
	}
}

// FuzzReflow does the same thing as TestReflow but with fuzzing
func FuzzReflow(f *testing.F) {
	const testdir = "testdata"
	fis, err := os.ReadDir(testdir)
	must(err)
	for _, fi := range fis {
		buf, _ := os.ReadFile(filepath.Join(testdir, fi.Name()))
		v := strings.Split(string(buf), "\n====\n")
		in := v[0]
		f.Add(in, 75)
	}
	f.Fuzz(func(t *testing.T, in string, sz int) {
		reflow(in, sz)
	})
}
