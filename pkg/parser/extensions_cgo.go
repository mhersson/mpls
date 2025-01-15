//go:build cgo

package parser

import (
	katex "github.com/FurqanSoftware/goldmark-katex"
	img64 "github.com/tenkoh/goldmark-img64"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
)

func defaultExtensions() []goldmark.Extender {
	return []goldmark.Extender{
		extension.GFM,
		highlighting.NewHighlighting(
			highlighting.WithStyle(CodeHighlightingStyle),
		),
		meta.Meta,
		img64.Img64,
		&katex.Extender{},
	}
}
