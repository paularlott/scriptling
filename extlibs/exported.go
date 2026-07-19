// Package extlibs provides external libraries that need explicit registration
package extlibs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/paularlott/scriptling/extlibs/fssecurity"
)

// This file exports the grep/find/sed worker functions as a plain-Go API that
// does not require a Scriptling interpreter. Callers (e.g. the knot agent)
// invoke these directly against the local filesystem; the worker pool, glob
// filtering, binary-file skipping, atomic-rename in-place edit, and fssecurity
// path jailing are all reused verbatim from the scriptling.* library internals.
//
// A nil context is treated as context.Background(). All path arguments are
// resolved to absolute paths and checked against AllowedPaths (when set) using
// the same fssecurity logic as the interpreter-facing libraries.

// ErrPathNotAllowed is returned when a requested path falls outside the
// AllowedPaths configured on the options struct.
var ErrPathNotAllowed = errors.New("path is outside allowed directories")

// GrepOptions controls a Grep search.
//
// Literal selects literal-string matching (scriptling.grep.string) when true,
// versus regular-expression matching (scriptling.grep.pattern) when false.
//
// MaxSize skips files larger than this many bytes. The zero value applies the
// scriptling default of 1 MiB; a negative value disables the limit.
//
// AllowedPaths, when non-nil, restricts every searched path to the listed
// absolute directories (nil = no restriction, matching the interpreter default).
type GrepOptions struct {
	Literal      bool
	Recursive    bool
	IgnoreCase   bool
	FollowLinks  bool
	Glob         string
	MaxSize      int64
	AllowedPaths []string
}

// GrepMatch is a single matching line.
type GrepMatch struct {
	File string
	Line int
	Text string
}

// SedOptions controls a sed replace or extract operation. See GrepOptions for
// the meaning of MaxSize and AllowedPaths; the semantics are identical.
type SedOptions struct {
	Recursive    bool
	IgnoreCase   bool
	FollowLinks  bool
	Glob         string
	MaxSize      int64
	AllowedPaths []string
}

// ExtractMatch is a single regex match with its capture groups, returned by
// SedExtract.
type ExtractMatch struct {
	File   string
	Line   int
	Text   string
	Groups []string
}

// FindOptions controls a find search.
//
// Recursive is a pointer so that the zero value (nil) preserves scriptling's
// default of descending into subdirectories. Pass a pointer to false to keep
// the search non-recursive.
//
// Type selects "any" (the zero value), "file", or "dir".
//
// MtimeMin/MtimeMax and SizeMin/SizeMax are pointers so that a zero value is
// not confused with the valid bound 0; nil means the filter is inactive.
//
// MaxDepth of 0 means unlimited.
type FindOptions struct {
	Recursive     *bool
	Type          string
	Name          string
	MtimeMin      *float64
	MtimeMax      *float64
	SizeMin       *int64
	SizeMax       *int64
	IncludeHidden bool
	FollowLinks   bool
	MaxDepth      int
	AllowedPaths  []string
}

// FindEntry is a single matching entry returned by FindEntries, carrying the
// metadata required to decide whether the entry has changed without re-reading
// it. Callers comparing two trees (e.g. a sync tool diffing local and remote)
// can rely on Size+Mtime alone for the common case.
type FindEntry struct {
	Path  string
	Size  int64
	Mtime time.Time
	IsDir bool
}

// resolveMaxSize normalises a caller-supplied MaxSize to the internal convention
// (0 = unlimited). The caller's zero value means "use the 1 MiB default".
func resolveMaxSize(n int64) int64 {
	if n == 0 {
		return fileopsDefaultMaxFileSize
	}
	if n < 0 {
		return 0
	}
	return n
}

// toFileopsOptions builds the internal options struct from GrepOptions/SedOptions.
func toFileopsOptions(recursive, ignoreCase, followLinks bool, glob string, maxSize int64) fileopsOptions {
	return fileopsOptions{
		recursive:   recursive,
		ignoreCase:  ignoreCase,
		glob:        glob,
		maxSize:     resolveMaxSize(maxSize),
		followLinks: followLinks,
	}
}

// checkAllowed returns ErrPathNotAllowed when config restricts paths and the
// requested path is outside every allowed directory.
func checkAllowed(config fssecurity.Config, path string) error {
	if !config.IsPathAllowed(path) {
		return fmt.Errorf("%w: %s", ErrPathNotAllowed, path)
	}
	return nil
}

