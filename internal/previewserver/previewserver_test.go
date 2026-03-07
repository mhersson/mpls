package previewserver

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetChromaStyleForTheme(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		themeName string
		expected  string
	}{
		// Special cases that map to different chroma styles
		{
			name:      "ayu-dark maps to github-dark",
			themeName: "ayu-dark",
			expected:  "github-dark",
		},
		{
			name:      "ayu-light maps to github",
			themeName: "ayu-light",
			expected:  "github",
		},
		{
			name:      "dark maps to catppuccin-mocha",
			themeName: "dark",
			expected:  "catppuccin-mocha",
		},
		{
			name:      "light maps to catppuccin-mocha",
			themeName: "light",
			expected:  "catppuccin-mocha",
		},
		{
			name:      "everforest-dark maps to evergarden",
			themeName: "everforest-dark",
			expected:  "evergarden",
		},
		{
			name:      "gruvbox-dark maps to gruvbox",
			themeName: "gruvbox-dark",
			expected:  "gruvbox",
		},
		{
			name:      "tokyonight maps to tokyonight-night",
			themeName: "tokyonight",
			expected:  "tokyonight-night",
		},
		// Direct mappings (theme name = chroma style)
		{
			name:      "catppuccin-mocha returns itself",
			themeName: "catppuccin-mocha",
			expected:  "catppuccin-mocha",
		},
		{
			name:      "catppuccin-frappe returns itself",
			themeName: "catppuccin-frappe",
			expected:  "catppuccin-frappe",
		},
		{
			name:      "nord returns itself",
			themeName: "nord",
			expected:  "nord",
		},
		{
			name:      "dracula returns itself",
			themeName: "dracula",
			expected:  "dracula",
		},
		{
			name:      "rose-pine returns itself",
			themeName: "rose-pine",
			expected:  "rose-pine",
		},
		// Unknown themes return themselves
		{
			name:      "unknown theme returns itself",
			themeName: "my-custom-theme",
			expected:  "my-custom-theme",
		},
		{
			name:      "empty theme returns empty",
			themeName: "",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := GetChromaStyleForTheme(tt.themeName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetThemeConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		themeName           string
		expectedCSSFile     string
		expectedMermaidDark bool
	}{
		// Dark themes with "dark" in name
		{
			name:                "github-dark is dark",
			themeName:           "github-dark",
			expectedCSSFile:     "themes/github-dark.css",
			expectedMermaidDark: true,
		},
		{
			name:                "ayu-dark is dark",
			themeName:           "ayu-dark",
			expectedCSSFile:     "themes/ayu-dark.css",
			expectedMermaidDark: true,
		},
		// Dark themes without "dark" in name
		{
			name:                "catppuccin-mocha is dark",
			themeName:           "catppuccin-mocha",
			expectedCSSFile:     "themes/catppuccin-mocha.css",
			expectedMermaidDark: true,
		},
		{
			name:                "catppuccin-frappe is dark",
			themeName:           "catppuccin-frappe",
			expectedCSSFile:     "themes/catppuccin-frappe.css",
			expectedMermaidDark: true,
		},
		{
			name:                "catppuccin-macchiato is dark",
			themeName:           "catppuccin-macchiato",
			expectedCSSFile:     "themes/catppuccin-macchiato.css",
			expectedMermaidDark: true,
		},
		{
			name:                "dracula is dark",
			themeName:           "dracula",
			expectedCSSFile:     "themes/dracula.css",
			expectedMermaidDark: true,
		},
		{
			name:                "nord is dark",
			themeName:           "nord",
			expectedCSSFile:     "themes/nord.css",
			expectedMermaidDark: true,
		},
		{
			name:                "rose-pine is dark",
			themeName:           "rose-pine",
			expectedCSSFile:     "themes/rose-pine.css",
			expectedMermaidDark: true,
		},
		{
			name:                "tokyonight is dark",
			themeName:           "tokyonight",
			expectedCSSFile:     "themes/tokyonight.css",
			expectedMermaidDark: true,
		},
		{
			name:                "tokyonight-storm is dark",
			themeName:           "tokyonight-storm",
			expectedCSSFile:     "themes/tokyonight-storm.css",
			expectedMermaidDark: true,
		},
		{
			name:                "tokyonight-moon is dark",
			themeName:           "tokyonight-moon",
			expectedCSSFile:     "themes/tokyonight-moon.css",
			expectedMermaidDark: true,
		},
		// Light themes
		{
			name:                "light is light",
			themeName:           "light",
			expectedCSSFile:     "themes/light.css",
			expectedMermaidDark: false,
		},
		{
			name:                "ayu-light is light",
			themeName:           "ayu-light",
			expectedCSSFile:     "themes/ayu-light.css",
			expectedMermaidDark: false,
		},
		{
			name:                "github is light",
			themeName:           "github",
			expectedCSSFile:     "themes/github.css",
			expectedMermaidDark: false,
		},
		// Custom/unknown themes default to light
		{
			name:                "unknown theme defaults to light",
			themeName:           "my-custom-theme",
			expectedCSSFile:     "themes/my-custom-theme.css",
			expectedMermaidDark: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cssFile, mermaidTheme := getThemeConfig(tt.themeName)
			assert.Equal(t, tt.expectedCSSFile, cssFile)

			if tt.expectedMermaidDark {
				assert.Equal(t, "dark", mermaidTheme)
			} else {
				assert.Equal(t, "default", mermaidTheme)
			}
		})
	}
}

