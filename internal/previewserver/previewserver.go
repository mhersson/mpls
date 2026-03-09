package previewserver

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mhersson/mpls/pkg/parser"
	"github.com/mhersson/mpls/pkg/plantuml"
)

// Current version of katex used: 0.16.25 (https://cdn.jsdelivr.net/npm/katex@0.16.25/dist/katex.min.css)
// Current version of mermaid used: 11.12.1 (https://cdn.jsdelivr.net/npm/mermaid@11.12.1/dist/mermaid.min.js)

var (
	Browser              string
	Theme                string
	FixedPort            int
	OpenBrowserOnStartup bool
	EnableTabs           bool

	// Current content state for single-page mode.
	currentHTML  string
	currentTitle string
	currentMeta  string
	contentMutex sync.RWMutex

	//go:embed web/index.html
	indexHTML string
	//go:embed web/katex.min.css
	katexMinCSS string
	//go:embed web/styles.css
	stylesCSS string
	//go:embed web/mermaid.min.js
	mermaid string
	//go:embed web/ws.js
	websocketJS string
	//go:embed web/presentation.js
	presentationJS string
	//go:embed web/presentation.css
	presentationCSS string
	//go:embed web/fonts
	katexFontsFS embed.FS
	//go:embed web/themes
	themesFS embed.FS

	clients         []*websocket.Conn
	clientsMutex    sync.Mutex
	clientConnected = make(chan struct{}, 1)
	stopChan        = make(chan os.Signal, 1)
	LSPRequestChan  = make(chan OpenDocumentRequest)
)

type OpenDocumentRequest struct {
	URI           string
	TakeFocus     bool
	UpdatePreview bool
}

type Server struct {
	Server         *http.Server
	InitialContent string
	Port           int
	WorkspaceRoot  string
}

func logTime() string {
	return time.Now().Local().Format("2006-01-02 15:04:05")
}

func ListThemes() {
	fmt.Println("Available themes:")

	entries, err := fs.ReadDir(themesFS, "web/themes")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading themes: %v\n", err)

		return
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".css") {
			themeName := strings.TrimSuffix(entry.Name(), ".css")
			_, mermaidTheme := getThemeConfig(themeName)

			fmt.Printf("  %-20s (mermaid: %s)\n", themeName, mermaidTheme)
		}
	}
}

func WaitForClients(timeout time.Duration) error {
	select {
	case <-clientConnected:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for clients to connect")
	}
}

// HasClients returns true if any WebSocket clients are connected.
func HasClients() bool {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	return len(clients) > 0
}

// broadcastToClients sends a message to all connected clients, removing any that fail.
func broadcastToClients(msg []byte) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	alive := clients[:0]
	for _, c := range clients {
		if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
			fmt.Fprintf(os.Stderr, "%s error sending message: %v\n", logTime(), err)

			_ = c.Close()
		} else {
			alive = append(alive, c)
		}
	}

	clients = alive
}

// GetChromaStyleForTheme returns a recommended chroma syntax highlighting style for a given theme.
// Since theme names match chroma conventions, most themes return their name directly.
func GetChromaStyleForTheme(themeName string) string {
	// Special cases where mpls theme name differs from chroma style name
	// or where there's no exact chroma match
	specialCases := map[string]string{
		"ayu-dark":        "github-dark",      // No exact ayu in chroma, github-dark is clean
		"ayu-light":       "github",           // No exact ayu in chroma, github is clean
		"dark":            "catppuccin-mocha", // Default dark → catppuccin-mocha (maintains original default)
		"light":           "catppuccin-mocha", // Default light → catppuccin-mocha (maintains original default)
		"everforest-dark": "evergarden",       // No everforest in chroma → evergarden
		"gruvbox-dark":    "gruvbox",          // Chroma uses "gruvbox" for dark variant
		"tokyonight":      "tokyonight-night", // Base variant maps to -night
	}

	if chromaStyle, exists := specialCases[themeName]; exists {
		return chromaStyle
	}

	// For all other themes, the theme name matches the chroma style name directly
	// (catppuccin-mocha, catppuccin-frappe, nord, dracula, rose-pine, etc.)
	return themeName
}

