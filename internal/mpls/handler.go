package mpls

import (
	protocol "github.com/mhersson/glsp/protocol_mpls"
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
	Handler.WorkspaceDidChangeConfiguration = WorkspaceDidChangeConfiguration
	Handler.MplsEditorDidChangeFocus = MplsEditorDidChangeFocus
}
