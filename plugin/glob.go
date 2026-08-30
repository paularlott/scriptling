package plugin

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// The fetch protocol's glob language, shared by the host, plugin authors and
// the helpers below:
//
//   - patterns are slash-separated paths relative to the source root
//   - "*" matches any run of characters within one segment (never "/")
//   - "?" matches one character within a segment
//   - "[class]" is a character class, as in path.Match
//   - "**" as a whole segment matches any number of segments, including none
//   - a pattern with no wildcards is legal: it matches at most that one path,
//     which is how existence and directory-ness are probed (a matched
//     directory carries is_dir=true, so an empty directory is distinguishable
//     from a missing one)

// MatchGlob reports whether name, a slash path relative to a source root,
// matches pattern in the fetch glob language. Fetcher implementations can
// serve Glob by matching their known paths with this helper instead of
// reimplementing the semantics.
func MatchGlob(pattern, name string) bool {
	return matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

// matchSegments matches segment-wise, greedily, backtracking only through
// "**": the two-pointer wildcard algorithm. When a segment fails to match,
// the most recent "**" consumes one more name segment and matching resumes
// after it; that single backtrack point is the classic glob result, and it
// keeps matching linear in the number of segments instead of exponential in
// the number of "**"s (a stacked-** pattern against a deep non-matching path
// is adversarial input a public helper must not choke on).
func matchSegments(pattern, name []string) bool {
	pi, ni := 0, 0
	starP, starN := -1, -1 // position after the last "**", and how far it has consumed
	for ni < len(name) {
		if pi < len(pattern) && pattern[pi] == "**" {
			// Greedy: let this "**" match zero segments first; the
			// backtrack below grows it if a later segment fails.
			starP, starN = pi+1, ni
			pi++
			continue
		}
		if pi < len(pattern) {
			if ok, err := path.Match(pattern[pi], name[ni]); err == nil && ok {
				pi++
				ni++
				continue
			}
		}
		if starP >= 0 {
			// No match at this segment: the last "**" swallows one more
			// name segment and we retry from just after it.
			starN++
			pi, ni = starP, starN
			continue
		}
		return false
	}
	// Name exhausted: the rest of the pattern must be "**"s (each matching
	// the zero segments that remain).
	for pi < len(pattern) && pattern[pi] == "**" {
		pi++
	}
	return pi == len(pattern)
}

// literalPrefix returns the leading literal directory segments of pattern:
// the part before the first segment carrying a wildcard or "**". It bounds
// disk walks, which can start at that subdirectory without missing matches.
func literalPrefix(pattern string) []string {
	var prefix []string
	for _, segment := range strings.Split(pattern, "/") {
		if segment == "**" || strings.ContainsAny(segment, "*?[") {
			break
		}
		prefix = append(prefix, segment)
	}
	return prefix
}

// GlobDisk is the reference Glob for a fetcher serving files from a
// directory: it walks root and returns every entry whose path relative to
// root matches pattern (directories included, with is_dir set).
//
// Symlink defense comes built in: each symlink encountered is resolved to its
// real path and must stay inside root, so a link planted in the served tree
// cannot serve files from outside it. Directory symlinks are not followed
// (the walk would leave root); file symlinks that resolve inside root are
// served as files. Fetchers with richer backends implement Glob themselves
// but owe their users the same containment guarantee.
func GlobDisk(root, pattern string) ([]FetchEntry, error) {
	start := root
	for _, segment := range literalPrefix(pattern) {
		start = filepath.Join(start, segment)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}

	entries := []FetchEntry{}
	err = filepath.WalkDir(start, func(current string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// A pruned-start directory that does not exist simply matches
			// nothing; anything else is the caller's filesystem talking.
			if current == start {
				return fs.SkipAll
			}
			return walkErr
		}
		rel, relErr := filepath.Rel(root, current)
		if relErr != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		isDir := d.IsDir()
		if d.Type()&fs.ModeSymlink != 0 {
			real, linkErr := filepath.EvalSymlinks(current)
			if linkErr != nil {
				return nil // a dangling link matches nothing
			}
			if !withinRoot(realRoot, real) {
				return nil // a link out of the tree is not served
			}
			info, statErr := os.Stat(real)
			if statErr != nil {
				return nil
			}
			if info.IsDir() {
				return nil // directory symlinks are not followed
			}
			isDir = false
		}
		if MatchGlob(pattern, rel) {
			entries = append(entries, FetchEntry{Name: rel, IsDir: isDir})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// withinRoot reports whether target is root itself or lies underneath it.
func withinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
