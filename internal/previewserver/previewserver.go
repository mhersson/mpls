package previewserver

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	Browser              string
	DarkMode             bool
	FixedPort            int
	OpenBrowserOnStartup bool

	//go:embed web/index.html
	indexHTML string
	//go:embed web/styles.css
	stylesCSS string
	//go:embed web/colors-dark.css
	colorsDarkCSS string
	//go:embed web/colors-light.css
	colorsLightCSS string
	//go:embed web/ws.js
	websocketJS string

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

func New() *Server {
	port := rand.Intn(65535-10000) + 10000 //nolint:gosec
	if FixedPort > 0 {
		port = FixedPort
	}

	theme := "colors-light.css"
	mermaidTheme := "default"

	if DarkMode {
		theme = "colors-dark.css"
		mermaidTheme = "dark"
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
	http.HandleFunc("/colors-light.css", handleResponse("text/css", colorsLightCSS))
	http.HandleFunc("/colors-dark.css", handleResponse("text/css", colorsDarkCSS))
	http.HandleFunc("/ws.js", handleResponse("application/javascript", fmt.Sprintf(websocketJS, s.Port)))

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
func (s *Server) Update(filename, newContent, section string, meta map[string]interface{}) {
	u := url.URL{Scheme: "ws", Host: s.Server.Addr, Path: "/ws"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s error connecting to server: %v\n", logTime(), err)

		return
	}
	defer conn.Close()

	type Event struct {
		HTML    string
		Section string
		Title   string
		Meta    string
	}

	t := strings.TrimSuffix(filename, ".md")
	m := convertMetaToHTMLTable(meta)

	e := Event{HTML: newContent, Section: section, Title: t, Meta: m}
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

func convertMetaToHTMLTable(meta map[string]interface{}) string {
	if len(meta) == 0 {
		return ""
	}

	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	html := "<table>"
	html += "<tr><th colspan='2'>Meta</th></tr>"
	for _, k := range keys {
		html += fmt.Sprintf("<tr><td>%s</td><td>%v</td></tr>", k, meta[k])
	}
	html += "</table>"

	return html
}
