package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mhersson/mpls/internal/previewserver"
	"github.com/mhersson/mpls/pkg/parser"
	"github.com/spf13/cobra"
)

var (
	demoWait   time.Duration
	demoNoAuto bool
)

const demoMarkdown = `# Theme and Feature Showcase

Demonstrating some of the **mpls** markdown rendering features.

> [!IMPORTANT]
> GitHub-style alerts supported - emojis work too! :smile: :rocket:

<img src="./assets/yoshi.png" alt="yoshi" style="float:right; width:250px; margin-right: 100px; margin-top:80px;">

## Tables & Images

> [!TIP]
> Images support both ` + "`![]()`" + ` and ` + "`<img style=\"...\">`" + ` syntax.

| Feature | Status | Notes     |
|---------|:------:|----------:|
| Tables  |   ✓    | GFM style |
| Mermaid |   ✓    | Diagrams  |
| KaTeX   |   ✓    | Math      |

## Code

` + "```go\nfunc main() {\n    fmt.Println(\"Hello, mpls!\")\n}\n```" + `

<div style="display:flex; gap:40px;">
<div>

## Lists, Tasks & Math :notebook:

- Unordered item
- Another with **bold**

1. First ordered
2. Second ordered

- [x] Completed task
- [ ] Pending task

</div>
<div style="flex:1; display:flex; justify-content:center; align-items:center;">

$$\int_{-\infty}^{\infty} e^{-x^2} dx = \sqrt{\pi}$$

</div>
</div>

## Mermaid Diagrams

` + "```mermaid\n" +
	"flowchart LR\n" +
	"    subgraph Editor\n" +
	"        E[📝 Edit Markdown]\n" +
	"    end\n" +
	"    subgraph mpls\n" +
	"        L[LSP Server]\n" +
	"        P[Parser]\n" +
	"        W[WebSocket]\n" +
	"        L --> P --> W\n" +
	"    end\n" +
	"    subgraph Browser\n" +
	"        V[🌐 Live Preview]\n" +
	"    end\n" +
	"    E -->|didChange| L\n" +
	"    W -->|update| V\n" +
	"    style E fill:#6c9,stroke:#485,color:#333\n" +
	"    style L fill:#69c,stroke:#458,color:#333\n" +
	"    style P fill:#c6c,stroke:#848,color:#333\n" +
	"    style W fill:#69c,stroke:#458,color:#333\n" +
	"    style V fill:#f96,stroke:#b64,color:#333\n" +
	"```" + `
`

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Start a demo preview server with sample content",
	Long: `Start a standalone preview server with sample markdown content.
Useful for taking screenshots of different themes.

Example:
  mpls demo --theme catppuccin-mocha --wait 10s
  mpls demo --theme dark --port 9999 --no-auto`,
	Run: func(_ *cobra.Command, _ []string) {
		// Get current working directory for workspace root
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get working directory: %v\n", err)
			os.Exit(1)
		}

		// Set code highlighting style based on theme
		if chromaStyle := previewserver.GetChromaStyleForTheme(previewserver.Theme); chromaStyle != "" {
			parser.CodeHighlightingStyle = chromaStyle
		}

		// Enable emoji support for demo
		parser.EnableEmoji = true

		// Set workspace root for relative path resolution
		parser.WorkspaceRoot = cwd

		// Render demo markdown
		demoURI := "file://" + cwd + "/demo.md"
		html, meta := parser.HTML(demoMarkdown, demoURI, 0)

		// Create and configure preview server
		server := previewserver.New()
		server.SetWorkspaceRoot(cwd)

		// Pre-populate content so it's available when browser connects
		server.Update("demo.md", html, meta)

		// Start server in background
		go server.Start()

		url := fmt.Sprintf("http://localhost:%d", server.Port)
		fmt.Printf("Demo server running at %s (theme: %s)\n", url, previewserver.Theme)

		// Open browser unless --no-auto
		if !demoNoAuto {
			if err := previewserver.Openbrowser(url, previewserver.Browser); err != nil {
				fmt.Printf("Failed to open browser: %v\n", err)
			}
		}

		// Wait for specified duration
		if demoWait > 0 {
			fmt.Printf("Waiting %s before exit...\n", demoWait)
			time.Sleep(demoWait)
			server.Stop()
		} else {
			fmt.Println("Press Ctrl+C to stop.")

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			<-sigChan

			fmt.Println("\nShutting down...")
			server.Stop()
		}
	},
}

func init() {
	demoCmd.Flags().DurationVar(&demoWait, "wait", 0, "Duration to keep server running (0 = run until interrupted)")
	demoCmd.Flags().BoolVar(&demoNoAuto, "no-auto", false, "Don't open browser automatically")

	// Theme, port, and browser flags are defined on root command
	// They work via the global previewserver variables
}
