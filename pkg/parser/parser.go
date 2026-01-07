package parser

import (
	"bytes"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	img64 "github.com/tenkoh/goldmark-img64"
	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/wikilink"
)

const ScrollAnchor = "mpls-scroll-anchor"

var (
	oldDocContentByURI    map[string]map[string]string // URI -> content map
	CodeHighlightingStyle string
	EnableWikiLinks       bool
	WorkspaceRoot         string

	EnableFootnotes bool
	EnableEmoji     bool
)

func getDocDir(uri string) string {
	return filepath.Dir(NormalizePath(uri))
}

func NormalizePath(uri string) string {
	f := strings.TrimPrefix(uri, "file://")

	if runtime.GOOS == "windows" {
		f = strings.TrimPrefix(uri, "file:///")
		f = filepath.FromSlash(f)
		f = strings.Replace(f, "%3A", ":", 1)
		f = strings.ReplaceAll(f, "%20", " ")
	}

	return f
}

type ScrollIDTransformer struct {
	currentURI string
}

func (t *ScrollIDTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	currentDocContent := make(map[string]string)
	changedNodes := make(map[ast.Node]bool)

	// Initialize the map if needed
	if oldDocContentByURI == nil {
		oldDocContentByURI = make(map[string]map[string]string)
	}

	// Get the old content for this specific document
	oldDocContent := oldDocContentByURI[t.currentURI]

	var walk func(ast.Node, string)
	walk = func(n ast.Node, path string) {
		key := path + ":" + n.Kind().String()
		content := string(n.Text(reader.Source()))
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
			walk(child, fmt.Sprintf("%s.%d", path, i))
		}
	}

	walk(doc, "")

	if len(changedNodes) == 0 {
		oldDocContentByURI[t.currentURI] = currentDocContent

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

	oldDocContentByURI[t.currentURI] = currentDocContent
}

type LinkResolverTransformer struct {
	currentURI string
}

func (t *LinkResolverTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
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

func HTML(document, uri string) (string, map[string]any) {
	source := []byte(document)

	dir := getDocDir(uri)

	extensions := defaultExtensions()

	optionalExtensions := map[goldmark.Extender]bool{
		&wikilink.Extender{}: EnableWikiLinks,
		extension.Footnote:   EnableFootnotes,
		emoji.Emoji:          EnableEmoji,
	}

	for ext, enabled := range optionalExtensions {
		if enabled {
			extensions = append(extensions, ext)
		}
	}

	markdown := goldmark.New(
		goldmark.WithExtensions(extensions...),
		goldmark.WithRendererOptions(
			img64.WithPathResolver(img64.ParentLocalPathResolver(dir)),
			html.WithUnsafe()),
		goldmark.WithParserOptions(
			parser.WithASTTransformers(
				util.Prioritized(&ScrollIDTransformer{currentURI: uri}, 100),
				util.Prioritized(&LinkResolverTransformer{currentURI: uri}, 99),
			),
		),
	)

	var buf bytes.Buffer

	ctx := parser.NewContext()
	if err := markdown.Convert(source, &buf, parser.WithContext(ctx)); err != nil {
		panic(err)
	}

	return buf.String(), meta.Get(ctx)
}
