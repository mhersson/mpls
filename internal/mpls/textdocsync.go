package mpls

import (
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/mhersson/mpls/internal/previewserver"
	"github.com/mhersson/mpls/pkg/parser"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

var previewServer *previewserver.Server
var document string
var currentURI string

func TextDocumentDidOpen(context *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	currentURI = params.TextDocument.URI
	doc := params.TextDocument

	_ = protocol.Trace(context, protocol.MessageTypeInfo, log("TextDocumentDidOpen: "+doc.URI))

	document = doc.Text

	html := parser.HTML(document)
	previewServer.Update(html, "")

	return nil
}

func TextDocumentDidChange(context *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	for _, change := range params.ContentChanges {
		if c, ok := change.(protocol.TextDocumentContentChangeEvent); ok {
			if params.TextDocument.URI != currentURI {
				_ = protocol.Trace(context, protocol.MessageTypeInfo,
					log("TextDocumentUriDidChange - switching document: "+params.TextDocument.URI))
				document = string(loadDocument(params.TextDocument.URI))
				currentURI = params.TextDocument.URI
			}

			startIndex, endIndex := c.Range.IndexesIn(document)
			document = document[:startIndex] + c.Text + document[endIndex:]

			currentSection := findSection(document, startIndex)
			html := parser.HTML(document)

			previewServer.Update(html, currentSection)
		} else if c, ok := change.(protocol.TextDocumentContentChangeEventWhole); ok {
			html := parser.HTML(c.Text)
			previewServer.Update(html, "")
		}
	}

	return nil
}

func TextDocumentDidSave(_ *glsp.Context, params *protocol.DidSaveTextDocumentParams) error {
	document = string(loadDocument(params.TextDocument.URI))

	html := parser.HTML(document)
	previewServer.Update(html, "")

	return nil
}

func TextDocumentDidClose(_ *glsp.Context, _ *protocol.DidCloseTextDocumentParams) error {
	return nil
}

func loadDocument(uri string) []byte {
	c, _ := os.ReadFile(strings.TrimPrefix(uri, "file://"))

	return c
}

// Find the closest section heading.
func findSection(document string, index int) string {
	section := ""
	start := 0

	for {
		end := strings.Index(document[start:], "\n")
		if end == -1 {
			end = len(document)
		} else {
			end += start
		}

		line := document[start:end]
		if strings.HasPrefix(line, "#") && start <= index {
			section = line
		}
		if end >= len(document) || start > index {
			break
		}

		start = end + 1
	}

	return formatSection(section)
}

func formatSection(section string) string {
	section = strings.ToLower(section)

	re := regexp.MustCompile(`[^a-z0-9]+`)
	section = re.ReplaceAllString(section, "-")

	section = strings.Trim(section, "-")

	if len(section) > 0 && !unicode.IsLetter(rune(section[0])) {
		section = "id-" + section
	}

	return section
}
