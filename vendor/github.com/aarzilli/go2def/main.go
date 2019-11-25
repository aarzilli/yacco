package go2def

import (
	"bytes"
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
	"runtime"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

var goroot string
var possibleGoos, possibleGoarch map[string]struct{}

var setupOnce sync.Once

func (ctx *context) Goroot() string {
	setupOnce.Do(func() {
		possibleGoos = make(map[string]struct{})
		possibleGoarch = make(map[string]struct{})
		b, _ := exec.Command("go", "env", "GOROOT").CombinedOutput()
		goroot = strings.TrimSpace(string(b))

		b, _ = exec.Command("go", "tool", "dist", "list").Output()
		lines := strings.Split(string(b), "\n")
		for _, line := range lines {
			slash := strings.Index(line, "/")
			if slash < 0 {
				continue
			}
			possibleGoos[line[:slash]] = struct{}{}
			possibleGoarch[line[slash+1:]] = struct{}{}
		}
	})
	return goroot
}

func (ctx *context) PossibleGoos() map[string]struct{} {
	ctx.Goroot()
	return possibleGoos
}

func (ctx *context) PossibleGoarch() map[string]struct{} {
	ctx.Goroot()
	return possibleGoarch
}

type Config struct {
	Out io.Writer // output writer, defaults to os.Stdout

	Wd string // working directory, defaults to path directory

	Modfiles map[string][]byte // modified files

	Verbose           bool
	DebugLoadPackages bool

	out Description
}

type context struct {
	Config

	currentFileSet *token.FileSet

	pkgs []*packages.Package

	originalPath string
	buildTags    []string
}

func Describe(path string, pos [2]int, cfg *Config) Description {
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

	ctx.originalPath = path

	err := loadPackages(&ctx, path)
	if err != nil {
		cfg.out.err("loading packages: %v", err)
		return nil
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
	} else {
		ctx.out.writeTo(ctx.Out)
	}

	return ctx.out
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

func isGoarch(ctx *context, x string) bool {
	_, ok := ctx.PossibleGoarch()[x]
	return ok
}

func isGoos(ctx *context, x string) bool {
	_, ok := ctx.PossibleGoos()[x]
	return ok
}

func decorateConfig(ctx *context, cfg *packages.Config) *packages.Config {
	const (
		goSuffix   = ".go"
		testSuffix = "_test"
	)
	p := ctx.originalPath
	if !strings.HasSuffix(p, goSuffix) {
		return cfg
	}
	p = p[:len(p)-len(goSuffix)]
	if strings.HasSuffix(p, testSuffix) {
		cfg.Tests = true
		p = p[:len(p)-len(testSuffix)]
	}

	if len(cfg.Env) == 0 {
		cfg.Env = os.Environ()
	}

	goarch := runtime.GOARCH
	goos := runtime.GOOS

	if underscore := strings.LastIndex(p, "_"); underscore >= 0 {
		if isGoarch(ctx, p[underscore+1:]) {
			goarch = p[underscore+1:]
			cfg.Env = append(cfg.Env, "GOARCH="+goarch)
			p = p[:underscore]
		}
	}

	if underscore := strings.LastIndex(p, "_"); underscore >= 0 {
		if isGoos(ctx, p[underscore+1:]) {
			goos = p[underscore+1:]
			cfg.Env = append(cfg.Env, "GOOS="+goos)
			p = p[:underscore]
		}
	}

	if ctx.buildTags == nil {
		findBuildTags(ctx, goos, goarch)
	}

	cfg.BuildFlags = append(cfg.BuildFlags, ctx.buildTags...)

	return cfg
}

func findBuildTags(ctx *context, goos, goarch string) {
	const (
		singlelineComment   = "//"
		multilineComment    = "/*"
		multilineCommentEnd = "*/"
		buildDirective      = "+build"
	)
	var fset token.FileSet
	f, err := parser.ParseFile(&fset, ctx.originalPath, nil, parser.ParseComments|parser.ImportsOnly)
	if err == nil {
	commentLoop:
		for _, cmtg := range f.Comments {
			for _, cmt := range cmtg.List {
				cmtt := cmt.Text
				switch {
				case strings.HasPrefix(cmtt, singlelineComment):
					cmtt = cmtt[len(singlelineComment):]
				case strings.HasPrefix(cmtt, multilineComment):
					cmtt = cmtt[len(multilineComment):]
					if strings.HasSuffix(cmtt, multilineComment) {
						cmtt = cmtt[:len(cmtt)-len(multilineCommentEnd)]
					}
				}

				cmtt = strings.TrimSpace(cmtt)

				if !strings.HasPrefix(cmtt, buildDirective) {
					continue
				}

				cmtt = strings.TrimSpace(cmtt[len(buildDirective):])

				for _, alt := range strings.Split(cmtt, " ") {
					alt = strings.TrimSpace(alt)
					if matchBuildDirectiveAlternative(ctx, alt, goos, goarch) {
						break commentLoop
					}
				}
			}
		}

		if ctx.buildTags == nil {
			ctx.buildTags = []string{}
		}
	}
}

func matchBuildDirectiveAlternative(ctx *context, alt, goos, goarch string) bool {
	tags := []string{}
	for _, term := range strings.Split(alt, ",") {
		if len(term) == 0 {
			continue
		}
		exclude := false
		if term[0] == '!' {
			exclude = true
			term = term[1:]
		}
		if len(term) == 0 {
			continue
		}

		switch {
		case isGoos(ctx, term):
			if (term == goos) == exclude {
				return false
			}

		case isGoarch(ctx, term):
			if (term == goarch) == exclude {
				return false
			}

		default:
			if !exclude {
				tags = append(tags, term)
			}
		}
	}

	ctx.buildTags = []string{"-tags", strings.Join(tags, ",")}
	return true

}

func loadPackages(ctx *context, path string) error {
	ctx.currentFileSet = token.NewFileSet()
	cfg := &packages.Config{
		Mode:      packages.LoadSyntax,
		Dir:       ctx.Wd,
		Fset:      ctx.currentFileSet,
		ParseFile: ctx.parseFile(),
	}
	decorateConfig(ctx, cfg)
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
			ctx.out.err("unknown identifier %v\n", node)
			return
		}

		declnode := findNodeInPackages(ctx, obj.Pkg().Path(), obj.Pos())
		if ctx.Verbose {
			log.Printf("declaration node %v\n", declnode)
		}

		if declnode != nil {
			describeDeclaration(ctx, declnode, obj.Type())

			ctx.out.pos(pos2str(ctx, ctx.getPosition(declnode.Pos())))
		} else {
			ctx.out.object(obj)
			describeType(ctx, "type:", obj.Type())

			ctx.out.pos(pos2str(ctx, ctx.getPosition(obj.Pos())))
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
				describeTypeContents(&ctx.out, typeOfExpr.Type, node.Sel.String())
				return
			}
			ctx.out.err(fmt.Sprintf("unknown selector expression %s", printerSprint(ctx.getFileSet(node.Pos()), node)))
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
				ctx.out.funcHeader(ctx.getFileSet(declnode.Pos()), declnode)
				fallbackdescr = false
			}
		}

		if fallbackdescr {
			ctx.out.selector(sel.Kind(), printerSprint(ctx.getFileSet(node.Pos()), node))

			describeType(ctx, "receiver:", sel.Recv())
			describeType(ctx, "type:", sel.Type())
		}

		ctx.out.pos(pos2str(ctx, pos))

	case ast.Expr:
		typeAndVal := pkg.TypesInfo.Types[node]
		describeType(ctx, "type:", typeAndVal.Type)
	}
}

