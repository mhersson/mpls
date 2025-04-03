package mpls

import (
	"errors"
	"fmt"
	"time"

	"github.com/mhersson/glsp"
	protocol "github.com/mhersson/glsp/protocol_3_16"
	"github.com/mhersson/mpls/internal/previewserver"
	"github.com/mhersson/mpls/pkg/parser"
)

func WorkspaceExecuteCommand(ctx *glsp.Context, param *protocol.ExecuteCommandParams) (any, error) {
	switch param.Command {
	case "open-preview":
		_ = protocol.Trace(ctx, protocol.MessageTypeInfo,
			log("WorkspaceExecuteCommand - Open preview: "+currentURI))

		err := previewserver.Openbrowser(fmt.Sprintf("http://localhost:%d", previewServer.Port), previewserver.Browser)
		if err != nil {
			return nil, err
		}

		if err := previewserver.WaitForClients(10 * time.Second); err != nil {
			return nil, err
		}

		content, err = loadDocument(currentURI)
		if err != nil {
			return nil, err
		}

		html, meta := parser.HTML(content, currentURI)
		html, err = insertPlantumlDiagram(html, true)
		if err != nil {
			_ = protocol.Trace(ctx, protocol.MessageTypeWarning, log("WorkspaceExcueCommand - Open preview: "+err.Error()))
		}

		previewServer.Update(filename, html, "", meta)
	default:
		return nil, errors.New("unknow  command")
	}

	return nil, nil
}

func WorkspaceDidChangeConfiguration(_ *glsp.Context, _ *protocol.DidChangeConfigurationParams) error {
	return nil
}