func getThemeConfig(themeName string) (cssFile, mermaidTheme string) {
	cssFile = fmt.Sprintf("themes/%s.css", themeName)

	// Dark themes that don't have "dark" in their name
	// All other themes with "dark" in the name are automatically detected
	darkThemesWithoutDarkInName := []string{
		"catppuccin-mocha", "catppuccin-frappe", "catppuccin-macchiato",
		"dracula", "nord", "rose-pine", "tokyonight", "tokyonight-storm",
		"tokyonight-moon",
	}

	// Determine mermaid theme
	mermaidTheme = "default"

	// Automatically detect themes with "dark" in their name
	if strings.Contains(themeName, "dark") {
		mermaidTheme = "dark"
	}

	// Check for dark themes without "dark" in their name
	if slices.Contains(darkThemesWithoutDarkInName, themeName) {
		mermaidTheme = "dark"
	}

	return cssFile, mermaidTheme
}

func New() *Server {
	port := rand.Intn(65535-10000) + 10000 //nolint:gosec
	if FixedPort > 0 {
		port = FixedPort
	}

	// Default to light theme if not specified
	if Theme == "" {
		Theme = "light"
	}

	theme, mermaidTheme := getThemeConfig(Theme)

	// Validate theme file exists
	themeFilePath := fmt.Sprintf("web/%s", theme)
	if _, err := themesFS.ReadFile(themeFilePath); err != nil {
		fmt.Fprintf(os.Stderr, "%s Warning: theme '%s' not found, falling back to light\n", logTime(), Theme)

		theme, mermaidTheme = getThemeConfig("light")
	}

	indexHTML = fmt.Sprintf(indexHTML, theme, mermaidTheme)

	srv := &http.Server{
		Addr:        fmt.Sprintf(":%d", port),
		ReadTimeout: time.Second * 5,
	}

	return &Server{
		Server:         srv,
		InitialContent: indexHTML,
		Port:           port,
	}
}

func (s *Server) SetWorkspaceRoot(root string) {
	s.WorkspaceRoot = root
}

func (s *Server) GetWorkspaceRoot() string {
	return s.WorkspaceRoot
}

func isStaticAsset(path string) bool {
	staticPaths := []string{
		"/styles.css",
		"/katex.min.css",
		"/mermaid.min.js",
		"/ws.js",
		"/ws",
		"/presentation.js",
		"/presentation.css",
	}

	if slices.Contains(staticPaths, path) {
		return true
	}

	// Check for /fonts/ and /themes/ prefixes
	return strings.HasPrefix(path, "/fonts/") || strings.HasPrefix(path, "/themes/")
}

func isValidMarkdownExt(ext string) bool {
	validExts := []string{".md", ".markdown", ".mkd", ".mkdn", ".mdwn"}

	return slices.Contains(validExts, ext)
}

func (s *Server) serveMarkdownFile(w http.ResponseWriter, r *http.Request) {
	workspaceRoot := s.GetWorkspaceRoot()

	// If no workspace root, serve the initial content
	if workspaceRoot == "" {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(s.InitialContent))

		return
	}

	// Clean the URL path
	urlPath := filepath.Clean(r.URL.Path)

	// Remove leading slash for relative path
	relativePath := strings.TrimPrefix(urlPath, "/")

	// Check for directory traversal attempts
	if strings.Contains(relativePath, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)

		return
	}

	// Construct absolute file path
	absolutePath := filepath.Join(workspaceRoot, relativePath)

	// Normalize workspace root for comparison
	normalizedRoot := parser.NormalizePath("file://" + workspaceRoot)

	// Verify path is within workspace
	normalizedPath := parser.NormalizePath("file://" + absolutePath)
	if !strings.HasPrefix(normalizedPath, normalizedRoot) {
		http.Error(w, "Forbidden", http.StatusForbidden)

		return
	}

	// Verify markdown extension
	ext := filepath.Ext(absolutePath)
	if !isValidMarkdownExt(ext) {
		http.Error(w, "Not a markdown file", http.StatusBadRequest)

		return
	}

	// Load file content
	content, err := os.ReadFile(absolutePath) //nolint:gosec // Path validated above: traversal check + workspace boundary check
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)

		return
	}

	// Render HTML
	fileURI := "file://" + absolutePath
	renderedHTML, meta := parser.HTML(string(content), fileURI, 0)

	// Process PlantUML diagrams
	renderedHTML, _, err = plantuml.InsertPlantumlDiagram(renderedHTML, true, []plantuml.Plantuml{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s error processing PlantUML: %v\n", logTime(), err)
		// Continue without PlantUML if there's an error
	}

	// Create metadata table
	metaJSON, _ := json.Marshal(meta)

	var metaMap map[string]any

	_ = json.Unmarshal(metaJSON, &metaMap)

	metaHTML := ""
	if len(metaMap) > 0 {
		metaHTML = "<table>"
		for key, value := range metaMap {
			metaHTML += fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>",
				html.EscapeString(key), html.EscapeString(fmt.Sprintf("%v", value)))
		}

		metaHTML += "</table>"
	}

	// Create full HTML page
	fullHTML := s.InitialContent
	fullHTML = strings.Replace(fullHTML, `<div class="preview-content" id="content"></div>`,
		fmt.Sprintf(`<div class="preview-content" id="content">%s</div>`, renderedHTML), 1)
	fullHTML = strings.Replace(fullHTML, `<div id="header-meta"></div>`,
		fmt.Sprintf(`<div id="header-meta">%s</div>`, metaHTML), 1)
	fullHTML = strings.Replace(fullHTML, `<summary id="header-summary"></summary>`,
		fmt.Sprintf(`<summary id="header-summary">%s</summary>`, filepath.Base(absolutePath)), 1)

	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(fullHTML)) //nolint:gosec
}