// Grep searches path for needle. When path is a directory the search runs
// concurrently over its files using the same bounded worker pool as
// scriptling.grep, respecting opts.Recursive / opts.Glob / opts.MaxSize.
//
// needle is interpreted as a regular expression unless opts.Literal is true.
// Matches are returned in arbitrary order.
func Grep(ctx context.Context, needle, path string, opts GrepOptions) ([]GrepMatch, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if needle == "" {
		return nil, errors.New("grep: pattern is empty")
	}
	config := fssecurity.Config{AllowedPaths: normalizeAllowed(opts.AllowedPaths)}
	if err := checkAllowed(config, path); err != nil {
		return nil, err
	}

	re, err := compilePattern(needle, opts.Literal, opts.IgnoreCase)
	if err != nil {
		return nil, fmt.Errorf("grep: invalid pattern: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("grep: cannot stat path: %w", err)
	}

	inst := &grepLibraryInstance{config: config}
	fopts := toFileopsOptions(opts.Recursive, opts.IgnoreCase, opts.FollowLinks, opts.Glob, opts.MaxSize)

	var matches []matchResult
	if info.IsDir() {
		matches = inst.searchDir(ctx, path, re, fopts)
	} else {
		matches = inst.searchFile(path, re, fopts)
	}

	out := make([]GrepMatch, len(matches))
	for i, m := range matches {
		out[i] = GrepMatch{File: m.File, Line: m.Line, Text: m.Text}
	}
	return out, nil
}

// toFindOptions translates the public FindOptions into the internal
// findOptions used by the find library. Recursive defaults to true (matching
// the scriptling default); opts.Recursive overrides it when non-nil. An error
// is returned when opts.Type is not one of "", "any", "file", or "dir".
func toFindOptions(opts FindOptions) (findOptions, error) {
	fopts := findOptions{
		recursive:     true, // scriptling default
		entryType:     "any",
		name:          opts.Name,
		includeHidden: opts.IncludeHidden,
		followLinks:   opts.FollowLinks,
		maxDepth:      opts.MaxDepth,
	}
	if opts.Recursive != nil {
		fopts.recursive = *opts.Recursive
	}
	switch opts.Type {
	case "", "any":
		fopts.entryType = "any"
	case "file", "dir":
		fopts.entryType = opts.Type
	default:
		return fopts, fmt.Errorf("find: type must be 'any', 'file', or 'dir', got %q", opts.Type)
	}
	if opts.MtimeMin != nil {
		fopts.mtimeMin = *opts.MtimeMin
		fopts.hasMtimeMin = true
	}
	if opts.MtimeMax != nil {
		fopts.mtimeMax = *opts.MtimeMax
		fopts.hasMtimeMax = true
	}
	if opts.SizeMin != nil {
		fopts.sizeMin = *opts.SizeMin
		fopts.hasSizeMin = true
	}
	if opts.SizeMax != nil {
		fopts.sizeMax = *opts.SizeMax
		fopts.hasSizeMax = true
	}
	return fopts, nil
}

// Find returns the paths under root that match the given filters. It uses the
// same concurrent walker as scriptling.find. Paths are returned in arbitrary
// order. The root itself is never included in the result.
func Find(ctx context.Context, root string, opts FindOptions) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	config := fssecurity.Config{AllowedPaths: normalizeAllowed(opts.AllowedPaths)}
	if err := checkAllowed(config, root); err != nil {
		return nil, err
	}

	fopts, err := toFindOptions(opts)
	if err != nil {
		return nil, err
	}

	inst := &findLibraryInstance{config: config}
	return inst.findPaths(ctx, root, fopts), nil
}

// FindEntries is like Find but returns FindEntry records with size, mtime, and
// type per match. Every matching entry is stat'd so the caller can compare
// trees without re-reading the bytes. Use Find instead when only the path
// strings are needed — Find skips the stat in the no-filter common case.
//
// Like Find, the root itself is never included in the result, and paths are
// returned in arbitrary order.
func FindEntries(ctx context.Context, root string, opts FindOptions) ([]FindEntry, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	config := fssecurity.Config{AllowedPaths: normalizeAllowed(opts.AllowedPaths)}
	if err := checkAllowed(config, root); err != nil {
		return nil, err
	}

	fopts, err := toFindOptions(opts)
	if err != nil {
		return nil, err
	}

	inst := &findLibraryInstance{config: config}
	return inst.findEntries(ctx, root, fopts), nil
}

// SedReplace replaces every occurrence of old with replacement in the file (or
// every matching file under the directory) at path. old is matched literally,
// not as a regular expression. Files are edited in place using an atomic
// temp-file + rename. The return value is the number of files modified.
func SedReplace(ctx context.Context, old, replacement, path string, opts SedOptions) (int64, error) {
	return sedRun(ctx, old, replacement, path, opts, true)
}

// SedReplacePattern is SedReplace with old interpreted as a regular expression.
// Capture groups may be referenced in replacement as ${1}, ${2}, or ${name}.
func SedReplacePattern(ctx context.Context, pattern, replacement, path string, opts SedOptions) (int64, error) {
	return sedRun(ctx, pattern, replacement, path, opts, false)
}

