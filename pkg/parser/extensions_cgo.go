//go:build cgo

package parser //nolint:revive // Package name does not conflict with stdlib (go/parser is different)

import (
	katex "github.com/FurqanSoftware/goldmark-katex"
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
		&katex.Extender{},
		&GitHubAlertExtension{},
	}
}
