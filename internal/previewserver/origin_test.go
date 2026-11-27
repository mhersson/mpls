package previewserver

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var webSocketURLPattern = regexp.MustCompile(`new WebSocket\("([^"]+)"\)`)

// webSocketURL returns the address the preview's own script, ws.js, connects to.
func webSocketURL(t *testing.T, mux *http.ServeMux) string {
	t.Helper()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ws.js", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	match := webSocketURLPattern.FindStringSubmatch(rec.Body.String())
	require.Len(t, match, 2, "ws.js does not open a WebSocket")

	return match[1]
}

// pageOrigin returns the Origin a browser sends from the page at pageURL.
func pageOrigin(t *testing.T, pageURL string) string {
	t.Helper()

	u, err := url.Parse(pageURL)
	require.NoError(t, err)

	return u.Scheme + "://" + u.Host
}

// TestNew_AddressesMatch checks that the browser is opened at, and ws.js
// connects to, the address the server listens on.
func TestNew_AddressesMatch(t *testing.T) { //nolint:paralleltest // New reads and changes package globals
	savedIndex, savedTheme, savedPort := indexHTML, Theme, FixedPort

	t.Cleanup(func() { indexHTML, Theme, FixedPort = savedIndex, savedTheme, savedPort })

	for _, tc := range []struct { //nolint:paralleltest // subtests share the globals
		name      string
		fixedPort int
	}{
		{name: "default port", fixedPort: 0},
		{name: "port given by --port", fixedPort: 8123},
	} {
		t.Run(tc.name, func(t *testing.T) {
			indexHTML, FixedPort = savedIndex, tc.fixedPort

			s := New()
			if tc.fixedPort > 0 {
				assert.Equal(t, tc.fixedPort, s.Port)
			}

			port := strconv.Itoa(s.Port)
			assert.Equal(t, "127.0.0.1:"+port, s.Server.Addr, "listens on loopback alone")

			pageURL := s.URL("/")
			assert.Equal(t, "http://localhost:"+port+"/", pageURL)
			assert.True(t, isPreviewOrigin(pageOrigin(t, pageURL), s.Port), "page at %s may not connect", pageURL)

			mux := http.NewServeMux()
			s.registerRoutes(mux)
			assert.Equal(t, "ws://localhost:"+port+"/ws", webSocketURL(t, mux))
		})
	}
}

func TestIsPreviewOrigin(t *testing.T) {
	t.Parallel()

	const port = 8123

	for _, tc := range []struct {
		origin string
		want   bool
	}{
		{origin: "http://localhost:8123", want: true},
		{origin: "http://127.0.0.1:8123", want: true},
		{origin: "http://LOCALHOST:8123", want: true},
		{origin: "", want: false},
		{origin: "null", want: false},
		{origin: "http://localhost", want: false},
		{origin: "http://localhost:8124", want: false},
		{origin: "https://localhost:8123", want: false},
		{origin: "http://[::1]:8123", want: false},
		{origin: "http://192.168.1.20:8123", want: false},
		{origin: "http://evil.example:8123", want: false},
		{origin: "http://localhost.evil.example:8123", want: false},
	} {
		t.Run(tc.origin, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, isPreviewOrigin(tc.origin, port))
		})
	}

	assert.True(t, isPreviewOrigin("http://localhost", 80), "default port omitted from origin")
}

func TestWebSocketOrigin(t *testing.T) {
	t.Parallel()

	// Listen where New does, and build the server for that port.
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", net.JoinHostPort(listenHost, "0"))
	require.NoError(t, err)

	mux := http.NewServeMux()
	ts := httptest.NewUnstartedServer(mux)
	require.NoError(t, ts.Listener.Close())
	ts.Listener = ln

	s := &Server{Port: ln.Addr().(*net.TCPAddr).Port, InitialContent: "<html></html>"}
	s.registerRoutes(mux)
	ts.Start()
	t.Cleanup(ts.Close)

	wsURL := webSocketURL(t, mux)
	ownOrigin := pageOrigin(t, s.URL("/"))

	dial := func(t *testing.T, origin string) (*websocket.Conn, *http.Response, error) {
		t.Helper()

		header := http.Header{}
		if origin != "" {
			header.Set("Origin", origin)
		}

		conn, resp, err := websocket.DefaultDialer.DialContext(t.Context(), wsURL, header)
		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			assert.NoError(t, resp.Body.Close())
		}

		return conn, resp, err
	}

	t.Run("another origin is refused", func(t *testing.T) {
		t.Parallel()

		for _, origin := range []string{
			"",
			"null",
			"http://evil.example",
			fmt.Sprintf("http://localhost:%d", s.Port+1),
		} {
			conn, resp, err := dial(t, origin)
			if conn != nil {
				_ = conn.Close()
			}

			require.ErrorIs(t, err, websocket.ErrBadHandshake, "origin %q", origin)
			require.NotNil(t, resp)
			assert.Equal(t, http.StatusForbidden, resp.StatusCode, "origin %q", origin)
		}
	})

	t.Run("the preview's own page is served", func(t *testing.T) {
		t.Parallel()

		for _, origin := range []string{ownOrigin, fmt.Sprintf("http://127.0.0.1:%d", s.Port)} {
			conn, _, err := dial(t, origin)
			require.NoError(t, err, "origin %q", origin)

			var msg struct{ Type string }

			require.NoError(t, conn.ReadJSON(&msg))
			assert.Equal(t, "config", msg.Type)
			require.NoError(t, conn.Close())
		}
	})
}
