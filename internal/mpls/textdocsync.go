package mpls

import (
	"fmt"
	"os"
	"strings"

	"github.com/mhersson/mpls/internal/previewserver"
	"github.com/mhersson/mpls/pkg/parser"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

var previewServer *previewserver.Server
var document string
var currentURI string

func TextDocumentDidOpen(_ *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	currentURI = params.TextDocument.URI
	doc := params.TextDocument

	fmt.Fprintf(os.Stderr, "TextDocumentDidOpen: %s\n", doc.URI)

	document = doc.Text

	html := parser.HTML(document)
	previewServer.Update(html)

	return nil
}

func TextDocumentDidChange(_ *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	for _, change := range params.ContentChanges {
		if c, ok := change.(protocol.TextDocumentContentChangeEvent); ok {
			if params.TextDocument.URI != currentURI {
				fmt.Fprintf(os.Stderr, "TextDocumentUriDidChange - switching document: %s\n", params.TextDocument.URI)
				document = string(loadDocument(params.TextDocument.URI))
				currentURI = params.TextDocument.URI
			}

			startIndex, endIndex := c.Range.IndexesIn(document)
			document = document[:startIndex] + c.Text + document[endIndex:]

			html := parser.HTML(document)
			previewServer.Update(html)
		} else if c, ok := change.(protocol.TextDocumentContentChangeEventWhole); ok {
			html := parser.HTML(c.Text)
			previewServer.Update(html)
		}
	}

	return nil
}

func TextDocumentDidSave(_ *glsp.Context, params *protocol.DidSaveTextDocumentParams) error {
	document = string(loadDocument(params.TextDocument.URI))

	html := parser.HTML(document)
	previewServer.Update(html)

	return nil
}

func TextDocumentDidClose(_ *glsp.Context, _ *protocol.DidCloseTextDocumentParams) error {
	return nil
}

func loadDocument(uri string) []byte {
	c, _ := os.ReadFile(strings.TrimPrefix(uri, "file://"))

	return c
}
