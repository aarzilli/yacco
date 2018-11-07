package go2def

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

func (ctx *context) Goroot() string {
	if ctx.goroot != nil {
		return *ctx.goroot
	}

	b, _ := exec.Command("go", "env", "GOROOT").CombinedOutput()
	s := strings.TrimSpace(string(b))
	ctx.goroot = &s
	return *ctx.goroot
}

type Config struct {
	Out io.Writer // output writer, defaults to os.Stdout

	Wd string // working directory, defaults to path directory

	Modfiles map[string][]byte // modified files

	Verbose           bool
	DebugLoadPackages bool
}

type context struct {
	Config
	goroot *string

	currentFileSet *token.FileSet

	pkgs []*packages.Package
}

func Describe(path string, pos [2]int, cfg *Config) {
	var ctx context

	if cfg != nil {
		ctx.Config = *cfg
	}
	if ctx.Wd == "" {
		ctx.Wd = filepath.Dir(path)
	}
	if ctx.Out == nil {
		ctx.Out = os.Stdout
	}

	err := loadPackages(&ctx, path)
	if err != nil {
		fmt.Fprintf(cfg.Out, "loading packages: %v", err)
		return
	}

	if cfg.Verbose && cfg.DebugLoadPackages {
		packages.Visit(ctx.pkgs, func(pkg *packages.Package) bool {
			log.Printf("package %v\n", pkg)
			return true
		}, nil)
	}

	found := false
	packages.Visit(ctx.pkgs, func(pkg *packages.Package) bool {
		for i := range pkg.Syntax {
			//TODO: better way to match file?
			if strings.HasSuffix(pkg.CompiledGoFiles[i], path) {
				node := findNodeInFile(pkg, pkg.Syntax[i], pos, pos[0] == pos[1])
				if node != nil {
					found = true
					describeNode(&ctx, pkg, node)
				}
				break
			}
		}
		return true
	}, nil)

	if !found {
		fmt.Fprintf(ctx.Out, "nothing found\n")
	}
}

func (ctx *context) getPosition(pos token.Pos) token.Position {
	return ctx.currentFileSet.Position(pos)
}

func (ctx *context) getFileSet(pos token.Pos) *token.FileSet {
	return ctx.currentFileSet
}

func (ctx *context) parseFile() func(*token.FileSet, string, []byte) (*ast.File, error) {
	if ctx.Modfiles == nil {
		return nil
	}

	return func(fset *token.FileSet, name string, obuf []byte) (*ast.File, error) {
		if buf, modified := ctx.Modfiles[name]; modified {
			return parser.ParseFile(fset, name, buf, parser.ParseComments)
		} else {
			return parser.ParseFile(fset, name, obuf, parser.ParseComments)
		}
	}
}

func loadPackages(ctx *context, path string) error {
	ctx.currentFileSet = token.NewFileSet()
	cfg := &packages.Config{
		Mode:      packages.LoadSyntax,
		Dir:       ctx.Wd,
		Fset:      ctx.currentFileSet,
		ParseFile: ctx.parseFile(),
	}
	if strings.HasSuffix(path, "_test.go") {
		cfg.Tests = true
	}
	var err error
	ctx.pkgs, err = packages.Load(cfg, "file="+path)
	return err
}

func findNodeInFile(pkg *packages.Package, root *ast.File, pos [2]int, autoexpand bool) ast.Node {
	v := &exactVisitor{pos, pkg, autoexpand, nil}
	ast.Walk(v, root)
	return v.ret
}

type exactVisitor struct {
	pos        [2]int
	pkg        *packages.Package
	autoexpand bool
	ret        ast.Node
}

func (v *exactVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}
	if v.pkg.Fset.Position(node.Pos()).Offset == v.pos[0] && v.pkg.Fset.Position(node.End()).Offset == v.pos[1] {
		v.ret = node
	} else if v.autoexpand && v.ret == nil {
		switch node.(type) {
		case *ast.Ident, *ast.SelectorExpr:
			if v.pkg.Fset.Position(node.Pos()).Offset == v.pos[0] || v.pkg.Fset.Position(node.End()).Offset == v.pos[0] {
				v.ret = node
			}
		}
	}
	return v
}

