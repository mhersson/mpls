//go:build !cgo

package parser

import (
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
	}
}
