package stdlib

import (
	"strings"
	"testing"
)

// TestRegexCacheCapEnforcedWhenAllRecent pins that the cache never exceeds
// its configured size: evictOldest used to refuse entries used within the
// last three seconds, so a burst of distinct patterns (all freshly inserted)
// grew the cache past the maximum without bound.
func TestRegexCacheCapEnforcedWhenAllRecent(t *testing.T) {
	saved := globalRegexCache.maxSize
	globalRegexCache.mu.Lock()
	globalRegexCache.maxSize = 4
	globalRegexCache.mu.Unlock()
	t.Cleanup(func() {
		globalRegexCache.mu.Lock()
		globalRegexCache.maxSize = saved
		globalRegexCache.mu.Unlock()
	})

	for i := 0; i < 50; i++ {
		pattern := "x" + strings.Repeat("a", i) + "y"
		if _, err := GetCompiledRegex(pattern); err != nil {
			t.Fatalf("compile %d: %v", i, err)
		}
	}

	globalRegexCache.mu.RLock()
	defer globalRegexCache.mu.RUnlock()
	if len(globalRegexCache.entries) > globalRegexCache.maxSize {
		t.Fatalf("cache holds %d entries with max %d", len(globalRegexCache.entries), globalRegexCache.maxSize)
	}
}
