package mpls

import (
	"strings"
	"time"

	"github.com/mhersson/glsp"
	protocol316 "github.com/mhersson/glsp/protocol_3_16"
	protocol "github.com/mhersson/glsp/protocol_mpls"
	serverPkg "github.com/mhersson/glsp/server"
	"github.com/mhersson/mpls/internal/previewserver"
	"github.com/mhersson/mpls/pkg/parser"

	// Must include a backend implementation
	// See CommonLog for other options: https://github.com/tliron/commonlog
	_ "github.com/tliron/commonlog/simple"
)

const lsName = "Markdown Preview Language Server"

var (
	TextDocumentUseFullSync bool
	Version                 string
	workspaceRoot           string
)

func log(message string) string {
	return time.Now().Local().Format("2006-01-02 15:04:05") + " " + message
}

func Run() {
	previewServer = previewserver.New()
	go previewServer.Start()

	lspServer := serverPkg.NewServer(&Handler, lsName, false)

	_ = lspServer.RunStdio()
}

func initialize(context *glsp.Context, params *protocol.InitializeParams) (any, error) {
	protocol316.SetTraceValue("message")
	_ = protocol316.Trace(context, protocol316.MessageTypeInfo, log("Initializing "+lsName))

	// Extract workspace root
	if len(params.WorkspaceFolders) > 0 {
		workspaceRoot = parser.NormalizePath(string(params.WorkspaceFolders[0].URI))
	} else if params.RootURI != nil {
		workspaceRoot = parser.NormalizePath(string(*params.RootURI))
	} else if params.RootPath != nil {
		workspaceRoot = *params.RootPath
	}

	// Initialize document registry with workspace root
	InitializeDocumentRegistry(workspaceRoot)

	// Pass workspace root to preview server
	previewServer.SetWorkspaceRoot(workspaceRoot)

	// Set workspace root for parser link resolution
	parser.WorkspaceRoot = workspaceRoot

	capabilities := Handler.CreateServerCapabilities()
	if TextDocumentUseFullSync {
		capabilities.TextDocumentSync = protocol316.TextDocumentSyncKindFull
	}

	capabilities.ExecuteCommandProvider.Commands = []string{"open-preview"}

	return protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &protocol316.InitializeResultServerInfo{
			Name:    lsName,
			Version: &Version,
		},
	}, nil
}

func initialized(ctx *glsp.Context, _ *protocol316.InitializedParams) error {
	// Start goroutine to handle browser -> LSP -> editor requests
	startDocumentRequestHandler(ctx)

	return nil
}

func setTrace(_ *glsp.Context, params *protocol316.SetTraceParams) error {
	protocol316.SetTraceValue(params.Value)

	return nil
}

func shutdown(_ *glsp.Context) error {
	previewServer.Stop()
	protocol316.SetTraceValue(protocol316.TraceValueOff)

	return nil
}

func boolPtr(b bool) *bool {
	return &b
}

func startDocumentRequestHandler(ctx *glsp.Context) {
	go func() {
		for req := range previewserver.LSPRequestChan {
			// Convert workspace-relative path to file:// URI
			relativePath := req.URI
			if strings.HasPrefix(relativePath, "/") {
				relativePath = strings.TrimPrefix(relativePath, "/")
			}

			// Construct absolute file path
			fileURI := documentRegistry.GetFileURI("/" + relativePath)

			// Create ShowDocumentParams
			params := protocol316.ShowDocumentParams{
				URI:       protocol316.URI(fileURI),
				External:  boolPtr(false),
				TakeFocus: boolPtr(req.TakeFocus),
			}

			// Send window/showDocument request to client
			var result protocol316.ShowDocumentResult

			ctx.Call(protocol316.ServerWindowShowDocument, params, &result)

			// Mark first preview shown for --no-auto behavior
			if result.Success {
				documentRegistry.MarkFirstPreviewShown()
			}
		}
	}()
}
