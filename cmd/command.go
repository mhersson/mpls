package cmd

import (
	"os"

	"github.com/mhersson/mpls/internal/mpls"
	"github.com/spf13/cobra"
)

var (
	Version string
)

var command = &cobra.Command{
	Use:     "mpls",
	Short:   "Markdown Preview Language Server",
	Version: Version,
	Run: func(_ *cobra.Command, _ []string) {
		mpls.Run()
	},
}

func Execute() {
	err := command.Execute()
	if err != nil {
		os.Exit(1)
	}
}
