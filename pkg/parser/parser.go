package parser

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strings"

	img64 "github.com/tenkoh/goldmark-img64"
	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	meta "github.com/yuin/goldmark-meta"
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
			parser.WithAutoHeadingID(),
		),
	)

	var buf bytes.Buffer

	ctx := parser.NewContext()
	if err := markdown.Convert(source, &buf, parser.WithContext(ctx)); err != nil {
		panic(err)
	}

	return buf.String(), meta.Get(ctx)
}
