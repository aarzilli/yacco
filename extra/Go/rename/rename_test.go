package rename

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func assertNoError(err error, t *testing.T) {
	t.Helper()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}

func TestFindDeclForComment(t *testing.T) {
	fset := &token.FileSet{}
	file, err := parser.ParseFile(fset, "_fixtures/finddecl.go", nil, parser.ParseComments)
	assertNoError(err, t)

	for _, cmtg := range file.Comments {
		for _, cmt := range cmtg.List {
			t.Run(strings.TrimSpace(cmt.Text[len("//"):]), func(t *testing.T) {
				decls := findDecls(fset, file, cmt.Slash)
				t.Logf("%s -> %#v\n", cmt.Text, decls)
				if len(decls) != 1 {
					t.Errorf("wrong number of declarations")
				}
			})
		}
	}
}

func TestDeclToObjects(t *testing.T) {
	fset := &token.FileSet{}
	file, err := parser.ParseFile(fset, "_fixtures/decltoobjects.go", nil, parser.ParseComments)
	assertNoError(err, t)

	var tconf types.Config
	tconf.DisableUnusedImportCheck = true
	var tinfo types.Info
	tinfo.Defs = make(map[*ast.Ident]types.Object)
	_, err = tconf.Check("_fixtures", fset, []*ast.File{file}, &tinfo)
	//assertNoError(err, t)

	for _, cmtg := range file.Comments {
		for _, cmt := range cmtg.List {
			text := strings.TrimSpace(cmt.Text[len("//"):])
			fields := strings.Split(text, " ")
			testname := fields[0]
			vars := fields[1:]
			t.Run(testname, func(t *testing.T) {
				decls := findDecls(fset, file, cmt.Slash)
				t.Logf("decls %s -> %#v\n", testname, decls)
				objs := declsToObjects(decls, &tinfo)
				t.Logf("objs %s -> %#v\n", testname, objs)
				if len(objs) != len(vars) {
					t.Errorf("mismatch %q", vars)
				} else {
					for i := range objs {
						if objs[i].obj.Name() != vars[i] {
							t.Errorf("mismatch %q %q", objs[i].obj.Name(), vars[i])
							break
						}
					}
				}
			})
		}
	}
}

func TestRenames(t *testing.T) {
	renamePackages("_fixtures/renametest.go")
}
