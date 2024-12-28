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
)

var CodeHighlightingStyle string

func HTML(document string) string {
	source := []byte(document)

	markdown := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle(CodeHighlightingStyle),
			),
			meta.Meta,
			img64.Img64,
			&katex.Extender{},
		),
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
