package rename

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"

	"github.com/aarzilli/yacco/util"
)

const debugFind = false
const debugChange = false
const debugPrintChange = false
const debug = debugFind || debugChange
const dummy = false

const arrow = "->"
const sparrow = " -> "

func Auto() []string {
	out, err := exec.Command("go", "list", "-m").CombinedOutput()
	util.Allergic(debug, err)

	return renamePackages(strings.TrimSpace(string(out)) + "/...")
}

func HasRenameComment(file *ast.File) bool {
	for _, cmtg := range file.Comments {
		for _, cmt := range cmtg.List {
			if !strings.HasPrefix(cmt.Text, "//") {
				continue
			}
			t := strings.TrimLeft(cmt.Text[len("//"):], " ")
			var from, to string
			switch {
			case strings.HasPrefix(t, arrow):
				from = ""
				to = t[len(arrow):]
			default:
				idx := strings.Index(t, sparrow)
				if idx >= 0 {
					from = t[:idx]
					to = t[idx+len(sparrow):]
				}
			}
			if from != "" || to != "" {
				from = strings.TrimSpace(from)
				to = strings.TrimSpace(to)
				if !strings.Contains(from, " ") && !strings.Contains(to, " ") {
					return true
				}
			}
		}
	}
	return false
}

func renamePackages(pattern string) []string {
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedFiles | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedCompiledGoFiles,
		Fset:  &token.FileSet{},
		Tests: true,
		ParseFile: func(fset *token.FileSet, filename string, src []byte) (*ast.File, error) {
			return parser.ParseFile(fset, filename, src, parser.ParseComments)
		},
	}

	pkgs, err := packages.Load(cfg, pattern)
	util.Allergic(debug, err)

	changedFiles := make(map[string]*ast.File)
	seenFile := make(map[string]struct{})

	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		for i := range pkg.CompiledGoFiles {
			if _, isseen := seenFile[pkg.CompiledGoFiles[i]]; isseen {
				// Some files are part of both the normal package and the test package,
				// only visit each once.
				continue
			}
			seenFile[pkg.CompiledGoFiles[i]] = struct{}{}
			file := pkg.Syntax[i]
			for j, cmtg := range file.Comments {
				for k, cmt := range cmtg.List {
					if !strings.HasPrefix(cmt.Text, "//") {
						continue
					}
					t := strings.TrimLeft(cmt.Text[len("//"):], " ")
					var cf map[string]*ast.File
					switch {
					case strings.HasPrefix(t, arrow):
						cf = rename("", t[len(arrow):], cfg, pkgs, pkg, file, cmt.Slash)
					default:
						idx := strings.Index(t, sparrow)
						if idx >= 0 {
							cf = rename(t[:idx], t[idx+len(sparrow):], cfg, pkgs, pkg, file, cmt.Slash)
						}
					}
					if len(cf) > 0 {
						cmtg.List[k] = nil
					}
					mergeChangedFiles(changedFiles, cf)
				}
				compactCommentGroup(cmtg)
				if len(cmtg.List) == 0 {
					file.Comments[j] = nil
				}
			}
			compactComments(file)
		}

		return true
	}, nil)

	if debugPrintChange {
		for k := range changedFiles {
			fmt.Printf("%s:\n", k)
			format.Node(os.Stdout, cfg.Fset, changedFiles[k])
		}
	}

	r := make([]string, len(changedFiles))
	for k := range changedFiles {
		r = append(r, k)
		if !dummy {
			f, err := os.OpenFile(k, os.O_WRONLY|os.O_TRUNC, 0)
			if err != nil {
				fmt.Fprintf(os.Stderr, "could not update %s: %v", k, err)
				continue
			}
			w := bufio.NewWriter(f)
			err = format.Node(w, cfg.Fset, changedFiles[k])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error formatting %s: %v", k, err)
			}
			err = w.Flush()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error writing %s: %v", k, err)
			}
			err = f.Close()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error closing %s: %v", k, err)
			}
		}
	}

	sort.Strings(r)
	return r
}

