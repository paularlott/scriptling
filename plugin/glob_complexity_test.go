package plugin

import (
	"path"
	"strings"
	"testing"
	"time"
)

// MatchGlob is exported for fetcher authors and used by the reference
// GlobDisk. Its `**` handling recurses on every segment split with no
// memoization or bound, so a pattern stacking several `**` segments against a
// deep non-matching path is exponential. Reproduced against the real matcher:
// ~4 `**` is microseconds, but `**`-separated-by-`*` reaches ~1s at 10 and
// ~3.5s at 12, and consecutive `**/**/.../**/<miss>` fails to return within
// 10s past ~12 segments.
//
// Reachability today is narrow: the host only ever builds "*", "<dir>/*", or
// wildcard-free patterns (see pluginFS), so it never generates the adversarial
// shape, and an external fetcher absorbs the cost in its own process. But
// MatchGlob is a public helper the docs point plugin authors at, and any
// in-process fetcher (or host code) that lets a caller-influenced pattern
// reach it turns this into a CPU-bound hang on the host goroutine that no
// fetch timeout interrupts (the matcher never checks context).
//
// This test bounds a single MatchGlob call. It FAILS on the current recursive
// matcher and PASSES once matching is linear (two-pointer / DP) or the `**`
// count / recursion depth is bounded.
func TestMatchGlobDoesNotBlowUp(t *testing.T) {
	// `**` separated by `*`, trailing literal that never matches: the shape
	// that forces the full search tree in the current matcher.
	var parts []string
	for i := 0; i < 12; i++ {
		parts = append(parts, "**", "*")
	}
	parts = append(parts, "zzz")
	pattern := strings.Join(parts, "/")
	name := strings.Repeat("a/", 30) + "b"

	done := make(chan bool, 1)
	go func() { done <- MatchGlob(pattern, name) }()

	select {
	case got := <-done:
		if got {
			t.Fatalf("pattern unexpectedly matched %q", name)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("MatchGlob did not return within 2s on a stacked-** pattern: " +
			"exponential backtracking. MatchGlob is a public helper for fetcher " +
			"authors; make matching linear or bound the ** count / recursion depth.")
	}
}

// TestMatchGlobStackedStarStarStillCorrect guards that whatever bound/rewrite
// fixes the blowup preserves the semantics: stacked ** collapses to "any
// number of segments", so a reachable deep path still matches and a
// non-matching final segment still fails.
func TestMatchGlobStackedStarStarStillCorrect(t *testing.T) {
	if !MatchGlob("**/**/x.py", "a/b/c/x.py") {
		t.Fatal("stacked ** should match a reachable deep path")
	}
	if MatchGlob("**/**/x.py", "a/b/c/y.py") {
		t.Fatal("stacked ** should not match when the final segment differs")
	}
}

// referenceMatch is the obviously-correct exponential matcher the linear one
// replaced: try every split at each "**". It is the oracle for the
// differential test below; keep it simple, never fast.
func referenceMatch(pattern, name []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == "**" {
			if len(pattern) == 1 {
				return true
			}
			for i := 0; i <= len(name); i++ {
				if referenceMatch(pattern[1:], name[i:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		ok, err := path.Match(pattern[0], name[0])
		if err != nil || !ok {
			return false
		}
		pattern, name = pattern[1:], name[1:]
	}
	return len(name) == 0
}

// TestMatchGlobMatchesReference differentially tests the linear matcher
// against the exhaustive reference on a generated cross-product of small
// patterns and paths: the single-backtrack-point rewrite must be exactly as
// correct as the try-every-split original.
func TestMatchGlobMatchesReference(t *testing.T) {
	segments := []string{"a", "b", "*", "**", "*.py", "a*"}
	var patterns, names [][]string
	var build func(prefix []string, depth int, into *[][]string)
	build = func(prefix []string, depth int, into *[][]string) {
		*into = append(*into, prefix)
		if depth == 3 {
			return
		}
		for _, segment := range segments {
			build(append(append([]string{}, prefix...), segment), depth+1, into)
		}
	}
	build(nil, 0, &patterns)
	build(nil, 0, &names)

	for _, pattern := range patterns {
		for _, name := range names {
			want := referenceMatch(pattern, name)
			got := matchSegments(pattern, name)
			if got != want {
				t.Fatalf("matchSegments(%v, %v) = %v, reference says %v", pattern, name, got, want)
			}
		}
	}
}
