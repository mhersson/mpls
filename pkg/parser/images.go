package parser //nolint:revive

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// cachedImage stores a base64-encoded image data URI along with its file modification time.
type cachedImage struct {
	dataURI string
	modTime time.Time
}

var (
	imageCache      = make(map[string]cachedImage)
	imageCacheMutex sync.RWMutex
	maxImageCache   = 50
)

// convertHTMLImages processes all <img> tags in HTML, converting local
// src paths to base64 data URIs. Preserves all attributes.
func convertHTMLImages(htmlContent, docDir string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(htmlContent))

	var result strings.Builder

	for {
		tt := tokenizer.Next()

		switch tt {
		case html.ErrorToken:
			// End of document or error
			return result.String()

		case html.SelfClosingTagToken, html.StartTagToken:
			token := tokenizer.Token()
			if token.Data == "img" {
				result.WriteString(processImgTag(token, docDir))
			} else {
				result.WriteString(token.String())
			}

		default:
			result.WriteString(string(tokenizer.Raw()))
		}
	}
}

// processImgTag processes an img tag, converting local src to base64 data URI.
func processImgTag(token html.Token, docDir string) string {
	var (
		srcIdx   = -1
		srcValue string
	)

	// Find the src attribute

	for i, attr := range token.Attr {
		if attr.Key == "src" {
			srcIdx = i
			srcValue = attr.Val

			break
		}
	}

	// No src attribute, return as-is
	if srcIdx == -1 {
		return token.String()
	}

	// Skip external URLs and data URIs
	if strings.HasPrefix(srcValue, "http://") ||
		strings.HasPrefix(srcValue, "https://") ||
		strings.HasPrefix(srcValue, "data:") {
		return token.String()
	}

	// Resolve the path relative to the document directory
	imagePath := srcValue
	if !filepath.IsAbs(imagePath) {
		imagePath = filepath.Join(docDir, srcValue)
	}

	imagePath = filepath.Clean(imagePath)

	// Convert to data URI
	dataURI, err := getImageDataURI(imagePath)
	if err != nil {
		// Log warning but leave src unchanged for browser to handle
		return token.String()
	}

	// Build the new img tag with data URI
	var result strings.Builder
	result.WriteString("<img")

	for i, attr := range token.Attr {
		result.WriteString(" ")

		if i == srcIdx {
			result.WriteString(`src="`)
			result.WriteString(dataURI)
			result.WriteString(`"`)
		} else {
			result.WriteString(attr.Key)
			result.WriteString(`="`)
			result.WriteString(html.EscapeString(attr.Val))
			result.WriteString(`"`)
		}
	}

	result.WriteString(">")

	return result.String()
}

// getImageDataURI returns a base64-encoded data URI for the given image file.
// Results are cached based on file path and modification time.
func getImageDataURI(imagePath string) (string, error) {
	// Check file info first
	info, err := os.Stat(imagePath)
	if err != nil {
		return "", fmt.Errorf("cannot stat image: %w", err)
	}

	// Check cache
	imageCacheMutex.RLock()

	if cached, ok := imageCache[imagePath]; ok {
		if info.ModTime().Equal(cached.modTime) {
			imageCacheMutex.RUnlock()

			return cached.dataURI, nil
		}
	}

	imageCacheMutex.RUnlock()

	// Cache miss or stale - read and encode
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("cannot read image: %w", err)
	}

	// Detect MIME type
	mimeType := http.DetectContentType(data)

	// Create data URI
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))

	// Update cache
	imageCacheMutex.Lock()

	if len(imageCache) >= maxImageCache {
		// Simple eviction: clear half the cache
		for k := range imageCache {
			delete(imageCache, k)

			if len(imageCache) < maxImageCache/2 {
				break
			}
		}
	}

	imageCache[imagePath] = cachedImage{dataURI: dataURI, modTime: info.ModTime()}
	imageCacheMutex.Unlock()

	return dataURI, nil
}

// ClearImageCache clears the image cache. Useful for testing.
func ClearImageCache() {
	imageCacheMutex.Lock()
	imageCache = make(map[string]cachedImage)
	imageCacheMutex.Unlock()
}
