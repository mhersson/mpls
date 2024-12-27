package mpls

import (
	"time"

	"github.com/mhersson/mpls/internal/previewserver"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	serverPkg "github.com/tliron/glsp/server"

	// Must include a backend implementation
	// See CommonLog for other options: https://github.com/tliron/commonlog
	_ "github.com/tliron/commonlog/simple"
)

const lsName = "Markdown Preview Language Server"

var TextDocumentUseFullSync bool
var Version string

func log(message string) string {
	return time.Now().Local().Format("2006-01-02 15:04:05") + " " + message
}

func Run() {
	previewServer = previewserver.New()
	go previewServer.Start()

	time.Sleep(700 * time.Millisecond)

	lspServer := serverPkg.NewServer(&Handler, lsName, false)

	_ = lspServer.RunStdio()
}

func initialize(context *glsp.Context, _ *protocol.InitializeParams) (any, error) {
	protocol.SetTraceValue("message")
	_ = protocol.Trace(context, protocol.MessageTypeInfo, log("Initializing "+lsName))

	capabilities := Handler.CreateServerCapabilities()
	if TextDocumentUseFullSync {
		capabilities.TextDocumentSync = protocol.TextDocumentSyncKindFull
	}

	return protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    lsName,
			Version: &Version,
		},
	}, nil
}

func initialized(_ *glsp.Context, _ *protocol.InitializedParams) error {
	return nil
}

func setTrace(_ *glsp.Context, params *protocol.SetTraceParams) error {
	protocol.SetTraceValue(params.Value)

	return nil
}

func shutdown(_ *glsp.Context) error {
	previewServer.Stop()
	protocol.SetTraceValue(protocol.TraceValueOff)

	return nil
}
