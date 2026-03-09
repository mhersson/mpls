package parser

import (
	"bytes"
	"fmt"
	"html"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/wikilink"
)

const (
	ScrollAnchor       = "mpls-scroll-anchor"
	maxDocContentCache = 10
)

var (
	oldDocContentByURI    map[string]map[string]string // URI -> content map
	oldDocContentMutex    sync.RWMutex                 // Protects oldDocContentByURI
	CodeHighlightingStyle string
	EnableWikiLinks       bool
	WorkspaceRoot         string

	EnableFootnotes bool
	EnableEmoji     bool

	// Cached goldmark extensions (initialized once at first use).
	cachedExtensions []goldmark.Extender
	extensionsOnce   sync.Once
)

// getExtensions returns the cached goldmark extensions.
// Extensions are initialized once on first call since config is set at startup
// and never changes during runtime.
func getExtensions() []goldmark.Extender {
	extensionsOnce.Do(func() {
		cachedExtensions = defaultExtensions()
		if EnableWikiLinks {
			cachedExtensions = append(cachedExtensions, &wikilink.Extender{})
		}

		if EnableFootnotes {
			cachedExtensions = append(cachedExtensions, extension.Footnote)
		}

		if EnableEmoji {
			cachedExtensions = append(cachedExtensions, emoji.Emoji)
		}
	})

	return cachedExtensions
}

func getDocDir(uri string) string {
	return filepath.Dir(NormalizePath(uri))
}

func NormalizePath(uri string) string {
	// Windows uses file:/// (3 slashes), Unix uses file:// (2 slashes)
	var f string
	if runtime.GOOS == "windows" {
		f = strings.TrimPrefix(uri, "file:///")
	} else {
		f = strings.TrimPrefix(uri, "file://")
	}

	decoded, err := url.PathUnescape(f)
	if err != nil {
		decoded = f
	}

	if runtime.GOOS == "windows" {
		decoded = filepath.FromSlash(decoded)
	}

	return decoded
}

type ScrollIDTransformer struct {
	currentURI string
	changeLine int // Source line where change occurred (1-based, 0 = use content diff)
}

// buildLineIndex pre-computes line start offsets for fast lookups.
// Returns slice where index i contains the byte offset where line i+1 starts.
func buildLineIndex(source []byte) []int {
	lines := []int{0} // Line 1 starts at offset 0

	for i, b := range source {
		if b == '\n' && i+1 < len(source) {
			lines = append(lines, i+1)
		}
	}

	return lines
}

// offsetToLineWithIndex converts byte offset to 1-based line number using pre-built index.
func offsetToLineWithIndex(lineIndex []int, offset int) int {
	// Binary search for the line containing this offset
	low, high := 0, len(lineIndex)-1
	for low < high {
		mid := (low + high + 1) / 2
		if lineIndex[mid] <= offset {
			low = mid
		} else {
			high = mid - 1
		}
	}

	return low + 1 // Convert to 1-based
}

// findBlockAtLine finds the deepest structural block element at the given line.
func findBlockAtLine(doc *ast.Document, source []byte, targetLine int) ast.Node {
	var target ast.Node

	lineIndex := buildLineIndex(source)

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		// Check for structural block elements with line info
		var lines *text.Segments

		switch block := n.(type) {
		case *ast.Heading:
			lines = block.Lines()
		case *ast.Paragraph:
			lines = block.Lines()
		case *ast.ListItem:
			// ListItem doesn't have Lines(), check children
			return ast.WalkContinue, nil
		case *ast.Blockquote:
			// Blockquote doesn't have Lines(), check children
			return ast.WalkContinue, nil
		case *ast.FencedCodeBlock:
			lines = block.Lines()
		case *ast.CodeBlock:
			lines = block.Lines()
		default:
			return ast.WalkContinue, nil
		}

		if lines != nil && lines.Len() > 0 {
			first := lines.At(0)
			last := lines.At(lines.Len() - 1)
			// Get the line number where this block's content starts and ends
			startLine := offsetToLineWithIndex(lineIndex, first.Start)

			// Early exit: if we've passed the target line and have a match, stop
			if startLine > targetLine+1 && target != nil {
				return ast.WalkStop, nil
			}

			endLine := offsetToLineWithIndex(lineIndex, last.Stop-1)
			// Match if target line is within this block's range
			if targetLine >= startLine && targetLine <= endLine {
				target = n
			} else if _, ok := n.(*ast.FencedCodeBlock); ok && (targetLine == startLine-1 || targetLine == endLine+1) {
				// Fenced code: ``` markers are on lines before/after content
				target = n
			}
		}

		return ast.WalkContinue, nil
	})

	return target
}

