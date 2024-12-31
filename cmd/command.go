package cmd

import (
	"os"

	"github.com/mhersson/mpls/internal/mpls"
	"github.com/mhersson/mpls/internal/previewserver"
	"github.com/mhersson/mpls/pkg/parser"
	"github.com/spf13/cobra"
)

var (
	Version string
	noAuto  bool
)

var command = &cobra.Command{
	Use:     "mpls",
	Short:   "Markdown Preview Language Server",
	Version: Version,
	Run: func(_ *cobra.Command, _ []string) {
		previewserver.OpenBrowserOnStartup = !noAuto
		mpls.Run()
	},
}

func Execute() {
	err := command.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	command.Flags().BoolVar(&noAuto, "no-auto", false, "Don't open preview automatically")
	command.Flags().BoolVar(&mpls.TextDocumentUseFullSync, "full-sync", false, "Sync entire document for every change")
	command.Flags().StringVar(&parser.CodeHighlightingStyle, "code-style", "catppuccin-mocha", "Higlighting style for code blocks")
}
