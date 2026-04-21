// Package extlibs provides external libraries that need explicit registration
package extlibs

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

// grepWorkerPoolSize is the minimum number of concurrent file-search goroutines.
// The actual pool size is max(grepWorkerPoolSize, NumCPU/2).
const grepWorkerPoolSize = 4

// grepDefaultMaxFileSize is the default maximum file size to search (1 MiB, matching rg).
// Pass max_size=None to disable the limit.
const grepDefaultMaxFileSize int64 = 1 * 1024 * 1024

// grepBinarySniffLen is the number of bytes read to detect binary files.
const grepBinarySniffLen = 8000

// grepLibraryInstance holds the configured grep library instance.
type grepLibraryInstance struct {
	config fssecurity.Config
}

// RegisterGrepLibrary registers the scriptling.grep library with a Scriptling instance.
// If allowedPaths is nil, all paths are allowed. If non-nil, operations are restricted
// to those directories (same semantics as RegisterOSLibrary).
func RegisterGrepLibrary(registrar object.LibraryRegistrar, allowedPaths []string) {
	config := fssecurity.Config{AllowedPaths: allowedPaths}
	registrar.RegisterLibrary(NewGrepLibrary(config))
}

// NewGrepLibrary creates a new scriptling.grep library with the given configuration.
func NewGrepLibrary(config fssecurity.Config) *object.Library {
	if config.AllowedPaths != nil {
		normalized := make([]string, 0, len(config.AllowedPaths))
		for _, p := range config.AllowedPaths {
			abs, err := filepath.Abs(p)
			if err != nil {
				continue
			}
			normalized = append(normalized, filepath.Clean(abs))
		}
		config.AllowedPaths = normalized
	}
	inst := &grepLibraryInstance{config: config}
	return inst.createLibrary()
}

func (g *grepLibraryInstance) createLibrary() *object.Library {
	return object.NewLibrary(GrepLibraryName, map[string]*object.Builtin{
		"pattern": {
			Fn:       g.fnPattern,
			HelpText: grepPatternHelp,
		},
		"string": {
			Fn:       g.fnString,
			HelpText: grepStringHelp,
		},
	}, nil, "Fast file content search with regex or literal patterns")
}

// workerCount returns the bounded worker pool size.
func workerCount() int {
	n := runtime.NumCPU() / 2
	if n < grepWorkerPoolSize {
		n = grepWorkerPoolSize
	}
	return n
}

// grepOptions holds parsed kwargs for a search call.
type grepOptions struct {
	recursive   bool
	ignoreCase  bool
	glob        string
	maxSize     int64
	followLinks bool
}

func parseGrepKwargs(kwargs object.Kwargs) (grepOptions, object.Object) {
	opts := grepOptions{
		maxSize: grepDefaultMaxFileSize,
	}

	if v := kwargs.Get("recursive"); v != nil {
		b, err := v.AsBool()
		if err != nil {
			return opts, err
		}
		opts.recursive = b
	}
	if v := kwargs.Get("ignore_case"); v != nil {
		b, err := v.AsBool()
		if err != nil {
			return opts, err
		}
		opts.ignoreCase = b
	}
	if v := kwargs.Get("glob"); v != nil {
		s, err := v.AsString()
		if err != nil {
			return opts, err
		}
		opts.glob = s
	}
	if v := kwargs.Get("follow_links"); v != nil {
		b, err := v.AsBool()
		if err != nil {
			return opts, err
		}
		opts.followLinks = b
	}
	if v := kwargs.Get("max_size"); v != nil {
		if _, isNull := v.(*object.Null); isNull {
			opts.maxSize = 0
		} else {
			n, err := v.AsInt()
			if err != nil {
				return opts, err
			}
			opts.maxSize = n
		}
	}

	return opts, nil
}

// compilePattern compiles the search pattern into a *regexp.Regexp.
func compilePattern(pattern string, literal bool, ignoreCase bool) (*regexp.Regexp, error) {
	p := pattern
	if literal {
		p = regexp.QuoteMeta(p)
	}
	if ignoreCase {
		p = "(?i)" + p
	}
	return regexp.Compile(p)
}

// matchResult is a single line match.
type matchResult struct {
	File string
	Line int
	Text string
}

// fnPattern implements grep.pattern(pattern, path, **kwargs) — regex search
func (g *grepLibraryInstance) fnPattern(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	return g.run(ctx, kwargs, args, false)
}

// fnString implements grep.string(text, path, **kwargs) — literal string search
func (g *grepLibraryInstance) fnString(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	return g.run(ctx, kwargs, args, true)
}

