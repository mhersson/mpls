//go:build cgo

package parser //nolint:revive

import (
	"bytes"

	katex "github.com/FurqanSoftware/goldmark-katex"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// testHookRender is called on every cache-miss render. Tests use it to count
// real katex.Render invocations without stubbing CGO. Test-only hook; tests
// must serialise access (see //nolint:paralleltest in the test file) since
// this var is unsynchronised.
var testHookRender func(equation []byte, display bool)

type mplsKatexExtender struct {
	ThrowOnError bool
}

func (e *mplsKatexExtender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(
		util.Prioritized(&katex.Parser{}, 0),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&mplsKatexRenderer{throwOnError: e.ThrowOnError}, 0),
	))
}

type mplsKatexRenderer struct {
	throwOnError bool
}

func (r *mplsKatexRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(katex.KindInline, r.renderInline)
	reg.Register(katex.KindBlock, r.renderBlock)
}

func (r *mplsKatexRenderer) renderInline(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	node := n.(*katex.Inline)
	key := "i:" + string(node.Equation)

	if cached, ok := katexCacheGet(key); ok {
		_, _ = w.Write(cached)

		return ast.WalkContinue, nil
	}

	var buf bytes.Buffer

	if err := katex.Render(&buf, node.Equation, false, r.throwOnError); err != nil {
		return ast.WalkStop, err
	}

	if testHookRender != nil {
		testHookRender(node.Equation, false)
	}

	html := buf.Bytes()
	katexCacheSet(key, html)
	_, _ = w.Write(html)

	return ast.WalkContinue, nil
}

func (r *mplsKatexRenderer) renderBlock(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	node := n.(*katex.Block)
	key := "b:" + string(node.Equation)

	if cached, ok := katexCacheGet(key); ok {
		_, _ = w.WriteString("<div>")
		_, _ = w.Write(cached)
		_, _ = w.WriteString("</div>")

		return ast.WalkContinue, nil
	}

	var buf bytes.Buffer

	if err := katex.Render(&buf, node.Equation, true, r.throwOnError); err != nil {
		return ast.WalkStop, err
	}

	if testHookRender != nil {
		testHookRender(node.Equation, true)
	}

	html := buf.Bytes()
	katexCacheSet(key, html)

	_, _ = w.WriteString("<div>")
	_, _ = w.Write(html)
	_, _ = w.WriteString("</div>")

	return ast.WalkContinue, nil
}
