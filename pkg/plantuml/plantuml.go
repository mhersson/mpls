package plantuml

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const plantumlMap = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_"

var (
	Server     string
	BasePath   string
	DisableTLS bool
)

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

// InsertPlantumlDiagram processes HTML to replace PlantUML code blocks with rendered diagrams.
func InsertPlantumlDiagram(data string, generate bool, plantumls []Plantuml) (string, []Plantuml, error) {
	const startDelimiter = `<pre><code class="language-plantuml">`

	var builder strings.Builder

	var err error

	numDiagrams := 0
	start := 0

	for {
		s, e := extractPlantUMLSection(data[start:])
		if s == -1 || e == -1 {
			builder.WriteString(data[start:])

			break
		}

		builder.WriteString(data[start : start+s])

		htmlEncodedUml := data[start+s+len(startDelimiter) : start+e]
		uml := html.UnescapeString(htmlEncodedUml)

		// Only process if the UML content contains @startuml marker
		// This prevents accidental rendering of syntax examples or incomplete diagrams
		if !strings.Contains(uml, "@startuml") {
			// Not a valid PlantUML diagram, keep the original code block
			builder.WriteString(data[start+s : start+e+13])
			start += e + 13

			continue
		}

		p := Plantuml{}
		p.EncodedUML = Encode(uml)

		generated := false

		for _, enc := range plantumls {
			if p.EncodedUML == enc.EncodedUML {
				p.Diagram = enc.Diagram
				generated = true

				break
			}
		}

		if !generated && generate {
			p.Diagram, err = GetDiagram(p.EncodedUML)
			if err != nil {
				return data, plantumls, err
			}
		}

		numDiagrams++

		if generate {
			if len(plantumls) < numDiagrams {
				plantumls = append(plantumls, p)
			} else {
				plantumls[numDiagrams-1] = p
			}

			builder.WriteString(p.Diagram)
		} else if len(plantumls) >= numDiagrams {
			// Use existing until we save and generate a new one
			builder.WriteString(plantumls[numDiagrams-1].Diagram)
		}

		start += e + 13
	}

	return builder.String(), plantumls, nil
}

func extractPlantUMLSection(text string) (int, int) {
	const startDelimiter = `<pre><code class="language-plantuml">`

	const endDelimiter = "</code></pre>"

	startIndex := strings.Index(text, startDelimiter)
	if startIndex == -1 {
		return -1, -1
	}

	endIndex := strings.Index(text[startIndex+len(startDelimiter):], endDelimiter)
	if endIndex == -1 {
		return startIndex, -1
	}

	// Calculate the actual end index in the original text
	endIndex += startIndex + len(startDelimiter)

	return startIndex, endIndex
}
