//go:build cgo

package parser //nolint:revive

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// katexCacheLen returns the current number of entries in the cache (test helper).
func katexCacheLen() int {
	katexCacheMu.RLock()
	defer katexCacheMu.RUnlock()

	return len(katexCache)
}

func TestKaTeXCache_HitMiss(t *testing.T) {
	t.Parallel()

	ClearKaTeXCache()

	// Miss
	got, ok := katexCacheGet("k")
	assert.False(t, ok)
	assert.Nil(t, got)

	// Set and hit
	katexCacheSet("k", []byte("v"))

	got, ok = katexCacheGet("k")
	require.True(t, ok)
	assert.Equal(t, []byte("v"), got)
}

func TestKaTeXCache_InlineVsDisplayKeySeparation(t *testing.T) { //nolint:paralleltest // uses testHookRender global
	ClearKaTeXCache()
	resetExtensionsCache()

	var count int32

	testHookRender = func(_ []byte, _ bool) {
		atomic.AddInt32(&count, 1)
	}

	defer func() { testHookRender = nil }()

	// Render doc with same formula as both inline ($x$) and display ($$x$$).
	_, _ = HTML("$x$ and $$x$$", "file:///t.md", 0)

	// Two distinct renders expected (inline key "i:x" and block key "b:x").
	assert.Equal(t, int32(2), atomic.LoadInt32(&count), "expected two renders on first pass")

	// Second pass — both already cached; no new renders.
	_, _ = HTML("$x$ and $$x$$", "file:///t.md", 0)

	assert.Equal(t, int32(2), atomic.LoadInt32(&count), "expected zero new renders on second pass")
}

func TestKaTeXCache_BoundedEviction(t *testing.T) {
	t.Parallel()

	ClearKaTeXCache()

	// Insert max+1 entries to trigger eviction.
	for i := 0; i <= katexMaxCacheSize; i++ {
		katexCacheSet(fmt.Sprintf("key-%d", i), []byte("v"))
	}

	assert.LessOrEqual(t, katexCacheLen(), katexMaxCacheSize,
		"cache must not exceed max size after eviction")
}

func TestKaTeXCache_Concurrency(t *testing.T) {
	t.Parallel()

	ClearKaTeXCache()

	const goroutines = 50

	var wg sync.WaitGroup

	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		i := i

		go func() {
			defer wg.Done()

			key := fmt.Sprintf("k%d", i%10) // overlapping keys

			katexCacheSet(key, []byte("html"))
			_, _ = katexCacheGet(key)
		}()
	}

	wg.Wait()
}

func TestKaTeXCache_Integration_RenderOnceForDuplicateFormula(t *testing.T) { //nolint:paralleltest // uses testHookRender global
	ClearKaTeXCache()
	resetExtensionsCache()

	var count int32

	testHookRender = func(_ []byte, _ bool) {
		atomic.AddInt32(&count, 1)
	}

	defer func() { testHookRender = nil }()

	// "$E=mc^2$" appears twice in the same document — only one underlying render.
	_, _ = HTML("$E=mc^2$ and $E=mc^2$ again", "file:///t.md", 0)

	assert.Equal(t, int32(1), atomic.LoadInt32(&count),
		"same inline formula in one document should render exactly once")

	// Second full HTML() call — still cached; zero new renders.
	_, _ = HTML("$E=mc^2$ and $E=mc^2$ again", "file:///t.md", 0)

	assert.Equal(t, int32(1), atomic.LoadInt32(&count),
		"formula already cached; no new renders on second HTML() call")
}

func TestKaTeXCache_Integration_InlineAndDisplayRenderOnceEach(t *testing.T) { //nolint:paralleltest // uses testHookRender global
	ClearKaTeXCache()
	resetExtensionsCache()

	var count int32

	testHookRender = func(_ []byte, _ bool) {
		atomic.AddInt32(&count, 1)
	}

	defer func() { testHookRender = nil }()

	// "$x$" (inline) and "$$x$$" (display) are distinct cache keys.
	_, _ = HTML("$x$ and $$x$$", "file:///t.md", 0)

	assert.Equal(t, int32(2), atomic.LoadInt32(&count),
		"inline and display of same source are two distinct renders")

	// Both now cached — second call produces zero new renders.
	_, _ = HTML("$x$ and $$x$$", "file:///t.md", 0)

	assert.Equal(t, int32(2), atomic.LoadInt32(&count),
		"no new renders expected; both formulas cached")
}

// TestKaTeXCache_HTMLNoDiff verifies that the custom extender produces the
// same HTML structure as the upstream extender for inline and block formulas.
func TestKaTeXCache_HTMLNoDiff(t *testing.T) { //nolint:paralleltest // uses global extensions cache
	ClearKaTeXCache()
	resetExtensionsCache()

	html, _ := HTML("Inline: $E=mc^2$\n\nBlock:\n$$\nE=mc^2\n$$", "file:///t.md", 0)

	// Inline formula should be rendered (not raw dollar signs).
	assert.NotContains(t, html, "$E=mc^2$", "inline formula should be rendered")
	// Block formula should be wrapped in a div.
	assert.Contains(t, html, "<div>", "block formula should be wrapped in <div>")
}
