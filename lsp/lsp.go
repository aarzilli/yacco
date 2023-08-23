package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/aarzilli/yacco/buf"
	"github.com/aarzilli/yacco/util"

	"github.com/sourcegraph/jsonrpc2"
)

const logToStdout = false

func must(err error) {
	if err != nil {
		panic(err)
	}
}

type readerWriter struct {
	io.ReadCloser
	io.WriteCloser
}

func (rw *readerWriter) Close() error {
	err1 := rw.ReadCloser.Close()
	err2 := rw.WriteCloser.Close()
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}

type writerLogger struct {
	wr io.WriteCloser
}

func (w *writerLogger) Close() error {
	return w.wr.Close()
}

func (w *writerLogger) Write(buf []byte) (int, error) {
	fmt.Fprintf(os.Stderr, "<- %s\n", string(buf))
	return w.wr.Write(buf)
}

func tojson(v interface{}) string {
	buf, err := json.Marshal(v)
	must(err)
	return string(buf)
}

var lspConns = map[string]*LspSrv{}
var lspMu sync.Mutex

const debug = false

func Restart(wd string) {
	if lspConns[wd] != nil {
		log.Reset()
		lspConns[wd].conn.Close()
		lspConns[wd] = nil
	}
}

func Killall() {
	for wd := range lspConns {
		lspConns[wd].conn.Close()
		lspConns[wd] = nil
	}
	log.Reset()
}

func LspFor(lang, wd string, create bool, warn func(string)) *LspSrv {
	if lang != ".go" {
		return nil
	}

	lspMu.Lock()
	defer lspMu.Unlock()

	if _, ok := lspConns[wd]; ok {
		return lspConns[wd]
	}

	if !create {
		return nil
	}

	cmd := exec.Command("gopls", "serve")
	stdin, err := cmd.StdinPipe()
	must(err)
	stdout, err := cmd.StdoutPipe()
	must(err)
	stderr, err := cmd.StderrPipe()
	must(err)
	go io.Copy(os.Stdout, stderr)
	err = cmd.Start()
	if err != nil {
		lspConns[wd] = nil
		return nil
	}
	go func() {
		cmd.Wait()
		lspMu.Lock()
		defer lspMu.Unlock()
		delete(lspConns, wd)
	}()

	if debug {
		stdin = &writerLogger{stdin}
	}

	stream := jsonrpc2.NewBufferedStream(&readerWriter{stdout, stdin}, &jsonrpc2.VSCodeObjectCodec{})

	handler := &lspHandler{}

	client := jsonrpc2.NewConn(context.Background(), stream, handler)
	var out InitializeResult

	tdcc := &TextDocumentClientCapabilities{}
	tdcc.Synchronization.DidSave = true // this is fucking impossible to instantiate!
	tdcc.Completion.CompletionItem.DocumentationFormat = []MarkupKind{"plaintext"}
	tdcc.Hover.ContentFormat = []MarkupKind{"plaintext"}
	tdcc.SignatureHelp.SignatureInformation.DocumentationFormat = []MarkupKind{"plaintext"}
	tdcc.CodeAction = &CodeActionClientCapabilities{
		DataSupport: true,
	}

	wcc := &WorkspaceClientCapabilities{}
	wcc.Configuration = true

	client.Call(context.Background(), "initialize", &InitializeParams{
		InnerInitializeParams{
			ProcessID: os.Getpid(),
			RootPath:  wd,
			RootURI:   "file://" + wd,
			Capabilities: ClientCapabilities{
				TextDocument: tdcc,
				Workspace:    wcc,
			},
		},
		WorkspaceFoldersInitializeParams{}}, &out)

	client.Notify(context.Background(), "initialized", &InitializedParams{})

	srv := &LspSrv{client, warn, out.Capabilities, make(map[string]int), nil, nil}
	handler.srv = srv
	lspConns[wd] = srv
	return srv
}

type LspBufferPos struct {
	Path    string
	Ln, Col int
	b       *buf.Buffer
	line    []uint16

	EndLn, EndCol int
}

func (a *LspBufferPos) tdpp() *TextDocumentPositionParams {
	return &TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{
			URI: "file://" + a.Path,
		},
		Position: Position{a.Ln, a.Col},
	}
}

type locationAndKind struct {
	Location
	kind string
}

