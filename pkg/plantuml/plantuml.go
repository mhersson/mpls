package plantuml

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const plantumlMap = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_"

var Server string
var BasePath string
var DisableTLS bool

var enc *base64.Encoding

func init() {
	enc = base64.NewEncoding(plantumlMap)
}

type Plantuml struct {
	EncodedUML string
	Diagram    string
}

func encode(text string) string {
	b := new(bytes.Buffer)

	w, _ := flate.NewWriter(b, flate.BestCompression)
	_, _ = w.Write([]byte(text))
	w.Close()

	return enc.EncodeToString(b.Bytes())
}

func call(payload string) ([]byte, error) {
	path, err := url.JoinPath(BasePath, "png", payload)
	if err != nil {
		return nil, err
	}

	scheme := "https"
	if DisableTLS {
		scheme = "http"
	}

	u := url.URL{Host: Server, Scheme: scheme, Path: path}

	timeout := 10 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed get diagram: %w", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

func getDiagram(encodedUML string) (string, error) {
	svg, err := call(encodedUML)

	var buf bytes.Buffer

	buf.Write([]byte(`<img src="data:image/png;base64,`))
	enc := base64.NewEncoder(base64.StdEncoding, &buf)
	_, _ = enc.Write(svg)
	enc.Close()
	buf.Write([]byte(`" alt="plantuml-diagram">`))

	return buf.String(), err
}

func Encode(uml string) string {
	return encode(uml)
}

func GetDiagram(encodedUML string) (string, error) {
	return getDiagram(encodedUML)
}