func (s *Server) Start() {
	// Static asset routes
	http.HandleFunc("/styles.css", handleResponse("text/css", stylesCSS))
	http.HandleFunc("/katex.min.css", handleResponse("text/css", katexMinCSS))
	http.HandleFunc("/mermaid.min.js", handleResponse("application/javascript", mermaid))
	http.HandleFunc("/ws.js", handleResponse("application/javascript", fmt.Sprintf(websocketJS, s.Port)))
	http.HandleFunc("/presentation.js", handleResponse("application/javascript", presentationJS))
	http.HandleFunc("/presentation.css", handleResponse("text/css", presentationCSS))

	// Serve embedded KaTeX fonts
	fontsSubFS, _ := fs.Sub(katexFontsFS, "web/fonts")
	http.Handle("/fonts/", http.StripPrefix("/fonts/", http.FileServer(http.FS(fontsSubFS))))

	// Serve embedded themes
	themesSubFS, _ := fs.Sub(themesFS, "web/themes")
	http.Handle("/themes/", http.StripPrefix("/themes/", http.FileServer(http.FS(themesSubFS))))

	http.HandleFunc("/ws", handleWebSocket)

	// Dynamic route handler for markdown files and root path
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Root path - serve initial content
		if path == "/" || path == "" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(s.InitialContent))

			return
		}

		// Static assets are handled by other handlers above
		// This shouldn't be reached for static assets, but check anyway
		if isStaticAsset(path) {
			http.NotFound(w, r)

			return
		}

		// Treat as markdown file request
		s.serveMarkdownFile(w, r)
	})

	signal.Notify(stopChan, os.Interrupt)

	go func() {
		if err := s.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("%s error starting server: %s\n", logTime(), err)
		}
	}()

	// Browser opening is handled by:
	// 1. TextDocumentDidOpen when ShouldAutoOpen() returns true
	// 2. The "open-preview" workspace command
	// We don't open here because workspace root isn't set yet during initialization

	// Wait for interrupt signal
	<-stopChan
	s.Stop()
}

// Update updates the current HTML content.
func (s *Server) Update(filename, newContent string, meta map[string]any) {
	s.UpdateWithURI(filename, "", newContent, meta)
}

// CloseDocument sends a close message to clients viewing the specified document.
func (s *Server) CloseDocument(documentURI string, isLastDocument bool) {
	type CloseEvent struct {
		Type           string
		DocumentURI    string
		IsLastDocument bool
	}

	e := CloseEvent{Type: "closeDocument", DocumentURI: documentURI, IsLastDocument: isLastDocument}

	eventJSON, err := json.Marshal(e)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling close event to JSON: %v\n", err)

		return
	}

	broadcastToClients(eventJSON)
}

// UpdateWithURI updates the current HTML content with document URI for client filtering.
func (s *Server) UpdateWithURI(filename, documentURI string, newContent string, meta map[string]any) {
	type Event struct {
		HTML        string
		Title       string
		Meta        string
		DocumentURI string
	}

	t := strings.TrimSuffix(filename, ".md")
	m := convertMetaToHTMLTable(meta)

	e := Event{HTML: newContent, Title: t, Meta: m, DocumentURI: documentURI}

	// Store current content for single-page mode (when no documentURI filtering)
	if !EnableTabs {
		contentMutex.Lock()
		currentHTML = newContent
		currentTitle = t
		currentMeta = m
		contentMutex.Unlock()
	}

	eventJSON, err := json.Marshal(e)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling event to JSON: %v\n", err)

		return
	}

	broadcastToClients(eventJSON)
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() {
	// Create a context with a timeout for the shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt to gracefully shut down the server
	if err := s.Server.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%s error shutting down server: %v\n", logTime(), err)
	}
}