// run is the shared implementation for pattern and string.
func (g *grepLibraryInstance) run(ctx context.Context, kwargs object.Kwargs, args []object.Object, literal bool) object.Object {
	if err := errors.ExactArgs(args, 2); err != nil {
		return err
	}
	needle, err := args[0].AsString()
	if err != nil {
		return err
	}
	searchPath, err := args[1].AsString()
	if err != nil {
		return err
	}

	if secErr := g.checkPath(searchPath); secErr != nil {
		return secErr
	}

	opts, oErr := parseGrepKwargs(kwargs)
	if oErr != nil {
		return oErr
	}

	re, reErr := compilePattern(needle, literal, opts.ignoreCase)
	if reErr != nil {
		return errors.NewError("grep: invalid pattern: %s", reErr.Error())
	}

	info, statErr := os.Stat(searchPath)
	if statErr != nil {
		return errors.NewError("grep: cannot stat path: %s", statErr.Error())
	}

	var matches []matchResult
	if info.IsDir() {
		matches = g.searchDir(ctx, searchPath, re, opts)
	} else {
		matches = g.searchFile(searchPath, re, opts)
	}

	return matchesToList(matches)
}

// searchDir walks a directory and searches files concurrently.
func (g *grepLibraryInstance) searchDir(ctx context.Context, root string, re *regexp.Regexp, opts grepOptions) []matchResult {
	type job struct{ path string }

	jobs := make(chan job, 64)
	resultsCh := make(chan []matchResult, 64)

	n := workerCount()
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				resultsCh <- g.searchFile(j.path, re, opts)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	// Walk in a separate goroutine so workers can start immediately.
	go func() {
		defer close(jobs)
		filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				if d != nil && d.IsDir() && !opts.recursive && path != root {
					return filepath.SkipDir
				}
				return nil
			}

			// Handle symlinks
			if d.Type()&os.ModeSymlink != 0 {
				if !opts.followLinks {
					return nil
				}
				real, err := filepath.EvalSymlinks(path)
				if err != nil {
					return nil
				}
				if !g.config.IsPathAllowed(real) {
					return nil
				}
			} else if !g.config.IsPathAllowed(path) {
				return nil
			}

			// Glob filter
			if opts.glob != "" {
				matched, err := filepath.Match(opts.glob, filepath.Base(path))
				if err != nil || !matched {
					return nil
				}
			}

			select {
			case <-ctx.Done():
				return filepath.SkipAll
			case jobs <- job{path: path}:
			}
			return nil
		})
	}()

	var all []matchResult
	for r := range resultsCh {
		all = append(all, r...)
	}
	return all
}

// searchFile searches a single file for matches.
func (g *grepLibraryInstance) searchFile(path string, re *regexp.Regexp, opts grepOptions) []matchResult {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	// Max size check
	if opts.maxSize > 0 {
		info, err := f.Stat()
		if err != nil || info.Size() > opts.maxSize {
			return nil
		}
	}

	// Binary sniff
	sniff := make([]byte, grepBinarySniffLen)
	n, _ := f.Read(sniff)
	if bytes.IndexByte(sniff[:n], 0) >= 0 {
		return nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil
	}

	var results []matchResult
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			results = append(results, matchResult{
				File: path,
				Line: lineNum,
				Text: strings.TrimRight(line, "\r"),
			})
		}
	}
	return results
}

// matchesToList converts []matchResult to a Scriptling list of dicts.
func matchesToList(matches []matchResult) object.Object {
	elements := make([]object.Object, len(matches))
	for i, m := range matches {
		d := &object.Dict{Pairs: make(map[string]object.DictPair)}
		d.SetByString("file", &object.String{Value: m.File})
		d.SetByString("line", object.NewInteger(int64(m.Line)))
		d.SetByString("text", &object.String{Value: m.Text})
		elements[i] = d
	}
	return &object.List{Elements: elements}
}

// checkPath validates a path against the security config.
func (g *grepLibraryInstance) checkPath(path string) object.Object {
	if !g.config.IsPathAllowed(path) {
		return errors.NewPermissionError("access denied: path '%s' is outside allowed directories", path)
	}
	return nil
}

const grepPatternHelp = `pattern(regex, path, *, recursive=False, ignore_case=False, glob="", follow_links=False, max_size=1048576) -> list

Search for a regex pattern in a file or directory. Returns a list of match dicts:
  {"file": str, "line": int, "text": str}

Parameters:
  regex        Regular expression pattern
  path         File or directory to search
  recursive    Recurse into subdirectories (default: False)
  ignore_case  Case-insensitive matching (default: False)
  glob         Only search files matching this glob pattern, e.g. "*.py"
  follow_links Follow symlinks if they resolve within allowed paths (default: False)
  max_size     Skip files larger than this many bytes (default: 1 MiB, None = no limit)`

const grepStringHelp = `string(text, path, *, recursive=False, ignore_case=False, glob="", follow_links=False, max_size=1048576) -> list

Search for a literal string in a file or directory. Returns a list of match dicts:
  {"file": str, "line": int, "text": str}

Parameters:
  text         Literal string to search for (not interpreted as regex)
  path         File or directory to search
  recursive    Recurse into subdirectories (default: False)
  ignore_case  Case-insensitive matching (default: False)
  glob         Only search files matching this glob pattern, e.g. "*.py"
  follow_links Follow symlinks if they resolve within allowed paths (default: False)
  max_size     Skip files larger than this many bytes (default: 1 MiB, None = no limit)`
