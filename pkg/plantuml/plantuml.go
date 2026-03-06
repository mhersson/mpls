package plantuml

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/base64"
	"fmt"
	htmlpkg "html"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

const plantumlMap = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_"

var (
	Server     string
	BasePath   string
	DisableTLS bool
)

var enc *base64.Encoding

// Diagram cache for avoiding repeated HTTP requests.
var (
	diagramCache      = make(map[string]string) // encodedUML -> diagram HTML
	diagramCacheMutex sync.RWMutex
	maxCacheSize      = 100 // Max cached diagrams
)

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
	_ = w.Close()

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

	resp, err := http.DefaultClient.Do(req) //nolint:gosec // Intentional: PlantUML server URL is user-configurable
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
	// Check cache first
	diagramCacheMutex.RLock()

	if cached, ok := diagramCache[encodedUML]; ok {
		diagramCacheMutex.RUnlock()

		return cached, nil
	}

	diagramCacheMutex.RUnlock()

	// Cache miss - make HTTP request
	svg, err := call(encodedUML)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer

	buf.Write([]byte(`<img src="data:image/png;base64,`))
	enc := base64.NewEncoder(base64.StdEncoding, &buf)
	_, _ = enc.Write(svg)
	_ = enc.Close()

	buf.Write([]byte(`" alt="plantuml-diagram">`))

	result := buf.String()

	// Store in cache
	diagramCacheMutex.Lock()
	if len(diagramCache) >= maxCacheSize {
		// Simple eviction: clear half the cache
		for k := range diagramCache {
			delete(diagramCache, k)

			if len(diagramCache) < maxCacheSize/2 {
				break
			}
		}
	}

	diagramCache[encodedUML] = result
	diagramCacheMutex.Unlock()

	return result, nil
}

func Encode(uml string) string {
	return encode(uml)
}

func GetDiagram(encodedUML string) (string, error) {
	return getDiagram(encodedUML)
}

// ClearDiagramCache clears the diagram cache. Useful for testing.
func ClearDiagramCache() {
	diagramCacheMutex.Lock()
	diagramCache = make(map[string]string)
	diagramCacheMutex.Unlock()
}

// InsertPlantumlDiagram processes HTML to replace PlantUML code blocks with rendered diagrams.
// Uses HTML tokenizer for proper parsing, handling nested tags and various attribute formats.
func InsertPlantumlDiagram(data string, generate bool, plantumls []Plantuml) (string, []Plantuml, error) {
	tokenizer := html.NewTokenizer(strings.NewReader(data))

	var result strings.Builder

	var err error

	numDiagrams := 0

	// State tracking
	var inPre bool

	var inPlantumlCode bool

	var currentPreHasPlantuml bool // tracks if current <pre> block has plantuml

	var preContent strings.Builder // accumulates content while scanning <pre>

	var codeContent strings.Builder // accumulates PlantUML source code

	for {
		tt := tokenizer.Next()

		switch tt {
		case html.ErrorToken:
			// End of document - flush any pending content.
			// If we're mid-PlantUML block (malformed HTML), flush codeContent to preContent first.
			if inPlantumlCode {
				preContent.WriteString(codeContent.String())
			}

			if inPre {
				result.WriteString(preContent.String())
			}

			return result.String(), plantumls, err

		case html.StartTagToken:
			token := tokenizer.Token()

			if token.Data == "pre" {
				inPre = true
				currentPreHasPlantuml = false

				preContent.Reset()
				preContent.WriteString(token.String())

				continue
			}

			if inPre && token.Data == "code" && hasLanguagePlantuml(token.Attr) {
				inPlantumlCode = true
				currentPreHasPlantuml = true

				codeContent.Reset()
				preContent.WriteString(token.String())

				continue
			}

			if inPre {
				preContent.WriteString(token.String())
			} else {
				result.WriteString(token.String())
			}

		case html.EndTagToken:
			token := tokenizer.Token()

			if token.Data == "code" && inPlantumlCode {
				inPlantumlCode = false

				htmlEncodedUml := codeContent.String()
				uml := htmlpkg.UnescapeString(htmlEncodedUml)

				// Only process if the UML content contains @startuml marker
				if !strings.Contains(uml, "@startuml") {
					// Not a valid PlantUML diagram, keep the original code block
					currentPreHasPlantuml = false

					preContent.WriteString(htmlEncodedUml)
					preContent.WriteString(token.String())

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
					// Don't add to preContent - we'll output the diagram when </pre> is reached
				}

				continue
			}

			if token.Data == "pre" && inPre {
				inPre = false

				if currentPreHasPlantuml {
					// We had PlantUML in this pre block - output diagram instead
					if generate {
						result.WriteString(plantumls[numDiagrams-1].Diagram)
					} else if len(plantumls) >= numDiagrams {
						result.WriteString(plantumls[numDiagrams-1].Diagram)
					}
				} else {
					// Regular pre block - output as-is
					preContent.WriteString(token.String())
					result.WriteString(preContent.String())
				}

				continue
			}

			if inPre {
				if inPlantumlCode {
					// Content inside <code class="language-plantuml">
					codeContent.WriteString(token.String())
				} else {
					preContent.WriteString(token.String())
				}
			} else {
				result.WriteString(token.String())
			}

		case html.TextToken:
			raw := string(tokenizer.Raw())

			switch {
			case inPlantumlCode:
				codeContent.WriteString(raw)
			case inPre:
				preContent.WriteString(raw)
			default:
				result.WriteString(raw)
			}

		default:
			raw := string(tokenizer.Raw())

			if inPre {
				if inPlantumlCode {
					codeContent.WriteString(raw)
				} else {
					preContent.WriteString(raw)
				}
			} else {
				result.WriteString(raw)
			}
		}
	}
}

// hasLanguagePlantuml checks if the attributes contain class="language-plantuml".
func hasLanguagePlantuml(attrs []html.Attribute) bool {
	for _, attr := range attrs {
		if attr.Key == "class" {
			// Check if "language-plantuml" is one of the classes
			classes := strings.Fields(attr.Val)

			return slices.Contains(classes, "language-plantuml")
		}
	}

	return false
}
