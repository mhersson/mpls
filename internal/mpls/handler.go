package mpls

import (
	protocol "github.com/tliron/glsp/protocol_3_16"
)

var Handler protocol.Handler

func init() {
	Handler.Initialize = initialize
	Handler.Initialized = initialized
	Handler.Shutdown = shutdown
	Handler.SetTrace = setTrace
	Handler.TextDocumentDidOpen = TextDocumentDidOpen
	Handler.TextDocumentDidChange = TextDocumentDidChange
	Handler.TextDocumentDidSave = TextDocumentDidSave
	Handler.TextDocumentDidClose = TextDocumentDidClose
	Handler.WorkspaceExecuteCommand = WorkspaceExecuteCommand
}
