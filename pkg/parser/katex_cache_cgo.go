//go:build cgo

package parser //nolint:revive

import "sync"

const katexMaxCacheSize = 256

// katexCache is the process-level KaTeX render cache.
// Keys use a type prefix: "i:" for inline, "b:" for block.
var (
	katexCache   = make(map[string][]byte)
	katexCacheMu sync.RWMutex
)

func katexCacheGet(key string) ([]byte, bool) {
	katexCacheMu.RLock()
	defer katexCacheMu.RUnlock()

	v, ok := katexCache[key]

	return v, ok
}

func katexCacheSet(key string, html []byte) {
	katexCacheMu.Lock()
	defer katexCacheMu.Unlock()

	if len(katexCache) >= katexMaxCacheSize {
		// Clear half on overflow, mirroring the PlantUML eviction strategy.
		for k := range katexCache {
			delete(katexCache, k)

			if len(katexCache) < katexMaxCacheSize/2 {
				break
			}
		}
	}

	katexCache[key] = html
}

// ClearKaTeXCache empties the render cache. Useful for tests.
func ClearKaTeXCache() {
	katexCacheMu.Lock()
	katexCache = make(map[string][]byte)
	katexCacheMu.Unlock()
}
