package parser

import (
	"bytes"

	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/wikilink"
)

var (
	CodeHighlightingStyle string
	EnableWikiLinks       bool

	EnableFootnotes bool
	EnableEmoji     bool
)

func HTML(document string) string {
	source := []byte(document)

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