func (srv *LspSrv) Describe(a LspBufferPos) string {
	const (
		linkPerKindMax = 2
		hoverLen       = 10
		nothing        = `(nothing)

Lsp restart
`
	)
	srv.Changed(a)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(60*time.Second))
	defer cancel()

	srv.getCodeActionsList(ctx, a)

	tp := a.tdpp()

	linestr := string(utf16.Decode(a.line))
	if a.Col > 0 && a.Col < len(linestr) && linestr[a.Col] == '.' {
		// autocompletions
		var cmpl CompletionList
		srv.conn.Call(ctx, "textDocument/completion", tp, &cmpl)
		out := new(strings.Builder)
		for _, cmplItem := range cmpl.Items {
			if cmplItem.TextEdit == nil {
				continue
			}
			if cmplItem.TextEdit.Range.Start.Line != cmplItem.TextEdit.Range.End.Line {
				continue
			}
			if cmplItem.TextEdit.Range.Start.Line != a.Ln {
				continue
			}
			if a.Col != cmplItem.TextEdit.Range.Start.Character {
				continue
			}
			fmt.Fprintln(out, "."+cmplItem.TextEdit.NewText)
		}
		return srv.appendCodeActions(out.String())
	}

	var hover Hover
	srv.conn.Call(ctx, "textDocument/hover", tp, &hover)
	if hover.Contents.Value != "" {
		var def []locationAndKind

		getlocs := func(kind, hkind string) {
			var def2 []locationAndKind
			srv.conn.Call(ctx, kind, tp, &def2)
			for i := range def2 {
				def2[i].kind = hkind
			}
			if len(def2) > linkPerKindMax {
				def2 = def2[:linkPerKindMax]
			}
			def = append(def, def2...)
		}
		if srv.Capabilities.DefinitionProvider {
			getlocs("textDocument/definition", "")
		}
		if srv.Capabilities.TypeDefinitionServerCapabilities.TypeDefinitionProvider {
			getlocs("textDocument/typeDefinition", "Type:")
		}
		if srv.Capabilities.DeclarationProvider {
			getlocs("textDocument/declaration", "Declaration:")
		}
		if srv.Capabilities.ImplementationProvider {
			getlocs("textDocument/implementation", "Implementation:")
		}

		lines := strings.Split(hover.Contents.Value, "\n")
		if len(lines) > hoverLen {
			lines = lines[:hoverLen]
			lines = append(lines, "...")
		}
		s := strings.Join(lines, "\n")
		s = appendLocs(s, def, a.Path, a.Ln)
		s += "\nLsp refs\nPrepare Lsp rename"
		lspLog(fmt.Sprint("hover for ", a, " got ", len(lines), "\n"))
		return srv.appendCodeActions(s)
	}

	var sign SignatureHelp
	srv.conn.Call(context.Background(), "textDocument/signatureHelp", tp, &sign)
	if len(sign.Signatures) == 0 {
		return srv.appendCodeActions(nothing)
	}
	lspLog(fmt.Sprint("no hover for", a, "and signature help len is", len(sign.Signatures[0].Label), "\n"))
	if sign.Signatures[0].Label != "" {
		return srv.appendCodeActions(sign.Signatures[0].Label)
	}
	return srv.appendCodeActions(nothing)
}

func (srv *LspSrv) References(a LspBufferPos) string {
	srv.Changed(a)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(60*time.Second))
	defer cancel()
	//ctx := context.Background()

	tp := a.tdpp()
	var locs []locationAndKind
	srv.conn.Call(ctx, "textDocument/references", tp, &locs)
	return appendLocs("", locs, a.Path, a.Ln)
}

func appendLocs(s string, defs []locationAndKind, curPath string, ln int) string {
	const sillyURI = "file://"
	if len(defs) <= 0 {
		return s
	}

	strdefs := []string{}
	seen := map[string]bool{}

	for _, def := range defs {
		if def.URI == "" {
			continue
		}
		path := def.URI
		if strings.HasPrefix(path, sillyURI) {
			path = path[len(sillyURI):]
		}
		if path == curPath && def.Range.Start.Line == ln {
			continue
		}
		defstr := fmt.Sprintf("%s:%d", path, def.Range.Start.Line+1)
		if seen[defstr] {
			continue
		}
		seen[defstr] = true
		if def.kind != "" {
			defstr = def.kind + " " + defstr
		}
		strdefs = append(strdefs, defstr)
	}
	if len(strdefs) <= 0 {
		return s
	}
	if s == "" {
		return strings.Join(strdefs, "\n")
	}
	return s + "\n\n" + strings.Join(strdefs, "\n")
}