func TestIsValidMarkdownExt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ext      string
		expected bool
	}{
		// Valid extensions
		{name: ".md is valid", ext: ".md", expected: true},
		{name: ".markdown is valid", ext: ".markdown", expected: true},
		{name: ".mkd is valid", ext: ".mkd", expected: true},
		{name: ".mkdn is valid", ext: ".mkdn", expected: true},
		{name: ".mdwn is valid", ext: ".mdwn", expected: true},
		// Invalid extensions
		{name: ".txt is invalid", ext: ".txt", expected: false},
		{name: ".html is invalid", ext: ".html", expected: false},
		{name: ".go is invalid", ext: ".go", expected: false},
		{name: ".MD uppercase is invalid", ext: ".MD", expected: false},
		{name: "empty is invalid", ext: "", expected: false},
		{name: "md without dot is invalid", ext: "md", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := isValidMarkdownExt(tt.ext)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsStaticAsset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		// Static asset paths
		{name: "styles.css", path: "/styles.css", expected: true},
		{name: "katex.min.css", path: "/katex.min.css", expected: true},
		{name: "mermaid.min.js", path: "/mermaid.min.js", expected: true},
		{name: "ws.js", path: "/ws.js", expected: true},
		{name: "ws websocket endpoint", path: "/ws", expected: true},
		{name: "presentation.js", path: "/presentation.js", expected: true},
		{name: "presentation.css", path: "/presentation.css", expected: true},
		// Font paths
		{name: "font file", path: "/fonts/KaTeX_Main-Regular.woff2", expected: true},
		{name: "fonts root", path: "/fonts/", expected: true},
		// Theme paths
		{name: "theme file", path: "/themes/dark.css", expected: true},
		{name: "themes root", path: "/themes/", expected: true},
		// Non-static paths
		{name: "root path", path: "/", expected: false},
		{name: "markdown file", path: "/docs/readme.md", expected: false},
		{name: "random path", path: "/some/path", expected: false},
		{name: "empty path", path: "", expected: false},
		{name: "partial match styles", path: "/styles.css.bak", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := isStaticAsset(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertMetaToHTMLTable(t *testing.T) {
	t.Parallel()

	t.Run("empty map returns empty string", func(t *testing.T) {
		t.Parallel()

		result := convertMetaToHTMLTable(map[string]any{})
		assert.Empty(t, result)
	})

	t.Run("nil map returns empty string", func(t *testing.T) {
		t.Parallel()

		result := convertMetaToHTMLTable(nil)
		assert.Empty(t, result)
	})

	t.Run("single key-value pair", func(t *testing.T) {
		t.Parallel()

		meta := map[string]any{"title": "Test"}
		result := convertMetaToHTMLTable(meta)

		assert.Contains(t, result, "<table>")
		assert.Contains(t, result, "</table>")
		assert.Contains(t, result, "<th colspan='2'>Meta</th>")
		assert.Contains(t, result, "<td>title</td>")
		assert.Contains(t, result, "<td>Test</td>")
	})

	t.Run("multiple key-value pairs", func(t *testing.T) {
		t.Parallel()

		meta := map[string]any{
			"title":  "Test",
			"author": "John",
		}
		result := convertMetaToHTMLTable(meta)

		assert.Contains(t, result, "<td>title</td>")
		assert.Contains(t, result, "<td>author</td>")
	})

	t.Run("HTML escapes keys and values", func(t *testing.T) {
		t.Parallel()

		meta := map[string]any{
			"<script>": "<b>bold</b>",
		}
		result := convertMetaToHTMLTable(meta)

		// Should be escaped
		assert.Contains(t, result, "&lt;script&gt;")
		assert.Contains(t, result, "&lt;b&gt;bold&lt;/b&gt;")
		// Should NOT contain raw HTML
		assert.NotContains(t, result, "<script>")
		assert.NotContains(t, result, "<b>bold</b>")
	})

	t.Run("handles non-string values", func(t *testing.T) {
		t.Parallel()

		meta := map[string]any{
			"count":   42,
			"enabled": true,
			"pi":      3.14,
		}
		result := convertMetaToHTMLTable(meta)

		assert.Contains(t, result, "42")
		assert.Contains(t, result, "true")
		assert.Contains(t, result, "3.14")
	})
}

func TestConvertMetaToHTMLTable_SortedKeys(t *testing.T) {
	t.Parallel()

	meta := map[string]any{
		"zebra":    "z",
		"alpha":    "a",
		"middle":   "m",
		"beta":     "b",
		"yak":      "y",
		"cardinal": "c",
	}

	result := convertMetaToHTMLTable(meta)

	// Keys should be in alphabetical order
	alphaIdx := strings.Index(result, "alpha")
	betaIdx := strings.Index(result, "beta")
	cardinalIdx := strings.Index(result, "cardinal")
	middleIdx := strings.Index(result, "middle")
	yakIdx := strings.Index(result, "yak")
	zebraIdx := strings.Index(result, "zebra")

	assert.Less(t, alphaIdx, betaIdx, "alpha should come before beta")
	assert.Less(t, betaIdx, cardinalIdx, "beta should come before cardinal")
	assert.Less(t, cardinalIdx, middleIdx, "cardinal should come before middle")
	assert.Less(t, middleIdx, yakIdx, "middle should come before yak")
	assert.Less(t, yakIdx, zebraIdx, "yak should come before zebra")
}
