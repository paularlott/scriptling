// Package extlibs provides external libraries that need explicit registration
package extlibs

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

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

// matchResult is a single line match.
type matchResult struct {
	File string
	Line int
	Text string
}

// fnPattern implements grep.pattern(regex, path, **kwargs) — regex search.
func (g *grepLibraryInstance) fnPattern(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	return g.run(ctx, kwargs, args, false)
}

// fnString implements grep.string(text, path, **kwargs) — literal string search.
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

	if !g.config.IsPathAllowed(searchPath) {
		return errors.NewPermissionError("access denied: path '%s' is outside allowed directories", searchPath)
	}

	opts, oErr := parseFileopsKwargs(kwargs)
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
func (g *grepLibraryInstance) searchDir(ctx context.Context, root string, re *regexp.Regexp, opts fileopsOptions) []matchResult {
	jobs := make(chan string, 64)
	resultsCh := make(chan []matchResult, 64)

	var wg sync.WaitGroup
	for i := 0; i < workerCount(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				resultsCh <- g.searchFile(path, re, opts)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	go walkFiles(ctx, root, opts, g.config, jobs)

	var all []matchResult
	for r := range resultsCh {
		all = append(all, r...)
	}
	return all
}

// searchFile searches a single file for matches.
func (g *grepLibraryInstance) searchFile(path string, re *regexp.Regexp, opts fileopsOptions) []matchResult {
	f, ok := openTextFile(path, opts.maxSize)
	if !ok {
		return nil
	}
	defer f.Close()

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
