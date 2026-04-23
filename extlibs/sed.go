// Package extlibs provides external libraries that need explicit registration
package extlibs

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

// sedLibraryInstance holds the configured text library instance.
type sedLibraryInstance struct {
	config fssecurity.Config
}

// RegisterSedLibrary registers the scriptling.sed library with a Scriptling instance.
// If allowedPaths is nil, all paths are allowed. If non-nil, operations are restricted
// to those directories (same semantics as RegisterOSLibrary).
func RegisterSedLibrary(registrar object.LibraryRegistrar, allowedPaths []string) {
	config := fssecurity.Config{AllowedPaths: allowedPaths}
	registrar.RegisterLibrary(NewSedLibrary(config))
}

// NewSedLibrary creates a new scriptling.sed library with the given configuration.
func NewSedLibrary(config fssecurity.Config) *object.Library {
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
	inst := &sedLibraryInstance{config: config}
	return inst.createLibrary()
}

func (t *sedLibraryInstance) createLibrary() *object.Library {
	return object.NewLibrary(SedLibraryName, map[string]*object.Builtin{
		"replace": {
			Fn:       t.fnReplace,
			HelpText: textReplaceHelp,
		},
		"replace_pattern": {
			Fn:       t.fnReplacePattern,
			HelpText: textReplacePatternHelp,
		},
		"extract": {
			Fn:       t.fnExtract,
			HelpText: textExtractHelp,
		},
	}, nil, "In-place file content replacement and capture group extraction")
}

// fnReplace implements text.replace(old, new, path, **kwargs) — literal replacement.
func (t *sedLibraryInstance) fnReplace(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	return t.run(ctx, kwargs, args, true)
}

// fnReplacePattern implements text.replace_pattern(regex, new, path, **kwargs) — regex replacement.
func (t *sedLibraryInstance) fnReplacePattern(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	return t.run(ctx, kwargs, args, false)
}

// run is the shared implementation for replace and replace_pattern.
// args: [needle, replacement, path]
func (t *sedLibraryInstance) run(ctx context.Context, kwargs object.Kwargs, args []object.Object, literal bool) object.Object {
	if err := errors.ExactArgs(args, 3); err != nil {
		return err
	}
	needle, err := args[0].AsString()
	if err != nil {
		return err
	}
	replacement, err := args[1].AsString()
	if err != nil {
		return err
	}
	targetPath, err := args[2].AsString()
	if err != nil {
		return err
	}

	if !t.config.IsPathAllowed(targetPath) {
		return errors.NewPermissionError("access denied: path '%s' is outside allowed directories", targetPath)
	}

	opts, oErr := parseFileopsKwargs(kwargs)
	if oErr != nil {
		return oErr
	}

	re, reErr := compilePattern(needle, literal, opts.ignoreCase)
	if reErr != nil {
		return errors.NewError("text: invalid pattern: %s", reErr.Error())
	}

	info, statErr := os.Stat(targetPath)
	if statErr != nil {
		return errors.NewError("text: cannot stat path: %s", statErr.Error())
	}

	var count int64
	if info.IsDir() {
		count = t.replaceDir(ctx, targetPath, re, replacement, opts)
	} else {
		if t.replaceFile(targetPath, re, replacement, opts) {
			count = 1
		}
	}

	return object.NewInteger(count)
}

// replaceDir walks a directory and replaces concurrently, returning the number of files modified.
func (t *sedLibraryInstance) replaceDir(ctx context.Context, root string, re *regexp.Regexp, replacement string, opts fileopsOptions) int64 {
	jobs := make(chan string, 64)
	resultsCh := make(chan bool, 64)

	var wg sync.WaitGroup
	for i := 0; i < workerCount(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				resultsCh <- t.replaceFile(path, re, replacement, opts)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	go walkFiles(ctx, root, opts, t.config, jobs)

	var count int64
	for modified := range resultsCh {
		if modified {
			count++
		}
	}
	return count
}

// replaceFile performs in-place replacement on a single file using a temp file + rename.
// Returns true if the file was modified.
func (t *sedLibraryInstance) replaceFile(path string, re *regexp.Regexp, replacement string, opts fileopsOptions) bool {
	f, ok := openTextFile(path, opts.maxSize)
	if !ok {
		return false
	}

	// Read all lines, track whether anything changed.
	var lines []string
	modified := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		replaced := re.ReplaceAllString(line, replacement)
		lines = append(lines, replaced)
		if replaced != line {
			modified = true
		}
	}
	f.Close()

	if !modified {
		return false
	}

	// Write to a temp file in the same directory, then rename atomically.
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".scriptling-text-*")
	if err != nil {
		return false
	}
	tmpName := tmp.Name()

	w := bufio.NewWriter(tmp)
	for i, line := range lines {
		if i > 0 {
			w.WriteByte('\n')
		}
		w.WriteString(line)
	}
	// Preserve trailing newline if original file had one.
	if len(lines) > 0 {
		w.WriteByte('\n')
	}

	if err := w.Flush(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return false
	}

	// Copy permissions from original file.
	if info, err := os.Stat(path); err == nil {
		tmp.Chmod(info.Mode())
	}
	tmp.Close()

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return false
	}

	return true
}