func printerSprint(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	printer.Fprint(&buf, fset, node)
	return buf.String()
}

func describeDeclaration(ctx *context, declnode ast.Node, typ types.Type) {
	normaldescr := true

	switch declnode := declnode.(type) {
	case *ast.FuncDecl:
		ctx.out.funcHeader(ctx.getFileSet(declnode.Pos()), declnode)
		normaldescr = false
	default:
	}

	if normaldescr {
		describeType(ctx, "type:", typ)
	}
}

func describeType(ctx *context, prefix string, typ types.Type) {
	typstr := printTypesTypeNice(typ)
	if ptyp, isptr := typ.(*types.Pointer); isptr {
		typ = ptyp.Elem()
	}
	ntyp, isnamed := typ.(*types.Named)
	if !isnamed {
		ctx.out.typ(prefix, typstr, "")
		return
	}
	obj := ntyp.Obj()
	if obj == nil {
		ctx.out.typ(prefix, typstr, "")
		return
	}
	pos := ctx.getPosition(obj.Pos())
	ctx.out.typ(prefix, typstr, pos2str(ctx, pos))
}

func pos2str(ctx *context, pos token.Position) string {
	filename := pos.Filename
	filename = replaceGoroot(ctx, filename)
	return fmt.Sprintf("%s:%d", filename, pos.Line)
}

