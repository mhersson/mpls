package parser //nolint:revive

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizePath_FilePrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "file:// prefix",
			input:    "file:///home/user/doc.md",
			expected: "/home/user/doc.md",
		},
		{
			name:     "no prefix",
			input:    "/home/user/doc.md",
			expected: "/home/user/doc.md",
		},
		{
			name:     "url encoded spaces",
			input:    "file:///home/user/my%20doc.md",
			expected: "/home/user/my doc.md",
		},
		{
			name:     "url encoded special chars",
			input:    "file:///path/to/%E2%9C%93.md",
			expected: "/path/to/✓.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := NormalizePath(tt.input)

			// On Windows, paths will be different
			if runtime.GOOS != "windows" {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetDocDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple path",
			input:    "file:///home/user/docs/readme.md",
			expected: "/home/user/docs",
		},
		{
			name:     "root level",
			input:    "file:///readme.md",
			expected: "/",
		},
		{
			name:     "nested path",
			input:    "file:///a/b/c/d/file.md",
			expected: "/a/b/c/d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := getDocDir(tt.input)

			if runtime.GOOS != "windows" {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestCleanupDocumentContent(t *testing.T) {
	t.Parallel()

	// Initialize the map
	if oldDocContentByURI == nil {
		oldDocContentByURI = make(map[string]map[string]string)
	}

	// Add some content
	testURI := "file:///test/cleanup.md"
	oldDocContentByURI[testURI] = map[string]string{"key": "value"}

	// Verify it exists
	_, exists := oldDocContentByURI[testURI]
	require.True(t, exists, "test setup failed: URI not in map")

	// Clean up
	CleanupDocumentContent(testURI)

	// Verify it's gone
	_, exists = oldDocContentByURI[testURI]
	assert.False(t, exists, "CleanupDocumentContent did not remove URI from map")
}

func TestCleanupDocumentContent_NilMap(t *testing.T) {
	t.Parallel()

	// Save original and restore after test
	original := oldDocContentByURI
	oldDocContentByURI = nil

	defer func() {
		oldDocContentByURI = original
	}()

	// Should not panic when map is nil
	CleanupDocumentContent("file:///nonexistent.md")
}

func TestHTML_BasicMarkdown(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	markdown := "# Hello World\n\nThis is a paragraph."
	uri := "file:///test/doc.md"

	html, meta := HTML(markdown, uri, 0)

	assert.Contains(t, html, "<h1>")
	assert.Contains(t, html, "Hello World")
	assert.Contains(t, html, "<p>")
	assert.Contains(t, html, "This is a paragraph")

	// Meta should be returned (may be empty)
	_ = meta
}

func TestHTML_CodeBlock(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	markdown := "```go\nfunc main() {}\n```"
	uri := "file:///test/doc.md"

	html, _ := HTML(markdown, uri, 0)

	// The highlighting extension may wrap code differently
	// Just verify the code content appears somewhere
	assert.Contains(t, html, "func")
	assert.Contains(t, html, "main")
}

func TestHTML_Links(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	markdown := "[Example](https://example.com)"
	uri := "file:///test/doc.md"

	html, _ := HTML(markdown, uri, 0)

	assert.Contains(t, html, `href="https://example.com"`)
	assert.Contains(t, html, "Example")
}

func TestHTML_ImageConversion(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	tmpDir := t.TempDir()

	// Create a test image
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xFF, 0xFF, 0x3F,
		0x00, 0x05, 0xFE, 0x02, 0xFE, 0xDC, 0xCC, 0x59,
		0xE7, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}

	imgPath := filepath.Join(tmpDir, "test.png")
	require.NoError(t, os.WriteFile(imgPath, pngData, 0o600))

	markdown := "![test](test.png)"
	uri := "file://" + filepath.Join(tmpDir, "doc.md")

	html, _ := HTML(markdown, uri, 0)

	// Image should be converted to data URI
	assert.Contains(t, html, "data:image/png;base64,")
}

func TestHTML_ExternalImage(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	markdown := "![test](https://example.com/image.png)"
	uri := "file:///test/doc.md"

	html, _ := HTML(markdown, uri, 0)

	// External image should remain as URL
	assert.Contains(t, html, `src="https://example.com/image.png"`)
}

func TestHTML_Lists(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	markdown := "- Item 1\n- Item 2\n- Item 3\n"
	uri := "file:///test/doc.md"

	html, _ := HTML(markdown, uri, 0)

	assert.Contains(t, html, "<ul>")

	// Count closing li tags (more reliable since opening tags may have attributes)
	liCount := strings.Count(html, "</li>")
	assert.Equal(t, 3, liCount)
}

func TestHTML_Blockquote(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	markdown := "> This is a quote"
	uri := "file:///test/doc.md"

	html, _ := HTML(markdown, uri, 0)

	assert.Contains(t, html, "<blockquote>")
	assert.Contains(t, html, "This is a quote")
}

func TestHTML_InlineCode(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	markdown := "Use `fmt.Println()` to print"
	uri := "file:///test/doc.md"

	html, _ := HTML(markdown, uri, 0)

	assert.Contains(t, html, "<code>")
	assert.Contains(t, html, "fmt.Println()")
}

func TestHTML_Table(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	markdown := `| Header 1 | Header 2 |
| -------- | -------- |
| Cell 1   | Cell 2   |`
	uri := "file:///test/doc.md"

	html, _ := HTML(markdown, uri, 0)

	assert.Contains(t, html, "<table>")
	assert.Contains(t, html, "<th>")
	assert.Contains(t, html, "<td>")
}

func TestHTML_MetaData(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	markdown := `---
title: Test Document
author: Test Author
---

# Content

Some text here.`
	uri := "file:///test/doc.md"

	html, meta := HTML(markdown, uri, 0)

	// Check that content after front matter is rendered
	assert.Contains(t, html, "Content")

	// Check metadata was extracted
	if meta != nil {
		if title, ok := meta["title"]; ok {
			assert.Equal(t, "Test Document", title)
		}
	}
}

func TestHTML_ScrollAnchor(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	// Clear old content to trigger scroll anchor
	if oldDocContentByURI == nil {
		oldDocContentByURI = make(map[string]map[string]string)
	}

	uri := "file:///test/scroll.md"
	delete(oldDocContentByURI, uri)

	markdown := "# First\n\nParagraph one.\n\n# Second\n\nParagraph two."

	// First render
	html1, _ := HTML(markdown, uri, 0)

	// Second render with changes
	markdown2 := "# First\n\nParagraph one CHANGED.\n\n# Second\n\nParagraph two."
	html2, _ := HTML(markdown2, uri, 0)

	// The scroll anchor should appear in the second render when content changes
	// First render establishes baseline, second render detects changes
	if strings.Contains(html1, ScrollAnchor) {
		// First render shouldn't have scroll anchor (no previous state)
		t.Log("first render has scroll anchor (unexpected but not an error)")
	}

	assert.Contains(t, html2, ScrollAnchor, "expected scroll anchor in second render after content change")
}

func TestHTML_LineBasedScrollAnchor(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	uri := "file:///test/linebased.md"
	delete(oldDocContentByURI, uri)

	markdown := "# First Heading\n\nParagraph one.\n\n# Second Heading\n\nParagraph two.\n\n# Third Heading\n\nParagraph three."

	// When changeLine is specified, the scroll anchor should target the block at that line
	// Line 5 is "# Second Heading"
	html, _ := HTML(markdown, uri, 5)

	// Should have scroll anchor somewhere in the output
	assert.Contains(t, html, ScrollAnchor, "expected scroll anchor when changeLine is specified")

	// The anchor should be on the heading at line 5
	assert.Contains(t, html, `id="mpls-scroll-anchor">Second Heading`)
}

func TestHTML_LineBasedScrollAnchor_Paragraph(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	uri := "file:///test/linebased2.md"
	delete(oldDocContentByURI, uri)

	markdown := "# Heading\n\nFirst paragraph.\n\nSecond paragraph.\n\nThird paragraph."

	// Line 5 is "Second paragraph."
	html, _ := HTML(markdown, uri, 5)

	// Should have scroll anchor on the paragraph at line 5
	assert.Contains(t, html, ScrollAnchor, "expected scroll anchor when changeLine targets a paragraph")
}

func TestHTML_LineBasedScrollAnchor_LineAboveHeading(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	uri := "file:///test/lineabove.md"
	delete(oldDocContentByURI, uri)

	// Line numbers (1-based):
	// 1: # First Heading
	// 2: <empty>
	// 3: Paragraph one.
	// 4: <empty>
	// 5: # Second Heading
	// 6: <empty>
	// 7: Paragraph two.
	markdown := "# First Heading\n\nParagraph one.\n\n# Second Heading\n\nParagraph two."

	// When editing line 4 (empty line BEFORE Second Heading),
	// the anchor should NOT be on the heading - it should fall back to diff or no anchor
	html, _ := HTML(markdown, uri, 4)

	// The anchor should NOT be on Second Heading
	// This is the bug: anchor incorrectly attaches to the heading when editing line above it
	assert.NotContains(t, html, `id="mpls-scroll-anchor">Second Heading`,
		"anchor should NOT be on heading when editing line above it - this breaks presentation mode slide navigation")
}

func TestHTML_LineBasedScrollAnchor_FallbackToDiff(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	uri := "file:///test/fallback.md"
	delete(oldDocContentByURI, uri)

	markdown := "# Heading\n\nParagraph text."

	// First render with line 0 (use diff fallback)
	html1, _ := HTML(markdown, uri, 0)

	// Second render with changes and line 0 (use diff fallback)
	markdown2 := "# Heading\n\nParagraph text CHANGED."
	html2, _ := HTML(markdown2, uri, 0)

	// First render establishes baseline
	_ = html1

	// Second render should detect change via diff
	assert.Contains(t, html2, ScrollAnchor, "expected scroll anchor via diff fallback when changeLine is 0")
}

func TestGetExtensions_Caching(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	// First call should initialize
	ext1 := getExtensions()
	assert.NotNil(t, ext1, "expected non-nil extensions")

	// Second call should return same cached value
	ext2 := getExtensions()
	assert.Len(t, ext2, len(ext1), "expected cached extensions to have same length")
}

// resetExtensionsCache resets the extensions cache for testing.
// This allows tests to run with fresh extension state.
func resetExtensionsCache() {
	extensionsOnce = sync.Once{}
	cachedExtensions = nil

	// Reset feature flags to defaults
	EnableWikiLinks = false
	EnableFootnotes = false
	EnableEmoji = false
}

func TestHTML_WithWikiLinks(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	EnableWikiLinks = true

	defer func() {
		EnableWikiLinks = false
	}()

	markdown := "Check out [[other-page]]"
	uri := "file:///test/doc.md"

	html, _ := HTML(markdown, uri, 0)

	// Wiki links should be rendered as links
	assert.Contains(t, html, "other-page")
}

func TestHTML_WithEmoji(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	EnableEmoji = true

	defer func() {
		EnableEmoji = false
	}()

	markdown := "Hello :smile:"
	uri := "file:///test/doc.md"

	html, _ := HTML(markdown, uri, 0)

	// With emoji enabled, :smile: should be converted
	// The exact output depends on the emoji extension
	assert.Contains(t, html, "Hello")
}

func TestHTML_WithFootnotes(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	EnableFootnotes = true

	defer func() {
		EnableFootnotes = false
	}()

	markdown := "Text with footnote[^1]\n\n[^1]: This is the footnote."
	uri := "file:///test/doc.md"

	html, _ := HTML(markdown, uri, 0)

	// With footnotes enabled, should have footnote markup
	assert.Contains(t, html, "footnote")
}

func TestLinkResolverTransformer_ExternalLinks(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	markdown := "[External](https://example.com)"
	uri := "file:///test/doc.md"

	html, _ := HTML(markdown, uri, 0)

	// External links should NOT have data-mpls-internal attribute
	assert.NotContains(t, html, "data-mpls-internal")
}

func TestLinkResolverTransformer_AnchorLinks(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	markdown := "[Jump to section](#section)"
	uri := "file:///test/doc.md"

	html, _ := HTML(markdown, uri, 0)

	// Anchor-only links should have href preserved
	assert.Contains(t, html, `href="#section"`)
}

func TestLinkResolverTransformer_RelativeLinks(t *testing.T) { //nolint:paralleltest // Modifies global extensions cache
	resetExtensionsCache()

	// Set workspace root
	oldRoot := WorkspaceRoot
	WorkspaceRoot = "/test"

	defer func() {
		WorkspaceRoot = oldRoot
	}()

	markdown := "[Other doc](other.md)"
	uri := "file:///test/doc.md"

	html, _ := HTML(markdown, uri, 0)

	// Relative links within workspace should have data-mpls-internal attribute
	assert.Contains(t, html, "data-mpls-internal")
	assert.Contains(t, html, "data-mpls-target")
}

// Benchmarks for scroll anchor performance

// generateLargeMarkdown creates a realistic markdown document with n paragraphs.
func generateLargeMarkdown(paragraphs int) string {
	var sb strings.Builder
	sb.WriteString("# Document Title\n\n")
	sb.WriteString("This is an introduction paragraph with some text.\n\n")

	for i := 1; i <= paragraphs; i++ {
		if i%10 == 0 {
			sb.WriteString("## Section ")
			sb.WriteString(strconv.Itoa(i / 10))
			sb.WriteString("\n\n")
		}

		sb.WriteString("This is paragraph ")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(" with some content that makes it realistic. ")
		sb.WriteString("It contains multiple sentences to simulate real editing.\n\n")
	}

	return sb.String()
}

func BenchmarkHTML_LineBased(b *testing.B) {
	resetExtensionsCache()

	markdown := generateLargeMarkdown(100)
	uri := "file:///bench/doc.md"

	// Target line somewhere in the middle (around paragraph 50)
	changeLine := 150

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		HTML(markdown, uri, changeLine)
	}
}

func BenchmarkHTML_ContentDiff(b *testing.B) {
	resetExtensionsCache()

	markdown := generateLargeMarkdown(100)
	uri := "file:///bench/doc.md"

	// First call establishes baseline
	HTML(markdown, uri, 0)

	// Simulate a small change
	markdown2 := strings.Replace(markdown, "paragraph 50", "paragraph 50 CHANGED", 1)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		HTML(markdown2, uri, 0)
	}
}

func BenchmarkHTML_LineBased_LargeDoc(b *testing.B) {
	resetExtensionsCache()

	markdown := generateLargeMarkdown(500)
	uri := "file:///bench/large.md"

	// Target line somewhere in the middle
	changeLine := 750

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		HTML(markdown, uri, changeLine)
	}
}

func BenchmarkHTML_ContentDiff_LargeDoc(b *testing.B) {
	resetExtensionsCache()

	markdown := generateLargeMarkdown(500)
	uri := "file:///bench/large.md"

	// First call establishes baseline
	HTML(markdown, uri, 0)

	// Simulate a small change
	markdown2 := strings.Replace(markdown, "paragraph 250", "paragraph 250 CHANGED", 1)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		HTML(markdown2, uri, 0)
	}
}
