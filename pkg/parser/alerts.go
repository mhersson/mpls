package parser

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// AlertType represents the type of a GitHub-style alert.
type AlertType int

const (
	AlertNote AlertType = iota
	AlertTip
	AlertImportant
	AlertWarning
	AlertCaution
)

// alertTags maps the raw tag text (as it appears in a text node, e.g.
// "!NOTE") to its AlertType.
var alertTags = map[string]AlertType{
	"!NOTE":      AlertNote,
	"!TIP":       AlertTip,
	"!IMPORTANT": AlertImportant,
	"!WARNING":   AlertWarning,
	"!CAUTION":   AlertCaution,
}

// String returns the display label for the alert type.
func (t AlertType) String() string {
	switch t {
	case AlertNote:
		return "Note"
	case AlertTip:
		return "Tip"
	case AlertImportant:
		return "Important"
	case AlertWarning:
		return "Warning"
	case AlertCaution:
		return "Caution"
	default:
		return "Note"
	}
}

// CSSClass returns the CSS class suffix for the alert type.
func (t AlertType) CSSClass() string { return strings.ToLower(t.String()) }

// SVGIcon returns the GitHub-matching SVG icon for the alert type.
func (t AlertType) SVGIcon() string {
	switch t {
	case AlertNote:
		return `<svg class="github-alert-icon" viewBox="0 0 16 16" version="1.1" width="16" height="16" aria-hidden="true"><path d="M0 8a8 8 0 1 1 16 0A8 8 0 0 1 0 8Zm8-6.5a6.5 6.5 0 1 0 0 13 6.5 6.5 0 0 0 0-13ZM6.5 7.75A.75.75 0 0 1 7.25 7h1a.75.75 0 0 1 .75.75v2.75h.25a.75.75 0 0 1 0 1.5h-2a.75.75 0 0 1 0-1.5h.25v-2h-.25a.75.75 0 0 1-.75-.75ZM8 6a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z"></path></svg>`
	case AlertTip:
		return `<svg class="github-alert-icon" viewBox="0 0 16 16" version="1.1" width="16" height="16" aria-hidden="true"><path d="M8 1.5c-2.363 0-4 1.69-4 3.75 0 .984.424 1.625.984 2.304l.214.253c.223.264.47.556.673.848.284.411.537.896.621 1.49a.75.75 0 0 1-1.484.211c-.04-.282-.163-.547-.37-.847a8.456 8.456 0 0 0-.542-.68c-.084-.1-.173-.205-.268-.32C3.201 7.75 2.5 6.766 2.5 5.25 2.5 2.31 4.863 0 8 0s5.5 2.31 5.5 5.25c0 1.516-.701 2.5-1.328 3.259-.095.115-.184.22-.268.319-.207.245-.383.453-.541.681-.208.3-.33.565-.37.847a.751.751 0 0 1-1.485-.212c.084-.593.337-1.078.621-1.489.203-.292.45-.584.673-.848.075-.088.147-.173.213-.253.561-.679.985-1.32.985-2.304 0-2.06-1.637-3.75-4-3.75ZM5.75 12h4.5a.75.75 0 0 1 0 1.5h-4.5a.75.75 0 0 1 0-1.5ZM6 15.25a.75.75 0 0 1 .75-.75h2.5a.75.75 0 0 1 0 1.5h-2.5a.75.75 0 0 1-.75-.75Z"></path></svg>`
	case AlertImportant:
		return `<svg class="github-alert-icon" viewBox="0 0 16 16" version="1.1" width="16" height="16" aria-hidden="true"><path d="M0 1.75C0 .784.784 0 1.75 0h12.5C15.216 0 16 .784 16 1.75v9.5A1.75 1.75 0 0 1 14.25 13H8.06l-2.573 2.573A1.458 1.458 0 0 1 3 14.543V13H1.75A1.75 1.75 0 0 1 0 11.25Zm1.75-.25a.25.25 0 0 0-.25.25v9.5c0 .138.112.25.25.25h2a.75.75 0 0 1 .75.75v2.19l2.72-2.72a.749.749 0 0 1 .53-.22h6.5a.25.25 0 0 0 .25-.25v-9.5a.25.25 0 0 0-.25-.25Zm7 2.25v2.5a.75.75 0 0 1-1.5 0v-2.5a.75.75 0 0 1 1.5 0ZM9 9a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z"></path></svg>`
	case AlertWarning:
		return `<svg class="github-alert-icon" viewBox="0 0 16 16" version="1.1" width="16" height="16" aria-hidden="true"><path d="M6.457 1.047c.659-1.234 2.427-1.234 3.086 0l6.082 11.378A1.75 1.75 0 0 1 14.082 15H1.918a1.75 1.75 0 0 1-1.543-2.575Zm1.763.707a.25.25 0 0 0-.44 0L1.698 13.132a.25.25 0 0 0 .22.368h12.164a.25.25 0 0 0 .22-.368Zm.53 3.996v2.5a.75.75 0 0 1-1.5 0v-2.5a.75.75 0 0 1 1.5 0ZM9 11a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z"></path></svg>`
	case AlertCaution:
		return `<svg class="github-alert-icon" viewBox="0 0 16 16" version="1.1" width="16" height="16" aria-hidden="true"><path d="M4.47.22A.749.749 0 0 1 5 0h6c.199 0 .389.079.53.22l4.25 4.25c.141.14.22.331.22.53v6a.749.749 0 0 1-.22.53l-4.25 4.25A.749.749 0 0 1 11 16H5a.749.749 0 0 1-.53-.22L.22 11.53A.749.749 0 0 1 0 11V5c0-.199.079-.389.22-.53Zm.84 1.28L1.5 5.31v5.38l3.81 3.81h5.38l3.81-3.81V5.31L10.69 1.5ZM8 4a.75.75 0 0 1 .75.75v3.5a.75.75 0 0 1-1.5 0v-3.5A.75.75 0 0 1 8 4Zm0 8a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z"></path></svg>`
	default:
		return ""
	}
}

