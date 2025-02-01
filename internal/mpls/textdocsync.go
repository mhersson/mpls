package mpls

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/mhersson/mpls/internal/previewserver"
	"github.com/mhersson/mpls/pkg/parser"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

var (
	content       string
	currentURI    string
	filename      string
	previewServer *previewserver.Server
)

func TextDocumentDidOpen(context *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	currentURI = params.TextDocument.URI
	filename = filepath.Base(currentURI)

	doc := params.TextDocument

	_ = protocol.Trace(context, protocol.MessageTypeInfo, log("TextDocumentDidOpen: "+doc.URI))

	content = doc.Text

	// Give the browser time to connect
	if err := previewserver.WaitForClients(10 * time.Second); err != nil {
		return err
	}

	html, meta := parser.HTML(content)
	previewServer.Update(filename, html, "", meta)

	return nil
}

func TextDocumentDidChange(context *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	var err error

	for _, change := range params.ContentChanges {
		if c, ok := change.(protocol.TextDocumentContentChangeEvent); ok {
			if params.TextDocument.URI != currentURI {
				_ = protocol.Trace(context, protocol.MessageTypeInfo,
					log("TextDocumentUriDidChange - switching document: "+params.TextDocument.URI))

				content, err = loadDocument(params.TextDocument.URI)
				if err != nil {
					return err
				}

				currentURI = params.TextDocument.URI
				filename = filepath.Base(currentURI)
			}

			startIndex, endIndex := c.Range.IndexesIn(content)
			content = content[:startIndex] + c.Text + content[endIndex:]

			currentSection := findSection(content, startIndex)
			html, meta := parser.HTML(content)

			previewServer.Update(filename, html, currentSection, meta)
		} else if c, ok := change.(protocol.TextDocumentContentChangeEventWhole); ok {
			html, meta := parser.HTML(c.Text)
			previewServer.Update(filename, html, "", meta)
		}
	}

	return nil
}

func TextDocumentDidSave(_ *glsp.Context, params *protocol.DidSaveTextDocumentParams) error {
	var err error

	content, err = loadDocument(params.TextDocument.URI)
	if err != nil {
		return err
	}

	html, meta := parser.HTML(content)
	previewServer.Update(filename, html, "", meta)

	return nil
}

func TextDocumentDidClose(_ *glsp.Context, _ *protocol.DidCloseTextDocumentParams) error {
	return nil
}

func loadDocument(uri string) (string, error) {
	f := strings.TrimPrefix(uri, "file://")

	if runtime.GOOS == "windows" {
		f = strings.TrimPrefix(uri, "file:///")
		f = filepath.FromSlash(f)
		f = strings.Replace(f, "%3A", ":", 1)
		f = strings.ReplaceAll(f, "%20", " ")
	}

	c, err := os.ReadFile(f)
	if err != nil {
		return "", err
	}

	return string(c), nil
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

	re := regexp.MustCompile(`[^a-z0-9&]+`)
	section = re.ReplaceAllString(section, "-")

	section = strings.ReplaceAll(section, "&", "")

	section = strings.Trim(section, "-")

	if len(section) > 0 && !unicode.IsLetter(rune(section[0])) {
		section = "id-" + section
	}

	return section
}
