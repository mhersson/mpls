package parser

import (
	"bytes"

	katex "github.com/FurqanSoftware/goldmark-katex"
	img64 "github.com/tenkoh/goldmark-img64"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

func HTML(document string) []byte {
	source := []byte(document)

	markdown := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("catppuccin-mocha"),
			),
			meta.Meta,
			img64.Img64,
			&katex.Extender{},
		),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	var buf bytes.Buffer

	if err := markdown.Convert(source, &buf); err != nil {
		panic(err)
	}

	return buf.Bytes()
}
