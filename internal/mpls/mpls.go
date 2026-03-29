package mpls

import (
	"encoding/json"
	"path/filepath"

	"github.com/mhersson/mpls/internal/previewserver"
	"github.com/mhersson/mpls/pkg/parser"
	"github.com/mhersson/mpls/pkg/plantuml"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

type editorDidChangeFocusParams struct {
	URI string `json:"uri"`
}

func editorDidChangeFocus(ctx *glsp.Context, params json.RawMessage) (any, error) {
	var p editorDidChangeFocusParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	uri := p.URI
	filename := filepath.Base(uri)

	_ = protocol.Trace(ctx, protocol.MessageTypeInfo, log("MplsEditorDidChangedFocus: "+uri))

	// Get document state from registry
	docState, exists := documentRegistry.Get(uri)
	if !exists {
		// Document not in registry, load from disk
		content, err := loadDocument(uri)
		if err != nil {
			return nil, err
		}

		docState = &DocumentState{
			URI:       uri,
			Content:   content,
			PlantUMLs: []plantuml.Plantuml{},
		}
		documentRegistry.Register(uri, docState)
	}

	// In single-page mode, always update regardless of --no-auto
	// In multi-tab mode, respect ShouldAutoOpen
	if previewserver.EnableTabs && !documentRegistry.ShouldAutoOpen() {
		return nil, nil
	}

	html, meta := parser.HTML(docState.Content, uri, 0)

	var err error

	html, docState.PlantUMLs, err = plantuml.InsertPlantumlDiagram(html, true, docState.PlantUMLs)
	if err != nil {
		_ = protocol.Trace(ctx, protocol.MessageTypeWarning, log("MplsEditorDidChangeFocus - plantuml: "+err.Error()))
	}

	docState.HTML = html
	docState.Meta = meta

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

	return nil, nil
}
