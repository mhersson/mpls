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
	oldDocContent         map[string]string
	CodeHighlightingStyle string
	EnableWikiLinks       bool

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

type ScrollIDTransformer struct{}

func (t *ScrollIDTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	currentDocContent := make(map[string]string)

	changedNodes := make(map[ast.Node]bool)

	var walk func(n ast.Node, path string)
	walk = func(n ast.Node, path string) {
		nodeKey := path + ":" + n.Kind().String()
		newContent := string(n.Text(reader.Source()))
		currentDocContent[nodeKey] = newContent

		if oldDocContent != nil {
			if oldContent, existed := oldDocContent[nodeKey]; !existed || oldContent != newContent {
				changedNodes[n] = true
			}
		}

		if n.HasChildren() {
			childIdx := 0
			child := n.FirstChild()

			for child != nil {
				childPath := fmt.Sprintf("%s.%d", path, childIdx)
				walk(child, childPath)
				child = child.NextSibling()
				childIdx++
			}
		}
	}

	walk(doc, "")

	if oldDocContent != nil && len(changedNodes) > 0 {
		var previousIDAbleNode ast.Node

		_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}

			switch n.(type) {
			case *ast.Heading, *ast.Paragraph, *ast.ListItem, *ast.Blockquote, *ast.Text:
				previousIDAbleNode = n
			}

			if changedNodes[n] {
				switch n.(type) {
				case *ast.Heading, *ast.Paragraph, *ast.ListItem, *ast.Blockquote, *ast.Text:
					n.SetAttribute([]byte("id"), []byte(ScrollAnchor))
				default:
					if previousIDAbleNode != nil {
						previousIDAbleNode.SetAttribute([]byte("id"), []byte(ScrollAnchor))
					}
				}
			}

			return ast.WalkContinue, nil
		})
	}

	oldDocContent = currentDocContent
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
				util.Prioritized(&ScrollIDTransformer{}, 100),
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