func (t *ScrollIDTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()

	// If we have a specific change line, use line-based targeting
	if t.changeLine > 0 {
		target := findBlockAtLine(doc, source, t.changeLine)
		if target != nil {
			target.SetAttribute([]byte("id"), []byte(ScrollAnchor))

			return
		}
		// Fall through to content-diff approach if no block found
	}

	currentDocContent := make(map[string]string)
	changedNodes := make(map[ast.Node]bool)

	// Get the old content for this specific document (with read lock)
	oldDocContentMutex.RLock()

	if oldDocContentByURI == nil {
		oldDocContentMutex.RUnlock()
		oldDocContentMutex.Lock()
		// Double-check after acquiring write lock
		if oldDocContentByURI == nil {
			oldDocContentByURI = make(map[string]map[string]string)
		}
		oldDocContentMutex.Unlock()
		oldDocContentMutex.RLock()
	}

	oldDocContent := oldDocContentByURI[t.currentURI]

	oldDocContentMutex.RUnlock()

	var walk func(ast.Node, string)

	walk = func(n ast.Node, path string) {
		key := path + ":" + n.Kind().String()
		content := string(n.Text(reader.Source())) //nolint:staticcheck // Using deprecated API; refactoring would be extensive
		currentDocContent[key] = content

		if oldDocContent != nil {
			if old, exists := oldDocContent[key]; !exists || old != content {
				changedNodes[n] = true

				for p := n.Parent(); p != nil; p = p.Parent() {
					if _, ok := p.(*ast.ListItem); ok {
						changedNodes[p] = true

						break
					}

					if _, ok := p.(*ast.Paragraph); ok {
						changedNodes[p] = true

						break
					}

					if _, ok := p.(*ast.Heading); ok {
						changedNodes[p] = true

						break
					}

					if _, ok := p.(*ast.Blockquote); ok {
						changedNodes[p] = true

						break
					}
				}
			}
		}

		for i, child := 0, n.FirstChild(); child != nil; i, child = i+1, child.NextSibling() {
			walk(child, path+"."+strconv.Itoa(i))
		}
	}

	walk(doc, "")

	if len(changedNodes) == 0 {
		oldDocContentMutex.Lock()
		oldDocContentByURI[t.currentURI] = currentDocContent
		oldDocContentMutex.Unlock()

		return
	}

	var target ast.Node

	var maxDepth int

	var lastStructural ast.Node

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch n.(type) {
		case *ast.Heading, *ast.Paragraph, *ast.ListItem, *ast.Blockquote:
			lastStructural = n

			if changedNodes[n] {
				depth := 0
				for p := n.Parent(); p != nil; p = p.Parent() {
					depth++
				}

				if depth > maxDepth {
					target, maxDepth = n, depth
				}
			}
		default:
			if changedNodes[n] && target == nil {
				target = lastStructural
			}
		}

		return ast.WalkContinue, nil
	})

	if target != nil {
		target.SetAttribute([]byte("id"), []byte(ScrollAnchor))
	}

	oldDocContentMutex.Lock()
	oldDocContentByURI[t.currentURI] = currentDocContent

	// Evict old entries if cache exceeds limit
	if len(oldDocContentByURI) > maxDocContentCache {
		for k := range oldDocContentByURI {
			delete(oldDocContentByURI, k)

			if len(oldDocContentByURI) < maxDocContentCache/2 {
				break
			}
		}
	}
	oldDocContentMutex.Unlock()
}

