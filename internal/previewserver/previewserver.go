package previewserver

import (
	"context"
	_ "embed"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed web/index.html
var indexHTML string

//go:embed web/styles.css
var stylesCSS string

//go:embed web/ws.js
var websocketJS string

var (
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

func New() *Server {
	port := rand.Intn(65535-10000) + 10000 // nolint:gosec

	srv := &http.Server{
		Addr:        fmt.Sprintf(":%d", port),
		ReadTimeout: time.Second * 5,
	}

	return &Server{
		Server:         srv,
		InitialContent: fmt.Sprintf(indexHTML, port),
		Port:           port,
	}
}

func (s *Server) Start() {
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, indexHTML)
	})

	http.HandleFunc("/styles.css", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		fmt.Fprint(w, stylesCSS)
	})

	http.HandleFunc("/ws.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		fmt.Fprintf(w, websocketJS, s.Port)
	})

	http.HandleFunc("/ws", handleWebSocket)

	signal.Notify(stopChan, os.Interrupt)

	go handleMessages()

	go func() {
		if err := s.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Error starting server: %s\n", err)
		}
	}()

	_ = openbrowser(fmt.Sprintf("http://localhost:%d", s.Port))

	// Wait for interrupt signal
	<-stopChan
	s.Stop()
}

// Update updates the current HTML content.
func (s *Server) Update(newContent []byte) {
	u := url.URL{Scheme: "ws", Host: s.Server.Addr, Path: "/ws"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Println("Error sending message:", err)

		return
	}
	defer conn.Close()

	// Send a message to the server
	err = conn.WriteMessage(websocket.TextMessage, newContent)
	if err != nil {
		fmt.Println("Error sending message:", err)

		return
	}
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() {
	fmt.Println("\nShutting down server...")

	// Create a context with a timeout for the shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt to gracefully shut down the server
	if err := s.Server.Shutdown(ctx); err != nil {
		fmt.Printf("Error shutting down server: %s\n", err)
	}

	fmt.Println("Server shut down gracefully.")
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
		fmt.Println("Error while upgrading connection:", err)

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
			fmt.Println("Error while reading message:", err)

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
			if err != nil {
				fmt.Println("Error while writing message:", err)
				client.Close()
				delete(clients, client)
			}
		}
		clientsMutex.Unlock()
	}
}

func openbrowser(url string) error {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", "-g", url).Start()
	}

	if err != nil {
		return err
	}

	return nil
}
