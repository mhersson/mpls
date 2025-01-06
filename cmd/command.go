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
	Run: func(cmd *cobra.Command, _ []string) {
		cmd.Printf("mpls %s - press Ctrl+D to quit.\n", Version)
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
	command.Flags().StringVar(&previewserver.Browser, "browser", "", "Specify the web browser to use for the preview")
	command.Flags().StringVar(&parser.CodeHighlightingStyle, "code-style", "catppuccin-mocha", "Higlighting style for code blocks")
	command.Flags().BoolVar(&previewserver.DarkMode, "dark-mode", false, "Enable dark mode")
	command.Flags().BoolVar(&parser.EnableEmoji, "enable-emoji", false, "Enable emoji support")
	command.Flags().BoolVar(&parser.EnableFootnotes, "enable-footnotes", false, "Enable footnotes")
	command.Flags().BoolVar(&parser.EnableWikiLinks, "enable-wikilinks", false, "Enable [[wiki]] style links")
	command.Flags().BoolVar(&mpls.TextDocumentUseFullSync, "full-sync", false, "Sync entire document for every change")
	command.Flags().BoolVar(&noAuto, "no-auto", false, "Don't open preview automatically")
	command.Flags().IntVar(&previewserver.FixedPort, "port", 0, "Set a fixed port for the preview server")
}
