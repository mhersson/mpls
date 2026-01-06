package mpls

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mhersson/glsp"
	protocol "github.com/mhersson/glsp/protocol_3_16"
	"github.com/mhersson/mpls/internal/previewserver"
)

func WorkspaceExecuteCommand(ctx *glsp.Context, param *protocol.ExecuteCommandParams) (any, error) {
	switch param.Command {
	case "open-preview":
		_ = protocol.Trace(ctx, protocol.MessageTypeInfo,
			log("WorkspaceExecuteCommand - Open preview"))

		// Get the most recent document to determine which URL to open
		doc := documentRegistry.GetMostRecentDocument()

		var previewURL string
		if previewserver.EnableTabs && doc != nil {
			// MULTI-TAB MODE: Open at file-specific URL
			relativePath := documentRegistry.GetRelativePath(doc.URI)
			if relativePath != "" {
				previewURL = fmt.Sprintf("http://localhost:%d%s", previewServer.Port, relativePath)
			} else {
				previewURL = fmt.Sprintf("http://localhost:%d/", previewServer.Port)
			}
		} else {
			// SINGLE-PAGE MODE: Always open at root
			previewURL = fmt.Sprintf("http://localhost:%d/", previewServer.Port)
		}

		// Open browser
		err := previewserver.Openbrowser(previewURL, previewserver.Browser)
		if err != nil {
			return nil, err
		}

		if err := previewserver.WaitForClients(10 * time.Second); err != nil {
			return nil, err
		}

		// Mark first preview shown for --no-auto behavior
		documentRegistry.MarkFirstPreviewShown()

		// If there are documents in registry, update preview with the most recent one
		// This ensures preview shows content when opened with --no-auto
		if doc != nil && doc.HTML != "" {
			documentURI := ""
			if previewserver.EnableTabs {
				relativePath := documentRegistry.GetRelativePath(doc.URI)
				if relativePath == "" {
					relativePath = "/"
				}
				documentURI = relativePath
			}

			previewServer.UpdateWithURI(filepath.Base(doc.URI), documentURI, doc.HTML, doc.Meta)
		}
	default:
		return nil, errors.New("unknown command")
	}

	return nil, nil
}

func WorkspaceDidChangeConfiguration(_ *glsp.Context, _ *protocol.DidChangeConfigurationParams) error {
	return nil
}