func (srv *LspSrv) Complete(a LspBufferPos) ([]string, string) {
	srv.Changed(a)

	first := true
	insertPrefix := ""

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(60*time.Second))
	defer cancel()
	//ctx := context.Background()

	tp := a.tdpp()

	var cmpl CompletionList
	srv.conn.Call(ctx, "textDocument/completion", tp, &cmpl)
	r := make([]string, 0, len(cmpl.Items))
	for _, cmplItem := range cmpl.Items {
		if cmplItem.TextEdit == nil {
			continue
		}
		if cmplItem.TextEdit.Range.Start.Line != cmplItem.TextEdit.Range.End.Line {
			continue
		}
		if cmplItem.TextEdit.Range.Start.Line != a.Ln {
			continue
		}

		nt := utf16.Encode([]rune(cmplItem.TextEdit.NewText))
		commonidx := a.Col - cmplItem.TextEdit.Range.Start.Character
		if commonidx < 0 || commonidx > len(nt) {
			continue
		}

		if !issfx(nt[:commonidx], a.line) {
			continue
		}

		nt = nt[commonidx:]

		r = append(r, cmplItem.TextEdit.NewText)
		if first {
			first = false
			insertPrefix = string(utf16.Decode(nt))
		} else {
			insertPrefix = commonPrefix2(insertPrefix, string(utf16.Decode(nt)))
		}
	}

	return r, insertPrefix
}

func (srv *LspSrv) Rename(a LspBufferPos, to string) []TextDocumentEdit {
	srv.Changed(a)
	tp := a.tdpp()
	renameOpts := RenameParams{
		TextDocument: tp.TextDocument,
		Position:     tp.Position,
		NewName:      to,
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(60*time.Second))
	defer cancel()
	var we WorkspaceEdit
	err := srv.conn.Call(ctx, "textDocument/rename", renameOpts, &we)
	if err != nil {
		srv.warn(err.Error())
	}
	return we.DocumentChanges
}

func (srv *LspSrv) getCodeActionsList(ctx context.Context, a LspBufferPos) {
	srv.codeActions = nil
	cap := &CodeActionParams{
		TextDocument: TextDocumentIdentifier{
			URI: "file://" + a.Path,
		},
		Range: Range{
			Start: Position{a.Ln, a.Col},
			End:   Position{a.EndLn, a.EndCol},
		},
	}
	var actions []CodeAction
	srv.conn.Call(ctx, "textDocument/codeAction", cap, &actions)
	for i := range actions {
		if actions[i].Edit == nil || (actions[i].Edit.Changes == nil && actions[i].Edit.DocumentChanges == nil) {
			srv.codeActions = append(srv.codeActions, actions[i])
		}
	}
}

func (srv *LspSrv) appendCodeActions(s string) string {
	if len(srv.codeActions) == 0 {
		return s
	}
	out := new(strings.Builder)
	for i := range srv.codeActions {
		fmt.Fprintf(out, "Lsp ca %d %s\n", i, srv.codeActions[i].Title)
	}
	return s + "\n" + out.String()
}

func (srv *LspSrv) ExecCodeAction(a LspBufferPos, rest string, execEdit func([]TextDocumentEdit)) {
	arg, _, _ := strings.Cut(rest, " ")
	i, _ := strconv.Atoi(arg)
	if i < 0 || i > len(srv.codeActions) {
		return
	}
	action := srv.codeActions[i]
	//TODO: execute action edits (must also remove the filtering above)
	cmd := action.Command
	srv.applyEdits = execEdit
	defer func() { srv.applyEdits = nil }()
	var out any
	srv.conn.Call(context.Background(), "workspace/executeCommand", &ExecuteCommandParams{Command: cmd.Command, Arguments: cmd.Arguments}, out)
}

func issfx(a, b []uint16) bool {
	for i, j := len(a)-1, len(b)-1; i >= 0 && j >= 0; i, j = i-1, j-1 {
		if a[i] != b[j] {
			return false
		}
	}
	return true
}

func commonPrefix2(a, b string) string {
	l := len(a)
	if l > len(b) {
		l = len(b)
	}
	for i := 0; i < l; i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a[:l]
}