// fnExtract implements text.extract(regex, path, **kwargs) — capture group extraction.
func (t *sedLibraryInstance) fnExtract(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 2); err != nil {
		return err
	}
	pattern, err := args[0].AsString()
	if err != nil {
		return err
	}
	targetPath, err := args[1].AsString()
	if err != nil {
		return err
	}

	if !t.config.IsPathAllowed(targetPath) {
		return errors.NewPermissionError("access denied: path '%s' is outside allowed directories", targetPath)
	}

	opts, oErr := parseFileopsKwargs(kwargs)
	if oErr != nil {
		return oErr
	}

	re, reErr := compilePattern(pattern, false, opts.ignoreCase)
	if reErr != nil {
		return errors.NewError("text: invalid pattern: %s", reErr.Error())
	}

	info, statErr := os.Stat(targetPath)
	if statErr != nil {
		return errors.NewError("text: cannot stat path: %s", statErr.Error())
	}

	var results []extractResult
	if info.IsDir() {
		results = t.extractDir(ctx, targetPath, re, opts)
	} else {
		results = t.extractFile(targetPath, re, opts)
	}

	return extractToList(results)
}

// extractResult holds a single line's capture groups.
type extractResult struct {
	File   string
	Line   int
	Text   string
	Groups []string
}

// extractDir walks a directory extracting captures concurrently.
func (t *sedLibraryInstance) extractDir(ctx context.Context, root string, re *regexp.Regexp, opts fileopsOptions) []extractResult {
	jobs := make(chan string, 64)
	resultsCh := make(chan []extractResult, 64)

	var wg sync.WaitGroup
	for i := 0; i < workerCount(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				resultsCh <- t.extractFile(path, re, opts)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	go walkFiles(ctx, root, opts, t.config, jobs)

	var all []extractResult
	for r := range resultsCh {
		all = append(all, r...)
	}
	return all
}

// extractFile scans a single file returning all capture group matches.
func (t *sedLibraryInstance) extractFile(path string, re *regexp.Regexp, opts fileopsOptions) []extractResult {
	f, ok := openTextFile(path, opts.maxSize)
	if !ok {
		return nil
	}
	defer f.Close()

	var results []extractResult
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		matches := re.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			// m[0] is the full match, m[1:] are the capture groups
			groups := make([]string, len(m)-1)
			copy(groups, m[1:])
			results = append(results, extractResult{
				File:   path,
				Line:   lineNum,
				Text:   line,
				Groups: groups,
			})
		}
	}
	return results
}

// extractToList converts []extractResult to a Scriptling list of dicts.
func extractToList(results []extractResult) object.Object {
	elements := make([]object.Object, len(results))
	for i, r := range results {
		d := &object.Dict{Pairs: make(map[string]object.DictPair)}
		d.SetByString("file", &object.String{Value: r.File})
		d.SetByString("line", object.NewInteger(int64(r.Line)))
		d.SetByString("text", &object.String{Value: r.Text})
		groupElems := make([]object.Object, len(r.Groups))
		for j, g := range r.Groups {
			groupElems[j] = &object.String{Value: g}
		}
		d.SetByString("groups", &object.List{Elements: groupElems})
		elements[i] = d
	}
	return &object.List{Elements: elements}
}

const textExtractHelp = `extract(regex, path, *, recursive=False, ignore_case=False, glob="", follow_links=False, max_size=1048576) -> list

Extract regex capture groups from a file or directory. Returns a list of match dicts:
  {"file": str, "line": int, "text": str, "groups": list}

Parameters:
  regex        Regular expression with capture groups
  path         File or directory to search
  recursive    Recurse into subdirectories (default: False)
  ignore_case  Case-insensitive matching (default: False)
  glob         Only search files matching this glob pattern, e.g. "*.py"
  follow_links Follow symlinks if they resolve within allowed paths (default: False)
  max_size     Skip files larger than this many bytes (default: 1 MiB, None = no limit)`

const textReplaceHelp = `replace(old, new, path, *, recursive=False, ignore_case=False, glob="", follow_links=False, max_size=1048576) -> int

Replace all occurrences of a literal string in a file or directory.
Files are modified in-place using atomic temp-file rename.
Returns the number of files modified.

Parameters:
  old          Literal string to search for
  new          Replacement string
  path         File or directory to modify
  recursive    Recurse into subdirectories (default: False)
  ignore_case  Case-insensitive matching (default: False)
  glob         Only modify files matching this glob pattern, e.g. "*.py"
  follow_links Follow symlinks if they resolve within allowed paths (default: False)
  max_size     Skip files larger than this many bytes (default: 1 MiB, None = no limit)`

const textReplacePatternHelp = "replace_pattern(regex, new, path, *, recursive=False, ignore_case=False, glob=\"\", follow_links=False, max_size=1048576) -> int\n\n" +
	"Replace all regex matches in a file or directory.\n" +
	"Files are modified in-place using atomic temp-file rename.\n" +
	"Capture groups are supported in the replacement string (e.g. ${1}, ${name}).\n" +
	"Returns the number of files modified.\n\n" +
	"Parameters:\n" +
	"  regex        Regular expression pattern\n" +
	"  new          Replacement string (may reference capture groups as ${1}, ${2}, etc.)\n" +
	"  path         File or directory to modify\n" +
	"  recursive    Recurse into subdirectories (default: False)\n" +
	"  ignore_case  Case-insensitive matching (default: False)\n" +
	"  glob         Only modify files matching this glob pattern, e.g. \"*.py\"\n" +
	"  follow_links Follow symlinks if they resolve within allowed paths (default: False)\n" +
	"  max_size     Skip files larger than this many bytes (default: 1 MiB, None = no limit)"
