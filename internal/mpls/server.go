package mpls

import (
	"time"

	"github.com/mhersson/glsp"
	protocol316 "github.com/mhersson/glsp/protocol_3_16"
	protocol "github.com/mhersson/glsp/protocol_mpls"
	serverPkg "github.com/mhersson/glsp/server"
	"github.com/mhersson/mpls/internal/previewserver"

	// Must include a backend implementation
	// See CommonLog for other options: https://github.com/tliron/commonlog
	_ "github.com/tliron/commonlog/simple"
)

const lsName = "Markdown Preview Language Server"

var (
	TextDocumentUseFullSync bool
	Version                 string
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

func initialize(context *glsp.Context, _ *protocol.InitializeParams) (any, error) {
	protocol316.SetTraceValue("message")
	_ = protocol316.Trace(context, protocol316.MessageTypeInfo, log("Initializing "+lsName))

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

func initialized(_ *glsp.Context, _ *protocol316.InitializedParams) error {
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
