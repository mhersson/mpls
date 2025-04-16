package mpls

import (
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/mhersson/glsp"
	protocol "github.com/mhersson/glsp/protocol_3_16"
	"github.com/mhersson/mpls/internal/previewserver"
	"github.com/mhersson/mpls/pkg/parser"
	"github.com/mhersson/mpls/pkg/plantuml"
)

var (
	content       string
	currentURI    string
	filename      string
	previewServer *previewserver.Server
	plantumls     []plantuml.Plantuml
)

func TextDocumentDidOpen(ctx *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	var err error
	currentURI = params.TextDocument.URI
	filename = filepath.Base(currentURI)
	plantumls = []plantuml.Plantuml{}

	_ = protocol.Trace(ctx, protocol.MessageTypeInfo, log("TextDocumentDidOpen: "+params.TextDocument.URI))

	if !previewserver.OpenBrowserOnStartup && content == "" {
		return nil
	}

	doc := params.TextDocument

	content = doc.Text

	// Give the browser time to connect
	if err = previewserver.WaitForClients(10 * time.Second); err != nil {
		return err
	}

	html, meta := parser.HTML(content, currentURI)
	html, err = insertPlantumlDiagram(html, true)
	if err != nil {
		_ = protocol.Trace(ctx, protocol.MessageTypeWarning, log("TextDocumentDidOpen - plantuml: "+err.Error()))
	}

	previewServer.Update(filename, html, "", meta)

	return nil
}

func TextDocumentDidChange(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	var err error
	switchedDocument := false

	for _, change := range params.ContentChanges {
		if c, ok := change.(protocol.TextDocumentContentChangeEvent); ok {
			if params.TextDocument.URI != currentURI {
				_ = protocol.Trace(ctx, protocol.MessageTypeInfo,
					log("TextDocumentUriDidChange - switching document: "+params.TextDocument.URI))

				content, err = loadDocument(params.TextDocument.URI)
				if err != nil {
					return err
				}

				currentURI = params.TextDocument.URI
				filename = filepath.Base(currentURI)

				switchedDocument = true
			}

			startIndex, endIndex := c.Range.IndexesIn(content)
			content = content[:startIndex] + c.Text + content[endIndex:]

			currentSection := findSection(content, startIndex)
			html, meta := parser.HTML(content, currentURI)
			html, err = insertPlantumlDiagram(html, switchedDocument)
			if err != nil {
				_ = protocol.Trace(ctx, protocol.MessageTypeWarning, log("TextDocumentDidChange - plantuml: "+err.Error()))
			}

			previewServer.Update(filename, html, currentSection, meta)
		} else if c, ok := change.(protocol.TextDocumentContentChangeEventWhole); ok {
			html, meta := parser.HTML(c.Text, currentURI)
			html, err = insertPlantumlDiagram(html, false)
			if err != nil {
				_ = protocol.Trace(ctx, protocol.MessageTypeWarning, log("TextDocumentDidChange - plantuml: "+err.Error()))
			}

			previewServer.Update(filename, html, "", meta)
		}
	}

	return nil
}

func TextDocumentDidSave(ctx *glsp.Context, params *protocol.DidSaveTextDocumentParams) error {
	var err error

	content, err = loadDocument(params.TextDocument.URI)
	if err != nil {
		return err
	}

	html, meta := parser.HTML(content, currentURI)
	html, err = insertPlantumlDiagram(html, true)
	if err != nil {
		_ = protocol.Trace(ctx, protocol.MessageTypeWarning, log("TextDocumentDidOpen - plantuml: "+err.Error()))
	}

	previewServer.Update(filename, html, "", meta)

	return nil
}

func TextDocumentDidClose(_ *glsp.Context, _ *protocol.DidCloseTextDocumentParams) error {
	return nil
}

func loadDocument(uri string) (string, error) {
	f := parser.NormalizePath(uri)

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

func insertPlantumlDiagram(data string, generate bool) (string, error) {
	const startDelimiter = `<pre><code class="language-plantuml">`
	var builder strings.Builder
	var err error
	numDiagrams := 0
	start := 0

	for {
		s, e := extractPlantUMLSection(data[start:])
		if s == -1 || e == -1 {
			builder.WriteString(data[start:])

			break
		}

		builder.WriteString(data[start : start+s])

		htmlEncodedUml := data[start+s+len(startDelimiter) : start+e]
		uml := html.UnescapeString(htmlEncodedUml)

		p := plantuml.Plantuml{}
		p.EncodedUML = plantuml.Encode(uml)

		generated := false

		for _, enc := range plantumls {
			if p.EncodedUML == enc.EncodedUML {
				p.Diagram = enc.Diagram
				generated = true

				break
			}
		}

		if !generated && generate {
			p.Diagram, err = plantuml.GetDiagram(p.EncodedUML)
			if err != nil {
				return data, err
			}
		}

		numDiagrams++

		if generate {
			if len(plantumls) < numDiagrams {
				plantumls = append(plantumls, p)
			} else {
				plantumls[numDiagrams-1] = p
			}
			builder.WriteString(p.Diagram)
		} else if len(plantumls) >= numDiagrams {
			// Use existing until we save and generate a new one
			builder.WriteString(plantumls[numDiagrams-1].Diagram)
		}

		start += e + 13
	}

	return builder.String(), nil
}

func extractPlantUMLSection(text string) (int, int) {
	const startDelimiter = `<pre><code class="language-plantuml">`
	const endDelimiter = "</code></pre>"

	startIndex := strings.Index(text, startDelimiter)
	if startIndex == -1 {
		return -1, -1
	}

	endIndex := strings.Index(text[startIndex+len(startDelimiter):], endDelimiter)
	if endIndex == -1 {
		return startIndex, -1
	}

	// Calculate the actual end index in the original text
	endIndex += startIndex + len(startDelimiter)

	return startIndex, endIndex
}