func (srv *LspSrv) Changed(a LspBufferPos) {
	if srv.revision[a.Path] == a.b.RevCount {
		return
	}
	if _, ok := srv.revision[a.Path]; !ok {
		srv.conn.Notify(context.Background(), "textDocument/didOpen", DidOpenTextDocumentParams{
			TextDocument: TextDocumentItem{URI: "file://" + a.Path, Version: 0},
		})
	}
	srv.revision[a.Path] = a.b.RevCount
	srv.conn.Notify(context.Background(), "textDocument/didChange", DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     "file://" + a.Path,
			Version: float64(a.b.RevCount),
		},
		ContentChanges: []TextDocumentContentChangeEvent{
			TextDocumentContentChangeEvent{
				Text: string(a.b.SelectionRunes(util.Sel{0, a.b.Size()})),
			},
		},
	})
}

type lspHandler struct {
	srv *LspSrv
}

var logMessageType = map[MessageType]string{
	1: "ERROR ",
	2: "WARN ",
	3: "INFO ",
	4: "LOG ",
}

func (h *lspHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	switch req.Method {
	case "workspace/configuration":
		var params ConfigurationParams
		must(json.Unmarshal(*req.Params, &params))

		v := []map[string]interface{}{
			map[string]interface{}{
				"enhancedHover": true,
			},
		}

		var respJson json.RawMessage
		respJson, err := json.Marshal(v)
		must(err)

		var r jsonrpc2.Response
		r.ID = req.ID
		r.Result = &respJson
		conn.SendResponse(ctx, &r)

	case "window/logMessage", "window/showMessage":
		var params ShowMessageParams
		must(json.Unmarshal(*req.Params, &params))
		lspLog(logMessageType[params.Type])
		lspLog(params.Message)
		if params.Message != "" && params.Message[len(params.Message)-1] != '\n' {
			lspLog("\n")
		}

	case "textDocument/publishDiagnostics":
		// not interesting

	case "workspace/applyEdit":
		if h.srv.applyEdits == nil {
			lspLog("workspace/applyEdit request rejected\n")
			conn.SendResponse(ctx, &jsonrpc2.Response{ID: req.ID, Error: &jsonrpc2.Error{Code: 500, Message: "unsupported now"}})
			return
		}
		var params ApplyWorkspaceEditParams
		must(json.Unmarshal(*req.Params, &params))
		if params.Edit.Changes != nil && len(*params.Edit.Changes) > 0 {
			lspLog("workspace/applyEdit request rejected (Changes field)\n")
			conn.SendResponse(ctx, &jsonrpc2.Response{ID: req.ID, Error: &jsonrpc2.Error{Code: 500, Message: "unsupported changes field"}})
			return
		}
		for i := range params.Edit.DocumentChanges {
			lspLog(fmt.Sprintf("applying edit %#v\n", params.Edit.DocumentChanges[i]))
		}
		h.srv.applyEdits(params.Edit.DocumentChanges)
		conn.Reply(ctx, req.ID, &ApplyWorkspaceEditResponse{Applied: true})

	default:
		buf, _ := json.Marshal(req)
		lspLog(string(buf))
		lspLog("\n")
	}
}

type LspSrv struct {
	conn         *jsonrpc2.Conn
	warn         func(string)
	Capabilities ServerCapabilities
	revision     map[string]int
	codeActions  []CodeAction
	applyEdits   func([]TextDocumentEdit)
}

func BufferToLsp(wd string, b *buf.Buffer, sel util.Sel, createLsp bool, warn func(string)) (*LspSrv, LspBufferPos) {
	srv := LspFor(filepath.Ext(b.Name), wd, createLsp, warn)
	if srv == nil {
		return nil, LspBufferPos{}
	}

	linestr, ln, col := b.GetLine(sel.S, true)
	_, endln, endcol := b.GetLine(sel.E, true)

	return srv, LspBufferPos{
		Path:   b.Path(),
		Ln:     ln - 1,
		Col:    col,
		b:      b,
		line:   utf16.Encode([]rune(linestr)),
		EndLn:  endln,
		EndCol: endcol,
	}
}

var log strings.Builder

func lspLog(s string) {
	if logToStdout {
		os.Stderr.WriteString(s)
	}
	log.WriteString(s)
}

func GetLog() string {
	return log.String()
}
