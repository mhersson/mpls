package previewserver

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Current version of katex used: 0.16.25 (https://cdn.jsdelivr.net/npm/katex@0.16.25/dist/katex.min.css)
// Current version of mermaid used: 11.12.1 (https://cdn.jsdelivr.net/npm/mermaid@11.12.1/dist/mermaid.min.js)

var (
	Browser              string
	Theme                string
	FixedPort            int
	OpenBrowserOnStartup bool

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
	//go:embed web/fonts
	katexFontsFS embed.FS
	//go:embed web/themes
	themesFS embed.FS

	broadcast    = make(chan []byte)
	clients      = make(map[*websocket.Conn]bool)
	clientsMutex sync.Mutex
	stopChan     = make(chan os.Signal, 1)
)

type Server struct {
	Server         *http.Server
	InitialContent string
	Port           int
}

func logTime() string {
	return time.Now().Local().Format("2006-01-02 15:04:05")
}

func ListThemes() {
	fmt.Println("Available themes:")
	fmt.Println()

	aliases := map[string]string{
		"light": "default-light.css",
		"dark":  "default-dark.css",
	}

	entries, err := fs.ReadDir(themesFS, "web/themes")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading themes: %v\n", err)

		return
	}

	fmt.Println("Aliases:")

	for alias, filename := range aliases {
		fmt.Printf("  %-20s -> %s\n", alias, filename)
	}

	fmt.Println()

	fmt.Println("All themes:")

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".css") {
			themeName := strings.TrimSuffix(entry.Name(), ".css")
			_, mermaidTheme := getThemeConfig(themeName)

			fmt.Printf("  %-20s (mermaid: %s)\n", themeName, mermaidTheme)
		}
	}
}

func WaitForClients(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for clients to connect")
		case <-ticker.C:
			if len(clients) > 0 {
				return nil
			}
		}
	}
}

// GetChromaStyleForTheme returns a recommended chroma syntax highlighting style for a given theme.
// Since theme names match chroma conventions, most themes return their name directly.
func GetChromaStyleForTheme(themeName string) string {
	// Map common theme aliases to actual filenames first
	themeMap := map[string]string{
		"light": "default-light",
		"dark":  "default-dark",
	}

	actualThemeName := themeName
	if mapped, exists := themeMap[themeName]; exists {
		actualThemeName = mapped
	}

	// Special cases where mpls theme name differs from chroma style name
	// or where there's no exact chroma match
	specialCases := map[string]string{
		"ayu-dark":        "github-dark",      // No exact ayu in chroma, github-dark is clean
		"ayu-light":       "github",           // No exact ayu in chroma, github is clean
		"default-dark":    "catppuccin-mocha", // Default dark → catppuccin-mocha
		"default-light":   "catppuccin-latte", // Default light → catppuccin-latte
		"everforest-dark": "evergarden",       // No everforest in chroma → evergarden
		"gruvbox-dark":    "gruvbox",          // Chroma uses "gruvbox" for dark variant
		"tokyonight":      "tokyonight-night", // Base variant maps to -night
	}

	if chromaStyle, exists := specialCases[actualThemeName]; exists {
		return chromaStyle
	}

	// For all other themes, the theme name matches the chroma style name directly
	// (catppuccin-mocha, catppuccin-frappe, nord, dracula, rose-pine, etc.)
	return actualThemeName
}

