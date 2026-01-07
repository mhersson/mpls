package mpls

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/mhersson/glsp"
	protocol "github.com/mhersson/glsp/protocol_3_16"
	"github.com/mhersson/mpls/internal/previewserver"
	"github.com/mhersson/mpls/pkg/parser"
	"github.com/mhersson/mpls/pkg/plantuml"
)

var (
	previewServer       *previewserver.Server
	validFileExtensions = []string{".md", ".markdown", ".mkd", ".mkdn", ".mdwn"}
)

func TextDocumentDidOpen(ctx *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	if !slices.Contains(validFileExtensions, filepath.Ext(params.TextDocument.URI)) {
		return nil
	}

	var err error

	uri := params.TextDocument.URI
	content := params.TextDocument.Text

	_ = protocol.Trace(ctx, protocol.MessageTypeInfo, log("TextDocumentDidOpen: "+params.TextDocument.URI))

	// Always render HTML (even with --no-auto, so it's ready when user runs open-preview)
	html, meta := parser.HTML(content, uri)

	var plantUMLs []plantuml.Plantuml

	html, plantUMLs, err = plantuml.InsertPlantumlDiagram(html, true, []plantuml.Plantuml{})
	if err != nil {
		_ = protocol.Trace(ctx, protocol.MessageTypeWarning, log("TextDocumentDidOpen - plantuml: "+err.Error()))
	}

	// Register document in registry with rendered content
	docState := &DocumentState{
		URI:       uri,
		Content:   content,
		HTML:      html,
		Meta:      meta,
		PlantUMLs: plantUMLs,
	}
	documentRegistry.Register(uri, docState)

	// Check if should auto-open browser
	if !documentRegistry.ShouldAutoOpen() {
		// Don't open browser, just register the document
		return nil
	}

	// Get relative path for URL
	relativePath := documentRegistry.GetRelativePath(uri)
	if relativePath == "" {
		relativePath = "/"
	}

	if previewserver.EnableTabs {
		// MULTI-TAB MODE: Open new browser tab at file-specific URL
		previewURL := fmt.Sprintf("http://localhost:%d%s", previewServer.Port, relativePath)

		err = previewserver.Openbrowser(previewURL, previewserver.Browser)
		if err != nil {
			_ = protocol.Trace(ctx, protocol.MessageTypeWarning, log("TextDocumentDidOpen - failed to open browser: "+err.Error()))
		}
	} else {
		// SINGLE-PAGE MODE: Update existing preview or open at root
		if len(previewserver.GetClients()) == 0 {
			// No browser open yet - open at root
			previewURL := fmt.Sprintf("http://localhost:%d/", previewServer.Port)

			err = previewserver.Openbrowser(previewURL, previewserver.Browser)
			if err != nil {
				_ = protocol.Trace(ctx, protocol.MessageTypeWarning, log("TextDocumentDidOpen - failed to open browser: "+err.Error()))
			}

			// Wait for WebSocket connection and send initial content
			if err := previewserver.WaitForClients(2 * time.Second); err == nil {
				previewServer.UpdateWithURI(filepath.Base(uri), "", html, meta)
			}
		} else {
			// Browser already open - send update via WebSocket
			previewServer.UpdateWithURI(filepath.Base(uri), "", html, meta)
		}
	}

	return nil
}

