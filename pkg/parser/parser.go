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

var oldDocContent map[string]string

var (
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

// ScrollIDTransformer adds the mpls-scroll-anchor ID to changed nodes.
type ScrollIDTransformer struct{}

// Transform checks for changed nodes and adds the ID.
func (t *ScrollIDTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	currentDocContent := make(map[string]string)

	var walk func(n ast.Node, path string)
	walk = func(n ast.Node, path string) {
		var content string
		switch n.(type) {
		case *ast.Heading, *ast.Paragraph, *ast.ListItem, *ast.Blockquote, *ast.Text:
			content = string(n.Text(reader.Source()))
		default:
			content = n.Kind().String()
		}

		nodeKey := path + ":" + n.Kind().String()
		currentDocContent[nodeKey] = content

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

	if oldDocContent != nil {
		var markChanged func(n ast.Node, path string)
		markChanged = func(n ast.Node, path string) {
			nodeKey := path + ":" + n.Kind().String()

			oldContent, existedBefore := oldDocContent[nodeKey]
			newContent := currentDocContent[nodeKey]

			if !existedBefore || oldContent != newContent {
				n.SetAttribute([]byte("id"), []byte(ScrollAnchor))
			}

			if n.HasChildren() {
				childIdx := 0
				child := n.FirstChild()

				for child != nil {
					childPath := fmt.Sprintf("%s.%d", path, childIdx)
					markChanged(child, childPath)
					child = child.NextSibling()
					childIdx++
				}
			}
		}

		markChanged(doc, "")
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