func sedRun(ctx context.Context, needle, replacement, path string, opts SedOptions, literal bool) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if needle == "" {
		return 0, errors.New("sed: pattern is empty")
	}
	config := fssecurity.Config{AllowedPaths: normalizeAllowed(opts.AllowedPaths)}
	if err := checkAllowed(config, path); err != nil {
		return 0, err
	}

	re, err := compilePattern(needle, literal, opts.IgnoreCase)
	if err != nil {
		return 0, fmt.Errorf("sed: invalid pattern: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("sed: cannot stat path: %w", err)
	}

	inst := &sedLibraryInstance{config: config}
	fopts := toFileopsOptions(opts.Recursive, opts.IgnoreCase, opts.FollowLinks, opts.Glob, opts.MaxSize)

	if info.IsDir() {
		return inst.replaceDir(ctx, path, re, replacement, fopts), nil
	}
	if inst.replaceFile(path, re, replacement, fopts) {
		return 1, nil
	}
	return 0, nil
}

// SedExtract returns every match of pattern (a regular expression with capture
// groups) found in the file or directory at path. The result includes the
// captured groups for each match.
func SedExtract(ctx context.Context, pattern, path string, opts SedOptions) ([]ExtractMatch, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if pattern == "" {
		return nil, errors.New("sed: pattern is empty")
	}
	config := fssecurity.Config{AllowedPaths: normalizeAllowed(opts.AllowedPaths)}
	if err := checkAllowed(config, path); err != nil {
		return nil, err
	}

	// extract always uses regex semantics (literal=false) — see fnExtract.
	re, err := compilePattern(pattern, false, opts.IgnoreCase)
	if err != nil {
		return nil, fmt.Errorf("sed: invalid pattern: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("sed: cannot stat path: %w", err)
	}

	inst := &sedLibraryInstance{config: config}
	fopts := toFileopsOptions(opts.Recursive, opts.IgnoreCase, opts.FollowLinks, opts.Glob, opts.MaxSize)

	var results []extractResult
	if info.IsDir() {
		results = inst.extractDir(ctx, path, re, fopts)
	} else {
		results = inst.extractFile(path, re, fopts)
	}

	out := make([]ExtractMatch, len(results))
	for i, r := range results {
		groups := make([]string, len(r.Groups))
		copy(groups, r.Groups)
		out[i] = ExtractMatch{File: r.File, Line: r.Line, Text: r.Text, Groups: groups}
	}
	return out, nil
}

// ErrSearchNotFound is returned by EditFile when the search text does not
// appear in the file at all.
var ErrSearchNotFound = errors.New("search text not found")

// ErrSearchNotUnique is returned by EditFile when the search text appears more
// than once. The caller should provide more surrounding context to disambiguate.
var ErrSearchNotUnique = errors.New("search text matched multiple times")

// EditFile performs a targeted search-and-replace on a single file: it finds
// the exact `search` text, verifies it appears exactly once, and replaces it
// with `replace`. The modification is written atomically (temp file + rename),
// matching sed's in-place edit semantics.
//
// Unlike SedReplace (which replaces every occurrence line-by-line), EditFile
// operates on the full file content and requires the match to be unique — the
// gold standard for coding-agent edits where "replace all" is dangerous.
//
// search and replace may span multiple lines. Returns the number of bytes
// written.
func EditFile(ctx context.Context, path, search, replace string) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if search == "" {
		return 0, errors.New("edit: search text is empty")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("edit: cannot read file: %w", err)
	}

	count := strings.Count(string(content), search)
	if count == 0 {
		return 0, fmt.Errorf("%w in %s", ErrSearchNotFound, path)
	}
	if count > 1 {
		return 0, fmt.Errorf("%w (%d occurrences) in %s", ErrSearchNotUnique, count, path)
	}

	newContent := strings.Replace(string(content), search, replace, 1)

	// Atomic write: temp file in the same directory, then rename.
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".scriptling-edit-*")
	if err != nil {
		return 0, fmt.Errorf("edit: cannot create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.WriteString(newContent); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return 0, fmt.Errorf("edit: cannot write temp file: %w", err)
	}

	// Preserve permissions from the original file.
	if info, err := os.Stat(path); err == nil {
		tmp.Chmod(info.Mode())
	}
	tmp.Close()

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return 0, fmt.Errorf("edit: cannot rename temp file: %w", err)
	}

	return len(newContent), nil
}

// normalizeAllowed mirrors the path absolutification that NewGrepLibrary etc.
// apply to their AllowedPaths, so the exported API enforces identical
// resolution semantics whether called from Go or from the interpreter.
func normalizeAllowed(paths []string) []string {
	if paths == nil {
		return nil
	}
	normalized := make([]string, 0, len(paths))
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		normalized = append(normalized, filepath.Clean(abs))
	}
	return normalized
}