func TextDocumentDidChange(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	if !slices.Contains(validFileExtensions, filepath.Ext(params.TextDocument.URI)) {
		return nil
	}

	var err error

	uri := params.TextDocument.URI
	filename := filepath.Base(uri)

	// Get document state from registry
	docState, exists := documentRegistry.Get(uri)
	if !exists {
		// Document not in registry, load from disk
		content, err := loadDocument(uri)
		if err != nil {
			return err
		}

		docState = &DocumentState{
			URI:       uri,
			Content:   content,
			PlantUMLs: []plantuml.Plantuml{},
		}
		documentRegistry.Register(uri, docState)

		_ = protocol.Trace(ctx, protocol.MessageTypeInfo,
			log("TextDocumentUriDidChange - loaded new document: "+uri))
	}

	// Get relative path for URI filtering
	relativePath := documentRegistry.GetRelativePath(uri)
	if relativePath == "" {
		relativePath = "/"
	}

	for _, change := range params.ContentChanges {
		if c, ok := change.(protocol.TextDocumentContentChangeEvent); ok {
			startIndex, endIndex := c.Range.IndexesIn(docState.Content)
			docState.Content = docState.Content[:startIndex] + c.Text + docState.Content[endIndex:]

			html, meta := parser.HTML(docState.Content, uri)

			html, docState.PlantUMLs, err = plantuml.InsertPlantumlDiagram(html, false, docState.PlantUMLs)
			if err != nil {
				_ = protocol.Trace(ctx, protocol.MessageTypeWarning, log("TextDocumentDidChange - plantuml: "+err.Error()))
			}

			docState.HTML = html
			docState.Meta = meta

			// Set documentURI based on mode
			documentURI := ""
			if previewserver.EnableTabs {
				documentURI = relativePath
			}

			previewServer.UpdateWithURI(filename, documentURI, html, meta)
		} else if c, ok := change.(protocol.TextDocumentContentChangeEventWhole); ok {
			docState.Content = c.Text

			html, meta := parser.HTML(c.Text, uri)

			html, docState.PlantUMLs, err = plantuml.InsertPlantumlDiagram(html, false, docState.PlantUMLs)
			if err != nil {
				_ = protocol.Trace(ctx, protocol.MessageTypeWarning, log("TextDocumentDidChange - plantuml: "+err.Error()))
			}

			docState.HTML = html
			docState.Meta = meta

			// Set documentURI based on mode
			documentURI := ""
			if previewserver.EnableTabs {
				documentURI = relativePath
			}

			previewServer.UpdateWithURI(filename, documentURI, html, meta)
		}
	}

	return nil
}

func TextDocumentDidSave(ctx *glsp.Context, params *protocol.DidSaveTextDocumentParams) error {
	if !slices.Contains(validFileExtensions, filepath.Ext(params.TextDocument.URI)) {
		return nil
	}

	var err error

	uri := params.TextDocument.URI
	filename := filepath.Base(uri)

	// Reload document from disk
	content, err := loadDocument(uri)
	if err != nil {
		return err
	}

	// Get document state from registry
	docState, exists := documentRegistry.Get(uri)
	if !exists {
		docState = &DocumentState{
			URI:       uri,
			PlantUMLs: []plantuml.Plantuml{},
		}
	}

	docState.Content = content

	html, meta := parser.HTML(content, uri)

	html, docState.PlantUMLs, err = plantuml.InsertPlantumlDiagram(html, true, docState.PlantUMLs)
	if err != nil {
		_ = protocol.Trace(ctx, protocol.MessageTypeWarning, log("TextDocumentDidSave - plantuml: "+err.Error()))
	}

	docState.HTML = html
	docState.Meta = meta

	documentRegistry.Register(uri, docState)

	// Get relative path for URI filtering
	relativePath := documentRegistry.GetRelativePath(uri)
	if relativePath == "" {
		relativePath = "/"
	}

	// Set documentURI based on mode
	documentURI := ""
	if previewserver.EnableTabs {
		documentURI = relativePath
	}

	previewServer.UpdateWithURI(filename, documentURI, html, meta)

	return nil
}

func TextDocumentDidClose(ctx *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
	if !slices.Contains(validFileExtensions, filepath.Ext(params.TextDocument.URI)) {
		return nil
	}

	uri := params.TextDocument.URI

	_ = protocol.Trace(ctx, protocol.MessageTypeInfo, log("TextDocumentDidClose: "+uri))

	// Remove document from registry
	documentRegistry.Remove(uri)

	// Get relative path for the closed document
	relativePath := documentRegistry.GetRelativePath(uri)
	if relativePath == "" {
		relativePath = "/"
	}

	// Check if this was the last document
	isLastDocument := documentRegistry.IsEmpty()

	// Send close message to browser
	previewServer.CloseDocument(relativePath, isLastDocument)

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
