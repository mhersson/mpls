package mpls

import (
	"errors"
	"fmt"
	"time"

	"github.com/mhersson/mpls/internal/previewserver"
	"github.com/mhersson/mpls/pkg/parser"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func WorkspaceExecuteCommand(context *glsp.Context, param *protocol.ExecuteCommandParams) (any, error) {
	switch param.Command {
	case "open-preview":
		_ = protocol.Trace(context, protocol.MessageTypeInfo,
			log("WorkspaceExecuteCommand - Open preview: "+currentURI))

		err := previewserver.Openbrowser(fmt.Sprintf("http://localhost:%d", previewServer.Port), previewserver.Browser)
		if err != nil {
			return nil, err
		}

		if err := previewserver.WaitForClients(10 * time.Second); err != nil {
			return nil, err
		}

		content = string(loadDocument(currentURI))

		html := parser.HTML(content)
		previewServer.Update(filename, html, "")
	default:
		return nil, errors.New("unknow  command")
	}

	return nil, nil
}
