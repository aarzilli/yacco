package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aarzilli/yacco/buf"
	"github.com/aarzilli/yacco/util"

	"github.com/sourcegraph/jsonrpc2"
)

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
		lspConns[wd].conn.Close()
	}
}

func LspFor(lang, wd string, create bool) *LspSrv {
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

	cmd := exec.Command("gopls", "-logfile=/tmp/gopls.log", "serve")
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

	srv := &LspSrv{client, out.Capabilities.InnerServerCapabilities.DefinitionProvider, out.Capabilities.TypeDefinitionServerCapabilities.TypeDefinitionProvider}
	lspConns[wd] = srv
	return srv
}

type LspBufferPos struct {
	Path      string
	Ln, Col   int
	VersionId int
	Contents  string
}

func (srv *LspSrv) Describe(a LspBufferPos) string {
	if a.Contents != "" {
		srv.Changed(a)
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(60*time.Second))
	defer cancel()
	//ctx := context.Background()

	tp := &TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{
			URI: "file://" + a.Path,
		},
		Position: Position{a.Ln, a.Col},
	}

	var hover Hover
	srv.conn.Call(ctx, "textDocument/hover", tp, &hover)
	if hover.Contents.Value != "" {
		if !srv.definitionProvider {
			return hover.Contents.Value
		}
		var def, typeDef []Location
		if srv.definitionProvider {
			srv.conn.Call(ctx, "textDocument/definition", tp, &def)
		}
		if srv.typeDefinitionProvider {
			srv.conn.Call(ctx, "textDocument/typeDefinition", tp, &typeDef)
		}
		s := hover.Contents.Value
		def = append(def, typeDef...)
		s = appendLocs(s, def, a.Path, a.Ln)
		return s
	}

	var sign SignatureHelp
	srv.conn.Call(context.Background(), "textDocument/signatureHelp", tp, &sign)
	if len(sign.Signatures) == 0 {
		return ""
	}
	return sign.Signatures[0].Label
}

func appendLocs(s string, defs []Location, curPath string, ln int) string {
	const sillyURI = "file://"
	if len(defs) <= 0 {
		return s
	}

	strdefs := []string{}

	var lastDef string
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
		if defstr == lastDef {
			continue
		}
		strdefs = append(strdefs, defstr)
		lastDef = defstr
	}
	if len(strdefs) <= 0 {
		return s
	}
	return s + "\n\n" + strings.Join(strdefs, "\n")
}

func (srv *LspSrv) Complete(a LspBufferPos) ([]string, string) {
	if a.Contents != "" {
		srv.Changed(a)
	}

	first := true
	insertPrefix := ""

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(60*time.Second))
	defer cancel()
	//ctx := context.Background()

	tp := &TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{
			URI: "file://" + a.Path,
		},
		Position: Position{a.Ln, a.Col},
	}

	var cmpl CompletionList
	srv.conn.Call(ctx, "textDocument/completion", tp, &cmpl)
	r := make([]string, 0, len(cmpl.Items))
	for _, cmplItem := range cmpl.Items {
		if cmplItem.TextEdit == nil {
			continue
		}
		r = append(r, cmplItem.Label)
		if first {
			first = false
			insertPrefix = cmplItem.TextEdit.NewText
		} else {
			insertPrefix = commonPrefix2(insertPrefix, cmplItem.TextEdit.NewText)
		}
	}

	return r, insertPrefix
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
	srv.conn.Notify(context.Background(), "textDocument/didChange", DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     "file://" + a.Path,
			Version: float64(a.VersionId),
		},
		ContentChanges: []TextDocumentContentChangeEvent{
			TextDocumentContentChangeEvent{
				Text: a.Contents,
			},
		},
	})
}

type lspHandler struct {
}

func (h *lspHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	if req.Method != "workspace/configuration" {
		return
	}
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
}

type LspSrv struct {
	conn                   *jsonrpc2.Conn
	definitionProvider     bool
	typeDefinitionProvider bool
}

func BufferToLsp(wd string, b *buf.Buffer, sel util.Sel, createLsp bool) (*LspSrv, LspBufferPos) {
	srv := LspFor(filepath.Ext(b.Name), wd, createLsp)
	if srv == nil {
		return nil, LspBufferPos{}
	}

	var changed string
	if b.Modified {
		changed = string(b.SelectionRunes(util.Sel{0, b.Size()}))
	}

	ln, col := b.GetLine(sel.S, true)

	return srv, LspBufferPos{b.Path(), ln - 1, col, b.RevCount, changed}
}