func handleResponse(contentType, response string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		fmt.Fprint(w, response)
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	wsupgrader := websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool {
			return true // allow all origins
		},
	}

	conn, err := wsupgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not open websocket connection", http.StatusBadRequest)
		fmt.Fprintf(os.Stderr, "%s error could not open websocket connection: %v\n", logTime(), err)

		return
	}

	// Send initial messages BEFORE adding to clients slice to avoid
	// concurrent writes with broadcastToClients
	configMsg := map[string]any{
		"Type":       "config",
		"EnableTabs": EnableTabs,
	}
	if msgJSON, err := json.Marshal(configMsg); err == nil {
		if err := conn.WriteMessage(websocket.TextMessage, msgJSON); err != nil {
			fmt.Fprintf(os.Stderr, "%s error sending config: %v\n", logTime(), err)
		}
	}

	// In single-page mode, send current content to newly connected client
	if !EnableTabs {
		contentMutex.RLock()

		if currentHTML != "" {
			contentMsg := map[string]any{
				"HTML":        currentHTML,
				"Title":       currentTitle,
				"Meta":        currentMeta,
				"DocumentURI": "",
			}
			if msgJSON, err := json.Marshal(contentMsg); err == nil {
				if err := conn.WriteMessage(websocket.TextMessage, msgJSON); err != nil {
					fmt.Fprintf(os.Stderr, "%s error sending current content: %v\n", logTime(), err)
				}
			}
		}

		contentMutex.RUnlock()
	}

	// Add client to list AFTER initial messages are sent
	clientsMutex.Lock()
	wasEmpty := len(clients) == 0
	clients = append(clients, conn)
	clientsMutex.Unlock()

	// Signal first client connected
	if wasEmpty {
		select {
		case clientConnected <- struct{}{}:
		default:
		}
	}

	defer func() {
		_ = conn.Close()

		// Remove client from slice
		clientsMutex.Lock()
		for i, c := range clients {
			if c == conn {
				clients = append(clients[:i], clients[i+1:]...)

				break
			}
		}
		clientsMutex.Unlock()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Fprintf(os.Stderr, "%s error while reading message: %v\n", logTime(), err)
			}

			break
		}

		// Try to parse as incoming request from browser
		var incomingMsg struct {
			Type          string `json:"type"`
			URI           string `json:"uri"`
			TakeFocus     bool   `json:"takeFocus"`
			UpdatePreview bool   `json:"updatePreview"`
		}

		if err := json.Unmarshal(msg, &incomingMsg); err == nil {
			// Handle different message types
			if incomingMsg.Type == "openDocument" {
				// Send to LSP request channel
				LSPRequestChan <- OpenDocumentRequest{
					URI:           incomingMsg.URI,
					TakeFocus:     incomingMsg.TakeFocus,
					UpdatePreview: incomingMsg.UpdatePreview,
				}

				continue
			}
		}

		// Unknown message types are ignored (no broadcast needed)
	}
}

func Openbrowser(url, browser string) error {
	var err error

	switch runtime.GOOS {
	case "linux":
		browserCommand := "xdg-open"
		if browser != "" {
			browserCommand = browser
		}

		err = exec.Command(browserCommand, url).Start() //nolint:gosec,noctx // Intentional: fire-and-forget browser launch
	case "windows":
		if browser != "" {
			err = exec.Command(browser, url).Start() //nolint:gosec,noctx // Intentional: fire-and-forget browser launch
		} else {
			err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start() //nolint:gosec,noctx // Intentional: fire-and-forget browser launch
		}
	case "darwin":
		openArgs := []string{"-g", url}
		if browser != "" {
			openArgs = append(openArgs[:1], "-a", browser, url)
		}

		err = exec.Command("open", openArgs...).Start() //nolint:gosec,noctx // Intentional: fire-and-forget browser launch
	}

	if err != nil {
		return err
	}

	return nil
}

func convertMetaToHTMLTable(meta map[string]any) string {
	if len(meta) == 0 {
		return ""
	}

	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var htmlBuilder strings.Builder

	htmlBuilder.WriteString("<table>")
	htmlBuilder.WriteString("<tr><th colspan='2'>Meta</th></tr>")

	for _, k := range keys {
		fmt.Fprintf(&htmlBuilder, "<tr><td>%s</td><td>%s</td></tr>",
			html.EscapeString(k), html.EscapeString(fmt.Sprintf("%v", meta[k])))
	}

	htmlBuilder.WriteString("</table>")

	return htmlBuilder.String()
}