func describeNode(ctx *context, pkg *packages.Package, node ast.Node) {
	switch node := node.(type) {
	case *ast.Ident:
		obj := pkg.TypesInfo.Uses[node]
		if obj == nil {
			fmt.Fprintf(ctx.Out, "unknown identifier %v\n", node)
			return
		}

		declnode := findNodeInPackages(ctx, obj.Pkg().Path(), obj.Pos())
		if ctx.Verbose {
			log.Printf("declaration node %v\n", declnode)
		}

		if declnode != nil {
			describeDeclaration(ctx, declnode, obj.Type())

			printPos(ctx, "\n", ctx.getPosition(declnode.Pos()))

		} else {
			fmt.Fprintf(ctx.Out, "%s\n", obj)
			describeType(ctx, "type:", obj.Type())

			printPos(ctx, "\n", ctx.getPosition(obj.Pos()))

		}

	case *ast.SelectorExpr:
		sel := pkg.TypesInfo.Selections[node]
		if sel == nil {
			if idobj := pkg.TypesInfo.Uses[node.Sel]; idobj != nil {
				describeNode(ctx, pkg, node.Sel)
				return
			}
			if typeOfExpr := pkg.TypesInfo.Types[node.X]; typeOfExpr.Type != nil {
				describeType(ctx, "receiver:", typeOfExpr.Type)
				describeTypeContents(ctx.Out, typeOfExpr.Type, node.Sel.String())
				return
			}
			fmt.Fprintf(ctx.Out, "unknown selector expression ")
			printer.Fprint(ctx.Out, ctx.getFileSet(node.Pos()), node)
			fmt.Fprintf(ctx.Out, "\n")
			return
		}

		obj := sel.Obj()

		fallbackdescr := true

		declnode := findNodeInPackages(ctx, obj.Pkg().Path(), obj.Pos())
		pos := ctx.getPosition(obj.Pos())
		if declnode != nil {
			pos = ctx.getPosition(declnode.Pos())
			switch declnode := declnode.(type) {
			case *ast.FuncDecl:
				printFunc(ctx, declnode)
				fallbackdescr = false
			}
		}

		if fallbackdescr {
			switch sel.Kind() {
			case types.FieldVal:
				fmt.Fprintf(ctx.Out, "struct field ")
			case types.MethodVal:
				fmt.Fprintf(ctx.Out, "method ")
			case types.MethodExpr:
				fmt.Fprintf(ctx.Out, "method expression ")
			default:
				fmt.Fprintf(ctx.Out, "unknown selector ")
			}

			printer.Fprint(ctx.Out, ctx.getFileSet(node.Pos()), node)
			fmt.Fprintf(ctx.Out, "\n")
			describeType(ctx, "receiver:", sel.Recv())
			describeType(ctx, "type:", sel.Type())
		}

		printPos(ctx, "\n", pos)

	case ast.Expr:
		typeAndVal := pkg.TypesInfo.Types[node]
		describeType(ctx, "type:", typeAndVal.Type)
	}
}

func describeDeclaration(ctx *context, declnode ast.Node, typ types.Type) {
	normaldescr := true

	switch declnode := declnode.(type) {
	case *ast.FuncDecl:
		printFunc(ctx, declnode)
		normaldescr = false
	default:
	}

	if normaldescr {
		describeType(ctx, "type:", typ)
	}
}

func printFunc(ctx *context, declnode *ast.FuncDecl) {
	body := declnode.Body
	declnode.Body = nil
	printer.Fprint(ctx.Out, ctx.getFileSet(declnode.Pos()), declnode)
	fmt.Fprintf(ctx.Out, "\n")
	declnode.Body = body
}

func describeType(ctx *context, prefix string, typ types.Type) {
	fmt.Fprintf(ctx.Out, "%s %s\n", prefix, printTypesTypeNice(typ))
	if ptyp, isptr := typ.(*types.Pointer); isptr {
		typ = ptyp.Elem()
	}
	ntyp, isnamed := typ.(*types.Named)
	if !isnamed {
		return
	}
	obj := ntyp.Obj()
	if obj == nil {
		return
	}
	pos := ctx.getPosition(obj.Pos())
	printPos(ctx, "\t", pos)
}

func printPos(ctx *context, prefix string, pos token.Position) {
	filename := pos.Filename
	filename = replaceGoroot(ctx, filename)
	fmt.Fprintf(ctx.Out, "%s%s:%d\n", prefix, filename, pos.Line)
}

func replaceGoroot(ctx *context, filename string) string {
	const gorootPrefix = "$GOROOT"
	if strings.HasPrefix(filename, gorootPrefix) {
		filename = ctx.Goroot() + filename[len(gorootPrefix):]
	}
	return filename
}

