package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	_ "os/exec"

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

func main() {
	/*
		cmd := exec.Command("gopls", "serve")
		stdin, err := cmd.StdinPipe()
		must(err)
		stdout, err := cmd.StdoutPipe()
		must(err)
		stderr, err := cmd.StderrPipe()
		must(err)
		go io.Copy(os.Stdout, stderr)
		must(cmd.Start())
		go func() {
			err := cmd.Wait()
			fmt.Printf("process exited %v\n", err)
		}()


		stream := jsonrpc2.NewBufferedStream(&readerWriter{ stdout, &writerLogger{ stdin } }, &jsonrpc2.VSCodeObjectCodec{})*/

	conn, err := net.Dial("tcp", "127.0.0.1:8080")
	must(err)
	stream := jsonrpc2.NewBufferedStream(conn, &jsonrpc2.VSCodeObjectCodec{})

	client := jsonrpc2.NewConn(context.Background(), stream, nil)
	var out InitializeResult
	wd, err := os.Getwd()
	must(err)

	tdcc := &TextDocumentClientCapabilities{}
	tdcc.Synchronization.DidSave = true // this is fucking impossible to instantiate!
	tdcc.Completion.CompletionItem.DocumentationFormat = []MarkupKind{"plaintext"}
	tdcc.Hover.ContentFormat = []MarkupKind{"plaintext"}
	tdcc.SignatureHelp.SignatureInformation.DocumentationFormat = []MarkupKind{"plaintext"}

	client.Call(context.Background(), "initialize", &InitializeParams{
		InnerInitializeParams{
			ProcessID: os.Getpid(),
			RootPath:  wd,
			RootURI:   "file://" + wd,
			Capabilities: ClientCapabilities{
				InnerClientCapabilities: InnerClientCapabilities{
					TextDocument: tdcc,
				},
			},
		},
		WorkspaceFoldersInitializeParams{}}, &out)
	fmt.Printf("%s\n", tojson(out))
	lspCompletionChars := []string{}
	if out.Capabilities.InnerServerCapabilities.CompletionProvider != nil {
		for _, s := range out.Capabilities.InnerServerCapabilities.CompletionProvider.TriggerCharacters {
			lspCompletionChars = append(lspCompletionChars, s)
		}
	}

	/*
		var sign SignatureHelp
		client.Call(context.Background(), "textDocument/signatureHelp", &TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{
				URI: "file://" + wd + "/lsp.go",
			},
			Position: Position{
				Line: 56, // zero based
				Character: 16,
			},
		}, &sign)
		fmt.Printf("%s\n", tojson(sign)) // sign.Signatures[0].Label is good

		// only send if definitionProvider is set in itialization
		var def Location
		client.Call(context.Background(), "textDocument/definition", &TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{
				URI: "file://" + wd + "/lsp.go",
			},
			Position: Position{
				Line: 56, // zero based
				Character: 16,
			},
		}, &def)
		fmt.Printf("definition: %s\n", tojson(def))

		// only send if typeDefinitionProvider is set in initialization
		var typeDef Location
		client.Call(context.Background(), "textDocument/typeDefinition", &TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{
				URI: "file://" + wd + "/lsp.go",
			},
			Position: Position{
				Line: 56, // zero based
				Character: 16,
			},
		}, &typeDef)
		fmt.Printf("typeDefinition: %s\n", tojson(typeDef))
	*/

	/*
		pos := Position{
				Line: 56, // zero based
				Character: 16,
			},*/

	pos := Position{
		Line:      75,
		Character: 20,
	}

	// only send if hoverProvider is set in initialization
	var hover Hover
	client.Call(context.Background(), "textDocument/hover", &TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{
			URI: "file://" + wd + "/lsp.go",
		},
		Position: pos,
	}, &hover)
	fmt.Printf("hover: %s\n", tojson(hover)) // this is unfortunately missing godoc, if it wasn't it would be fine

	// /home/a/n/go/pkg/mod/golang.org/x/tools@v0.0.0-20190228203856-589c23e65e65/internal/lsp/source/definition.go:85

	// 56:16 (exec.Command)
}

/*
- textDocument/signatureHelp, textDocument/declaration, textDocument/typeDefinition, textDocument/hover
- textDocument/didChange
- workspace/didChangeConfiguration? can we use it to set the configuration?
*/