// KindGitHubAlert is the NodeKind for a GitHub alert node.
var KindGitHubAlert = ast.NewNodeKind("GitHubAlert")

// GitHubAlertNode is a custom AST node representing a GitHub-style alert.
type GitHubAlertNode struct {
	ast.BaseBlock
	AlertType AlertType
}

// Kind returns the kind of this node.
func (n *GitHubAlertNode) Kind() ast.NodeKind {
	return KindGitHubAlert
}

// Dump dumps the node for debugging.
func (n *GitHubAlertNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"AlertType": n.AlertType.String(),
	}, nil)
}

// GitHubAlertTransformer is an AST transformer that converts blockquotes with
// [!TYPE] markers into GitHubAlertNode nodes.
type GitHubAlertTransformer struct{}

// detectAlert checks the inline text nodes of a paragraph for a GitHub alert
// tag. Goldmark parses "[!WARNING]" into three text nodes: "[", "!WARNING",
// "]". We look for a text node whose content starts with "!" and matches a
// known tag.
func detectAlert(para *ast.Paragraph, source []byte) (AlertType, bool) {
	var alertType AlertType

	for i, child := 0, para.FirstChild(); i < 3 && child != nil; i++ {
		textNode, ok := child.(*ast.Text)
		if !ok {
			return 0, false
		}

		content := strings.TrimSpace(string(textNode.Segment.Value(source)))

		switch i {
		case 0:
			if content != "[" {
				return 0, false
			}

			child = child.NextSibling()

		case 1:
			alertType, ok = alertTags[content]
			if !ok {
				return 0, false
			}

			child = child.NextSibling()

		case 2:
			if content != "]" {
				return 0, false
			}

			return alertType, true
		}
	}

	return 0, false
}

type transformEntry struct {
	bq        *ast.Blockquote
	alertType AlertType
}

// Transform walks the AST and replaces qualifying blockquotes with alert nodes.
func (t *GitHubAlertTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	var entries []transformEntry

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		bq, ok := n.(*ast.Blockquote)
		if !ok {
			return ast.WalkContinue, nil
		}

		para, ok := bq.FirstChild().(*ast.Paragraph)
		if !ok {
			return ast.WalkContinue, nil
		}

		alertType, found := detectAlert(para, reader.Source())
		if !found {
			return ast.WalkContinue, nil
		}

		entries = append(entries, transformEntry{bq: bq, alertType: alertType})

		return ast.WalkSkipChildren, nil
	})

	for _, e := range entries {
		t.transformBlockquote(e.bq, e.alertType)
	}
}

func (t *GitHubAlertTransformer) transformBlockquote(bq *ast.Blockquote, alertType AlertType) {
	// Remove the "[", "!TAG", and "]" text nodes
	para := bq.FirstChild().(*ast.Paragraph)
	for range 3 {
		// detectAlert guarantees there will be at least 3 children
		if child := para.FirstChild(); child != nil {
			para.RemoveChild(para, child)
		}
	}

	// Move all remaining nodes to the new alert node
	alertNode := &GitHubAlertNode{AlertType: alertType}

	for child := bq.FirstChild(); child != nil; {
		next := child.NextSibling()
		bq.RemoveChild(bq, child)
		alertNode.AppendChild(alertNode, child)
		child = next
	}

	// Replace the blockquote with an alert node
	bq.Parent().ReplaceChild(bq.Parent(), bq, alertNode)
}

// GitHubAlertRenderer renders GitHubAlertNode nodes to HTML.
type GitHubAlertRenderer struct{}

// RegisterFuncs registers the renderer function for GitHubAlertNode.
func (r *GitHubAlertRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindGitHubAlert, r.renderAlert)
}

func (r *GitHubAlertRenderer) renderAlert(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</div>\n")

		return ast.WalkContinue, nil
	}

	alertNode := node.(*GitHubAlertNode)

	_, _ = w.WriteString(`<div class="markdown-alert markdown-alert-`)
	_, _ = w.WriteString(alertNode.AlertType.CSSClass())
	_, _ = w.WriteString("\">\n")
	_, _ = w.WriteString(`<p class="markdown-alert-title">`)
	_, _ = w.WriteString(alertNode.AlertType.SVGIcon())
	_, _ = w.WriteString(alertNode.AlertType.String())
	_, _ = w.WriteString("</p>\n")

	return ast.WalkContinue, nil
}

// GitHubAlertExtension is a Goldmark extension that adds support for
// GitHub-style alerts (admonitions).
type GitHubAlertExtension struct{}

// Extend adds the GitHub alert transformer and renderer to Goldmark.
func (e *GitHubAlertExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithASTTransformers(
			util.Prioritized(&GitHubAlertTransformer{}, 50),
		),
	)

	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(&GitHubAlertRenderer{}, 50),
		),
	)
}