type LinkResolverTransformer struct {
	currentURI string
}

func CleanupDocumentContent(uri string) {
	oldDocContentMutex.Lock()
	defer oldDocContentMutex.Unlock()

	if oldDocContentByURI != nil {
		delete(oldDocContentByURI, uri)
	}
}

func (t *LinkResolverTransformer) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		if link, ok := n.(*ast.Link); ok {
			dest := string(link.Destination)

			// Skip external links
			if strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://") {
				return ast.WalkContinue, nil
			}

			// Skip anchor-only links
			if strings.HasPrefix(dest, "#") {
				return ast.WalkContinue, nil
			}

			// Resolve relative link
			resolvedPath := t.resolveRelativeLink(dest)
			if resolvedPath != "" {
				// Add data attributes for JavaScript to intercept
				link.SetAttribute([]byte("data-mpls-internal"), []byte("true"))
				link.SetAttribute([]byte("data-mpls-target"), []byte(resolvedPath))
			}
		}

		return ast.WalkContinue, nil
	})
}

func (t *LinkResolverTransformer) resolveRelativeLink(dest string) string {
	// If no workspace root, can't resolve
	if WorkspaceRoot == "" {
		return ""
	}

	// Split anchor from path
	path := dest
	anchor := ""

	if idx := strings.Index(dest, "#"); idx != -1 {
		path = dest[:idx]
		anchor = dest[idx:]
	}

	// If path is empty (anchor-only), return empty
	if path == "" {
		return ""
	}

	// Get current file's directory
	currentFilePath := NormalizePath(t.currentURI)
	currentDir := filepath.Dir(currentFilePath)

	// Resolve relative to current file
	absolutePath := filepath.Join(currentDir, path)
	absolutePath = filepath.Clean(absolutePath)

	// Convert to workspace-relative path
	normalizedRoot := NormalizePath("file://" + WorkspaceRoot)
	normalizedPath := NormalizePath("file://" + absolutePath)

	// Check if path is within workspace
	if !strings.HasPrefix(normalizedPath, normalizedRoot) {
		return ""
	}

	relativePath, err := filepath.Rel(normalizedRoot, normalizedPath)
	if err != nil {
		return ""
	}

	// Ensure it starts with /
	if !strings.HasPrefix(relativePath, "/") {
		relativePath = "/" + relativePath
	}

	// Add anchor back if present
	if anchor != "" {
		relativePath += anchor
	}

	return relativePath
}

func HTML(document, uri string, changeLine int) (string, map[string]any) {
	source := []byte(document)

	dir := getDocDir(uri)

	markdown := goldmark.New(
		goldmark.WithExtensions(getExtensions()...),
		goldmark.WithRendererOptions(
			goldmarkhtml.WithUnsafe()),
		goldmark.WithParserOptions(
			parser.WithASTTransformers(
				util.Prioritized(&ScrollIDTransformer{currentURI: uri, changeLine: changeLine}, 100),
				util.Prioritized(&LinkResolverTransformer{currentURI: uri}, 99),
			),
		),
	)

	var buf bytes.Buffer

	ctx := parser.NewContext()
	if err := markdown.Convert(source, &buf, parser.WithContext(ctx)); err != nil {
		errorHTML := fmt.Sprintf(
			`<div class="mpls-error"><strong>Markdown parsing error:</strong><pre>%s</pre></div>`,
			html.EscapeString(err.Error()),
		)

		return errorHTML, nil
	}

	// Convert all <img> tags with local paths to base64 data URIs
	htmlOutput := convertHTMLImages(buf.String(), dir)

	return htmlOutput, meta.Get(ctx)
}