// rename renames the identifier 'from' into 'to' declared in 'file' on the
// same line as 'pos'.
// If 'from' is the empty string then the line must either contain a single
// declaration or be a function declaration.
// If 'from' is not the empty string then the line can contain multiple
// declaration and one must match 'from' exactly.
// If 'to' is "$" the first letter of the identifier is changed from upcase
// to lower case or viceversa.
func rename(from, to string, cfg *packages.Config, pkgs []*packages.Package, pkg *packages.Package, file *ast.File, pos token.Pos) map[string]*ast.File {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if strings.Contains(from, " ") || strings.Contains(to, " ") {
		return nil
	}
	// Find all objects declared on the same line as 'pos'
	if debugFind {
		fmt.Printf("rename %s -> %s (%v)\n", from, to, file.Name)
	}
	decls := findDecls(cfg.Fset, file, pos)
	objs := declsToObjects(decls, pkg.TypesInfo)
	if debugFind {
		fmt.Printf("decls %#v\n", decls)
		fmt.Printf("objects %#v\n", objs)
	}
	if len(objs) == 0 {
		if debugFind {
			fmt.Printf("\n")
		}
		return nil
	}
	if !debugFind && debugChange {
		fmt.Printf("rename %s -> %s (%v)\n", from, to, file.Name)
	}

	// Apply rules about 'from', determine the target object
	var tgtobj types.Object
	if from == "" {
		// Must contain a single declation or have a function declaration
		if len(objs) == 1 {
			tgtobj = objs[0].obj
		} else {
			for _, obj := range objs {
				if obj.isfunc {
					tgtobj = obj.obj
					break
				}
			}
		}
		if tgtobj == nil {
			if debugChange {
				fmt.Printf("Multiple declaration on the current line: %q\n", objNames(objs))
			}
			return nil
		}
		from = tgtobj.Name()
	} else {
		for _, obj := range objs {
			if obj.obj.Name() == from {
				tgtobj = obj.obj
				break
			}
		}
		if tgtobj == nil {
			if debugChange {
				fmt.Printf("Could not find object %q on its line: %q\n", from, objNames(objs))
			}
			return nil
		}
	}
	if debugChange {
		fmt.Printf("target object (%s) %#v\n", tgtobj.Name(), tgtobj)
	}

	// Apply rules about 'to'
	if to == "$" {
		runes := []rune(from)
		if unicode.IsUpper(runes[0]) {
			runes[0] = unicode.ToLower(runes[0])
		} else if unicode.IsLower(runes[0]) {
			runes[0] = unicode.ToUpper(runes[0])
		}
		to = string(runes)
	}

	// Determine target scope
	var scopePkgs []*packages.Package

	if scope := tgtobj.Parent(); scope == nil || ((scope.Pos() == 0) && (scope.End() == 0)) {
		if unicode.IsUpper([]rune(tgtobj.Name())[0]) {
			if debugChange {
				fmt.Printf("global scope\n")
			}
			scopePkgs = pkgs
		} else {
			if debugChange {
				fmt.Printf("package scope\n")
			}
			scopePkgs = []*packages.Package{pkg}
		}
	} else {
		if debugChange {
			fmt.Printf("limited scope\n")
		}
		scopePkgs = []*packages.Package{pkg}
	}
	// add the .test package
	if len(scopePkgs) == 1 {
		for _, pkg2 := range pkgs {
			if pkg2.PkgPath == scopePkgs[0].PkgPath+".test" || pkg2.PkgPath == scopePkgs[0].PkgPath+"_test" {
				scopePkgs = append(scopePkgs, pkg2)
				break
			}
		}
	}

	changedFiles := make(map[string]*ast.File)

	for _, usepkg := range scopePkgs {
		mergeChangedFiles(changedFiles, dorename(tgtobj, to, cfg, usepkg))
	}

	if debugChange {
		fmt.Printf("\n")
	}

	return changedFiles
}

// findDecls finds all declarations on the same line as pos inside 'file'.
// The returned value will include:
// - ast.FuncDecl, ast.ValueSpec, ast.TypeSpec, ast.AssignStmt, ast.RangeStmt, ast.Field
func findDecls(fset *token.FileSet, file *ast.File, pos token.Pos) []ast.Node {
	line := fset.Position(pos).Line
	var decls []ast.Node

	addsameline := func(node ast.Node) {
		if sameLine(fset, node, line) {
			decls = append(decls, node)
		}
	}

	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil || pos < node.Pos() {
			return false
		}
		if pos > node.End() {
			if !sameLine(fset, node, line) {
				return false
			}
		}
		switch node := node.(type) {
		case *ast.AssignStmt:
			if node.Tok != token.DEFINE {
				return false
			} else {
				addsameline(node)
			}
		case *ast.ValueSpec:
			addsameline(node)
		case *ast.TypeSpec:
			addsameline(node)
		case *ast.FuncDecl:
			addsameline(node)
		case *ast.RangeStmt:
			addsameline(node)
		case *ast.Field:
			addsameline(node)
		}
		return true
	})
	return decls
}

