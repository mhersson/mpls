package cmd

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/mhersson/mpls/internal/mpls"
	"github.com/mhersson/mpls/internal/previewserver"
	"github.com/mhersson/mpls/pkg/parser"
	"github.com/mhersson/mpls/pkg/plantuml"
	"github.com/spf13/cobra"
)

var (
	noAuto    bool
	Version   = "dev"
	CommitSHA = "unknown"
	BuildTime = "unknown"
)

var command = &cobra.Command{
	Use:     "mpls",
	Short:   "Markdown Preview Language Server",
	Version: getVersionInfo(),
	Run: func(cmd *cobra.Command, _ []string) {
		cmd.Printf("mpls %s - press Ctrl+D to quit.\n", cmd.Version)
		previewserver.OpenBrowserOnStartup = !noAuto
		mpls.Run()
	},
}

func getVersionInfo() string {
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					CommitSHA = setting.Value[:8]
				}
				if setting.Key == "vcs.time" {
					BuildTime = setting.Value
				}
			}

			Version = info.Main.Version
			if Version == "(devel)" {
				return Version
			}
		}
	}

	return fmt.Sprintf("%s (commit: %s, built at: %s)", Version, CommitSHA, BuildTime)
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
	command.Flags().StringVar(&plantuml.BasePath, "plantuml-path", "plantuml", "Specify the base path for the plantuml server")
	command.Flags().StringVar(&plantuml.Server, "plantuml-server", "www.plantuml.com", "Specify the host for the plantuml server")
	command.Flags().BoolVar(&plantuml.DisableTLS, "plantuml-disable-tls", false, "Disable encryption on requests to the plantuml server")
}