func replaceGoroot(ctx *context, filename string) string {
	const gorootPrefix = "$GOROOT"
	if strings.HasPrefix(filename, gorootPrefix) {
		filename = ctx.Goroot() + filename[len(gorootPrefix):]
	}
	return filename
}

func describeTypeContents(descr *Description, typ types.Type, prefix string) {
	if prefix == "_" {
		prefix = ""
	}
	if ptyp, isptr := typ.(*types.Pointer); isptr {
		typ = ptyp.Elem()
	}

	out := bytes.NewBuffer(make([]byte, 0))

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

	descr.typeContents(out.String())
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
			pkgs2, err := packages.Load(decorateConfig(ctx, &packages.Config{
				Mode:      packages.LoadSyntax,
				Dir:       ctx.Wd,
				Fset:      pkg.Fset,
				ParseFile: ctx.parseFile()}), pkg.PkgPath)
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

//go:generate stringer -type InfoKind

type Description []Info

type Info struct {
	Kind InfoKind
	Text string
	Pos  string
}

type InfoKind uint8

const (
	InfoErr InfoKind = iota
	InfoObject
	InfoSelection
	InfoFunction
	InfoType
	InfoTypeContents
	InfoPos
)

func (descr Description) writeTo(out io.Writer) {
	for _, info := range descr {
		info.writeTo(out)
	}
}

func (descr *Description) err(fmtstr string, args ...interface{}) {
	*descr = append(*descr, Info{Kind: InfoErr, Text: fmt.Sprintf(fmtstr, args...)})
}

func (descr *Description) object(obj types.Object) {
	*descr = append(*descr, Info{Kind: InfoObject, Text: fmt.Sprintf("%s", obj)})
}

func (descr *Description) selector(kind types.SelectionKind, expr string) {
	kindstr := "unknown selector"
	switch kind {
	case types.FieldVal:
		kindstr = "struct field"
	case types.MethodVal:
		kindstr = "method"
	case types.MethodExpr:
		kindstr = "method expression"
	}

	*descr = append(*descr, Info{Kind: InfoSelection, Text: fmt.Sprintf("%s %s", kindstr, expr)})
}

func (descr *Description) funcHeader(fset *token.FileSet, declnode *ast.FuncDecl) {
	body := declnode.Body
	declnode.Body = nil
	var buf bytes.Buffer
	printer.Fprint(&buf, fset, declnode)
	*descr = append(*descr, Info{Kind: InfoFunction, Text: buf.String()})
	declnode.Body = body
}

func (descr *Description) typ(prefix, typeDescr, pos string) {
	*descr = append(*descr, Info{Kind: InfoType, Text: fmt.Sprintf("%s %s", prefix, typeDescr), Pos: pos})
}

func (descr *Description) typeContents(contents string) {
	*descr = append(*descr, Info{Kind: InfoTypeContents, Text: contents})
}

func (descr *Description) pos(pos string) {
	*descr = append(*descr, Info{Kind: InfoPos, Pos: pos})
}

func (info *Info) writeTo(out io.Writer) {
	switch info.Kind {
	case InfoErr, InfoObject, InfoSelection, InfoFunction:
		out.Write([]byte(info.Text))
		out.Write([]byte("\n"))

	case InfoTypeContents:
		out.Write([]byte(info.Text))

	case InfoType:
		out.Write([]byte(info.Text))
		out.Write([]byte("\n"))
		if info.Pos != "" {
			out.Write([]byte("\t"))
			out.Write([]byte(info.Pos))
			out.Write([]byte("\n"))
		}
	case InfoPos:
		out.Write([]byte("\n"))
		out.Write([]byte(info.Pos))
		out.Write([]byte("\n"))
	}
}