// sameLine is true if node.Pos() is on 'line' (the file is not checked)
func sameLine(fset *token.FileSet, node ast.Node, line int) bool {
	return fset.Position(node.Pos()).Line == line
}

type object struct {
	obj    types.Object
	isfunc bool
}

// declToIdent extracts objects declared in decls
func declsToObjects(decls []ast.Node, tinfo *types.Info) []object {
	r := []object{}
	for _, decl := range decls {
		for _, obj := range declToObjects(decl, tinfo) {
			r = append(r, obj)
		}
	}
	return r
}

func declToObjects(decl ast.Node, tinfo *types.Info) []object {
	objs := []object{}
	f := func(name *ast.Ident, isfunc bool) {
		if name == nil {
			return
		}
		if name.Name == "_" {
			return
		}
		obj := tinfo.ObjectOf(name)
		if obj == nil {
			return
		}
		if decl.Pos() <= obj.Pos() && obj.Pos() <= decl.End() {
			objs = append(objs, object{obj, isfunc})
		}
	}

	fx := func(x ast.Expr) {
		if id, isident := x.(*ast.Ident); isident {
			f(id, false)
		}
	}

	ff := func(fieldlist *ast.FieldList) {
		if fieldlist == nil {
			return
		}
		for _, field := range fieldlist.List {
			for _, name := range field.Names {
				f(name, false)
			}
		}
	}

	fn := func(names []*ast.Ident) {
		for _, name := range names {
			f(name, false)
		}
	}

	switch x := decl.(type) {
	case *ast.AssignStmt:
		for _, expr := range x.Lhs {
			fx(expr)
		}
	case *ast.ValueSpec:
		fn(x.Names)
	case *ast.TypeSpec:
		f(x.Name, false)
	case *ast.FuncDecl:
		f(x.Name, true)
		ff(x.Recv)
		ff(x.Type.Params)
		ff(x.Type.Results)
	case *ast.RangeStmt:
		fx(x.Key)
		fx(x.Value)
	case *ast.Field:
		fn(x.Names)
	}

	return objs
}

func objNames(objs []object) []string {
	names := make([]string, len(objs))
	for i := range objs {
		names[i] = objs[i].obj.Name()
	}
	return names
}

func dorename(tgtobj types.Object, to string, cfg *packages.Config, usepkg *packages.Package) map[string]*ast.File {
	changed := []*ast.Ident{}
	for name, obj := range usepkg.TypesInfo.Defs {
		if obj == tgtobj {
			name.Name = to
			changed = append(changed, name)
		}
	}
	for name, obj := range usepkg.TypesInfo.Uses {
		if obj == tgtobj {
			name.Name = to
			changed = append(changed, name)
		}
	}
	for selx, sel := range usepkg.TypesInfo.Selections {
		o := sel.Obj()
		if o == tgtobj {
			selx.Sel.Name = to
			changed = append(changed, selx.Sel)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	if debugChange {
		fmt.Printf("renamed %d identifiers\n", len(changed))
	}
	changedFiles := make(map[string]*ast.File)
	for _, ident := range changed {
		for i := range usepkg.CompiledGoFiles {
			path := usepkg.CompiledGoFiles[i]
			if changedFiles[path] != nil {
				continue
			}
			file := usepkg.Syntax[i]
			if containsPos(file, ident) {
				changedFiles[path] = file
			}
		}
	}
	if debugChange {
		for k := range changedFiles {
			fmt.Printf("\tchanged %s\n", k)
		}
	}

	return changedFiles
}

func containsPos(a, b ast.Node) bool {
	return a.Pos() <= b.Pos() && a.End() >= b.End()
}

func compactCommentGroup(cmtg *ast.CommentGroup) {
	list := cmtg.List[:0]
	for i := range cmtg.List {
		if cmtg.List[i] != nil {
			list = append(list, cmtg.List[i])
		}
	}
	cmtg.List = list
}

func compactComments(file *ast.File) {
	comments := file.Comments[:0]
	for i := range file.Comments {
		if file.Comments[i] != nil {
			comments = append(comments, file.Comments[i])
		}
	}
	file.Comments = comments
}

func mergeChangedFiles(dest, src map[string]*ast.File) {
	if src == nil {
		return
	}

	for k, v := range src {
		dest[k] = v
	}
}
