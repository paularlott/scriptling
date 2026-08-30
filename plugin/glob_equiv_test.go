package plugin

import (
	"path"
	"strings"
	"testing"
)

// oracleMatch is a deliberately naive, obviously-correct reference for the
// fetch glob language: it recurses on every "**" split. It is exponential and
// unfit for production (that is the whole reason matchSegments was rewritten),
// but over small inputs it is the ground truth to check the fast matcher
// against — the two-pointer rewrite must agree with it on every case.
func oracleMatch(pattern, name string) bool {
	return oracleSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func oracleSegments(pattern, name []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == "**" {
			if len(pattern) == 1 {
				return true
			}
			for i := 0; i <= len(name); i++ {
				if oracleSegments(pattern[1:], name[i:]) {
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

// TestMatchGlobEquivalentToOracle exhaustively compares the fast two-pointer
// matcher against the naive recursive oracle over generated patterns and
// names, with emphasis on the multi-** cases where a single-backtrack-point
// algorithm is most likely to diverge from the true semantics.
func TestMatchGlobEquivalentToOracle(t *testing.T) {
	segAlphabet := []string{"a", "b", "*", "?", "**", "[ab]", "a*", "*b"}
	nameAlphabet := []string{"a", "b", "ab", "abc"}

	// Build all patterns and names up to a small segment count.
	var patterns, names []string
	var gen func(alpha []string, maxLen int, prefix []string, out *[]string)
	gen = func(alpha []string, maxLen int, prefix []string, out *[]string) {
		if len(prefix) > 0 {
			*out = append(*out, strings.Join(prefix, "/"))
		}
		if len(prefix) == maxLen {
			return
		}
		for _, s := range alpha {
			gen(alpha, maxLen, append(prefix, s), out)
		}
	}
	gen(segAlphabet, 3, nil, &patterns)  // patterns up to 3 segments
	gen(nameAlphabet, 3, nil, &names)    // names up to 3 segments

	mismatches := 0
	for _, p := range patterns {
		for _, n := range names {
			got := matchSegments(strings.Split(p, "/"), strings.Split(n, "/"))
			want := oracleSegments(strings.Split(p, "/"), strings.Split(n, "/"))
			if got != want {
				mismatches++
				if mismatches <= 20 {
					t.Errorf("MatchGlob(%q, %q) = %v, oracle = %v", p, n, got, want)
				}
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d pattern/name pairs disagree with the reference matcher "+
			"(the two-pointer rewrite is not equivalent to the glob semantics)", mismatches)
	}
	t.Logf("checked %d patterns x %d names = %d pairs, all agree",
		len(patterns), len(names), len(patterns)*len(names))
}
