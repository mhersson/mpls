package mpls

import (
	"path/filepath"

	"github.com/mhersson/glsp"
	protocol316 "github.com/mhersson/glsp/protocol_3_16"
	protocol "github.com/mhersson/glsp/protocol_mpls"
	"github.com/mhersson/mpls/internal/previewserver"
	"github.com/mhersson/mpls/pkg/parser"
	"github.com/mhersson/mpls/pkg/plantuml"
)

func EditorDidChangeFocus(ctx *glsp.Context, params *protocol.EditorDidChangeFocusParams) error {
	var err error

	plantumls = []plantuml.Plantuml{}

	if currentURI == params.URI {
		return nil
	}

	_ = protocol316.Trace(ctx, protocol316.MessageTypeInfo, log("MplsEditorDidChangedFocus: "+params.URI))

	if !previewserver.OpenBrowserOnStartup && content == "" {
		return nil
	}

	content, err = loadDocument(params.URI)
	if err != nil {
		return err
	}

	filename = filepath.Base(params.URI)
	currentURI = params.URI

	html, meta := parser.HTML(content, currentURI)

	html, err = insertPlantumlDiagram(html, true)
	if err != nil {
		_ = protocol316.Trace(ctx, protocol316.MessageTypeWarning, log("MplsEditorDidChangeFocus - plantuml: "+err.Error()))
	}

	previewServer.Update(filename, html, meta)

	return nil
}
