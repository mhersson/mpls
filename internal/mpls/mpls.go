package mpls

import (
	"path/filepath"

	"github.com/mhersson/glsp"
	protocol316 "github.com/mhersson/glsp/protocol_3_16"
	protocol "github.com/mhersson/glsp/protocol_mpls"
	"github.com/mhersson/mpls/pkg/parser"
	"github.com/mhersson/mpls/pkg/plantuml"
)

func EditorDidChangeFocus(ctx *glsp.Context, params *protocol.EditorDidChangeFocusParams) error {
	var err error

	uri := params.URI
	filename := filepath.Base(uri)

	_ = protocol316.Trace(ctx, protocol316.MessageTypeInfo, log("MplsEditorDidChangedFocus: "+uri))

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
	}

	// Check if should auto-open
	if !documentRegistry.ShouldAutoOpen() {
		return nil
	}

	html, meta := parser.HTML(docState.Content, uri)

	html, docState.PlantUMLs, err = plantuml.InsertPlantumlDiagram(html, true, docState.PlantUMLs)
	if err != nil {
		_ = protocol316.Trace(ctx, protocol316.MessageTypeWarning, log("MplsEditorDidChangeFocus - plantuml: "+err.Error()))
	}

	docState.HTML = html
	docState.Meta = meta

	// Get relative path for URI filtering
	relativePath := documentRegistry.GetRelativePath(uri)
	if relativePath == "" {
		relativePath = "/"
	}

	previewServer.UpdateWithURI(filename, relativePath, html, meta)

	return nil
}