func getThemeConfig(themeName string) (cssFile, mermaidTheme string) {
	// Map common theme aliases to actual filenames
	themeMap := map[string]string{
		"light": "default-light",
		"dark":  "default-dark",
	}

	// Use mapped name if it exists, otherwise use the theme name as-is
	actualThemeName := themeName
	if mapped, exists := themeMap[themeName]; exists {
		actualThemeName = mapped
	}

	cssFile = fmt.Sprintf("themes/%s.css", actualThemeName)

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
	if strings.Contains(actualThemeName, "dark") {
		mermaidTheme = "dark"
	}

	// Check for dark themes without "dark" in their name
	if slices.Contains(darkThemesWithoutDarkInName, actualThemeName) {
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
		fmt.Fprintf(os.Stderr, "%s Warning: theme '%s' not found, falling back to default-light\n", logTime(), Theme)

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

func (s *Server) Start() {
	http.HandleFunc("/", handleResponse("text/html", s.InitialContent))
	http.HandleFunc("/styles.css", handleResponse("text/css", stylesCSS))
	http.HandleFunc("/katex.min.css", handleResponse("text/css", katexMinCSS))
	http.HandleFunc("/mermaid.min.js", handleResponse("application/javascript", mermaid))
	http.HandleFunc("/ws.js", handleResponse("application/javascript", fmt.Sprintf(websocketJS, s.Port)))

	// Serve embedded KaTeX fonts
	fontsSubFS, _ := fs.Sub(katexFontsFS, "web/fonts")
	http.Handle("/fonts/", http.StripPrefix("/fonts/", http.FileServer(http.FS(fontsSubFS))))

	// Serve embedded themes
	themesSubFS, _ := fs.Sub(themesFS, "web/themes")
	http.Handle("/themes/", http.StripPrefix("/themes/", http.FileServer(http.FS(themesSubFS))))

	http.HandleFunc("/ws", handleWebSocket)

	signal.Notify(stopChan, os.Interrupt)

	go handleMessages()

	go func() {
		if err := s.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("%s error starting server: %s\n", logTime(), err)
		}
	}()

	if OpenBrowserOnStartup {
		err := Openbrowser(fmt.Sprintf("http://localhost:%d", s.Port), Browser)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s error opening browser: %v\n", logTime(), err)
		}
	}

	// Wait for interrupt signal
	<-stopChan
	s.Stop()
}

// Update updates the current HTML content.
func (s *Server) Update(filename, newContent string, meta map[string]any) {
	u := url.URL{Scheme: "ws", Host: s.Server.Addr, Path: "/ws"}

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s error connecting to server: %v\n", logTime(), err)

		return
	}

	defer conn.Close()

	type Event struct {
		HTML  string
		Title string
		Meta  string
	}

	t := strings.TrimSuffix(filename, ".md")
	m := convertMetaToHTMLTable(meta)

	e := Event{HTML: newContent, Title: t, Meta: m}

	eventJSON, err := json.Marshal(e)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling event to JSON: %v\n", err)

		return
	}

	// Send a message to the server
	err = conn.WriteMessage(websocket.TextMessage, eventJSON)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s error sending message: %v\n", logTime(), err)

		return
	}
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

	defer func() {
		conn.Close()
		clientsMutex.Lock()
		delete(clients, conn)
		clientsMutex.Unlock()
	}()

	clientsMutex.Lock()
	clients[conn] = true
	clientsMutex.Unlock()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Fprintf(os.Stderr, "%s error while reading message: %v\n", logTime(), err)
			}

			break
		}
		broadcast <- msg
	}
}

func handleMessages() {
	for {
		msg := <-broadcast

		clientsMutex.Lock()
		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, msg)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Fprintf(os.Stderr, "%s error while writing message: %v\n", logTime(), err)
				client.Close()
				delete(clients, client)
			}
		}
		clientsMutex.Unlock()
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

		err = exec.Command(browserCommand, url).Start()
	case "windows":
		if browser != "" {
			err = exec.Command(browser, url).Start()
		} else {
			err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
		}
	case "darwin":
		openArgs := []string{"-g", url}
		if browser != "" {
			openArgs = append(openArgs[:1], "-a", browser, url)
		}

		err = exec.Command("open", openArgs...).Start()
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

	var html strings.Builder

	html.WriteString("<table>")
	html.WriteString("<tr><th colspan='2'>Meta</th></tr>")

	for _, k := range keys {
		fmt.Fprintf(&html, "<tr><td>%s</td><td>%v</td></tr>", k, meta[k])
	}

	html.WriteString("</table>")

	return html.String()
}
