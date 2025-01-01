package parser

import (
	"bytes"

	katex "github.com/FurqanSoftware/goldmark-katex"
	img64 "github.com/tenkoh/goldmark-img64"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/wikilink"
)

var (
	CodeHighlightingStyle string
	EnableWikiLinks       bool
)

func HTML(document string) string {
	source := []byte(document)

	extensions := []goldmark.Extender{
		extension.GFM,
		highlighting.NewHighlighting(
			highlighting.WithStyle(CodeHighlightingStyle),
		),
		meta.Meta,
		img64.Img64,
		&katex.Extender{},
	}

	if EnableWikiLinks {
		extensions = append(extensions, &wikilink.Extender{})
	}

	markdown := goldmark.New(
		goldmark.WithExtensions(extensions...),
		goldmark.WithRendererOptions(html.WithUnsafe()),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)

	var buf bytes.Buffer

	if err := markdown.Convert(source, &buf); err != nil {
		panic(err)
	}

	return buf.String()
}