func describeTypeContents(out io.Writer, typ types.Type, prefix string) {
	if prefix == "_" {
		prefix = ""
	}
	if ptyp, isptr := typ.(*types.Pointer); isptr {
		typ = ptyp.Elem()
	}

	switch styp := typ.(type) {
	case *types.Named:
		typ = styp.Underlying()
		ms := []*types.Func{}
		for i := 0; i < styp.NumMethods(); i++ {
			m := styp.Method(i)
			if strings.HasPrefix(m.Name(), prefix) {
				ms = append(ms, m)
			}
		}
		if len(ms) > 0 {
			fmt.Fprintf(out, "\nMethods:\n")
			for _, m := range ms {
				fmt.Fprintf(out, "\t%s\n", printTypesObjectNice(m))
			}
		}
	case *types.Interface:
		ms := []*types.Func{}
		for i := 0; i < styp.NumMethods(); i++ {
			m := styp.Method(i)
			if strings.HasPrefix(m.Name(), prefix) {
				ms = append(ms, m)
			}
		}
		if len(ms) > 0 {
			fmt.Fprintf(out, "\nMethods:\n")
			for _, m := range ms {
				fmt.Fprintf(out, "\t%s\n", printTypesObjectNice(m))
			}
		}
	}

	if typ, isstruct := typ.(*types.Struct); isstruct {
		fs := []*types.Var{}
		for i := 0; i < typ.NumFields(); i++ {
			f := typ.Field(i)
			if strings.HasPrefix(f.Name(), prefix) {
				fs = append(fs, f)
			}
		}
		if len(fs) > 0 {
			fmt.Fprintf(out, "\nFields:\n")
			for _, f := range fs {
				fmt.Fprintf(out, "\t%s\n", printTypesObjectNice(f))
			}
		}
	}
}

func findNodeInPackages(ctx *context, pkgpath string, pos token.Pos) ast.Node {
	var r ast.Node
	packages.Visit(ctx.pkgs, func(pkg *packages.Package) bool {
		if r != nil {
			return false
		}
		if pkg.PkgPath != pkgpath {
			return true
		}

		if pkg.Syntax == nil {
			if ctx.Verbose {
				log.Printf("loading syntax for %q", pkg.PkgPath)
			}
			pkgs2, err := packages.Load(&packages.Config{Mode: packages.LoadSyntax, Dir: ctx.Wd, Fset: pkg.Fset, ParseFile: ctx.parseFile()}, pkg.PkgPath)
			if err != nil {
				return true
			}

			pkg = pkgs2[0]
			p := pkg.Fset.Position(pos)
			filename := replaceGoroot(ctx, p.Filename)
			//XXX: ideally we would look for pkg.Fset.File(pos).Offset(pos) instead but it seems to be wrong.
			for i := range pkg.Syntax {
				node := findDeclByLine(ctx, pkg.Syntax[i], filename, p.Line)
				if node != nil {
					r = node
				}
			}
			return true
		}

		for i := range pkg.Syntax {
			node := findDecl(pkg.Syntax[i], pos)
			if node != nil {
				r = node
			}
		}
		return true
	}, nil)
	return r
}

func findDecl(root *ast.File, pos token.Pos) ast.Node {
	v := &exactVisitorForDecl{pos, nil}
	ast.Walk(v, root)
	return v.ret
}

type exactVisitorForDecl struct {
	pos token.Pos
	ret ast.Node
}

func (v *exactVisitorForDecl) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return v
	}
	if v.pos >= node.Pos() && v.pos < node.End() {
		switch node := node.(type) {
		case *ast.GenDecl, *ast.AssignStmt, *ast.DeclStmt:
			v.ret = node
		case *ast.FuncDecl:
			if v.pos == node.Name.Pos() {
				v.ret = node
			}
		}
	}
	return v
}

func findDeclByLine(ctx *context, root *ast.File, filename string, line int) ast.Node {
	v := &exactVisitorForFileLine{ctx, filename, line, nil}
	ast.Walk(v, root)
	return v.ret
}

type exactVisitorForFileLine struct {
	ctx      *context
	filename string
	line     int
	ret      ast.Node
}

func (v *exactVisitorForFileLine) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return v
	}
	p := v.ctx.getPosition(node.Pos())
	if v.filename == p.Filename && v.line == p.Line {
		switch node := node.(type) {
		case *ast.GenDecl, *ast.AssignStmt, *ast.DeclStmt, *ast.FuncDecl, *ast.Field:
			if v.ret == nil {
				v.ret = node
			}
		}
	}
	return v
}

func printTypesObjectNice(v types.Object) string {
	return types.ObjectString(v, func(pkg *types.Package) string {
		return pkg.Name()
	})
}

func printTypesTypeNice(t types.Type) string {
	return types.TypeString(t, func(pkg *types.Package) string {
		return pkg.Name()
	})
}
