// Package extlibs provides external libraries that need explicit registration
package extlibs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

// findLibraryInstance holds the configured find library instance.
type findLibraryInstance struct {
	config fssecurity.Config
}

// RegisterFindLibrary registers the scriptling.find library with a Scriptling
// instance. If allowedPaths is nil, all paths are allowed. If non-nil, all find
// operations are restricted to those directories (same semantics as
// RegisterGrepLibrary).
func RegisterFindLibrary(registrar object.LibraryRegistrar, allowedPaths []string) {
	registrar.RegisterLibrary(NewFindLibrary(fssecurity.Config{AllowedPaths: allowedPaths}))
}

// NewFindLibrary creates a new scriptling.find library with the given configuration.
func NewFindLibrary(config fssecurity.Config) *object.Library {
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
	inst := &findLibraryInstance{config: config}
	return inst.createLibrary()
}

// findOptions holds the parsed filters shared by the find functions.
type findOptions struct {
	recursive     bool
	entryType     string // "any", "file", "dir"
	name          string // shell-style glob matched against the base name
	mtimeMin      float64
	mtimeMax      float64
	hasMtimeMin   bool
	hasMtimeMax   bool
	sizeMin       int64
	sizeMax       int64
	hasSizeMin    bool
	hasSizeMax    bool
	includeHidden bool
	followLinks   bool
	maxDepth      int // 0 = unlimited
}

func (f *findLibraryInstance) createLibrary() *object.Library {
	return object.NewLibrary(FindLibraryName, map[string]*object.Builtin{
		"path": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				searchPath, err := args[0].AsString()
				if err != nil {
					return err
				}
				if !f.config.IsPathAllowed(searchPath) {
					return errors.NewPermissionError("access denied: path '%s' is outside allowed directories", searchPath)
				}

				opts, oErr := parseFindKwargs(kwargs)
				if oErr != nil {
					return oErr
				}

				matches := f.findPaths(ctx, searchPath, opts)

				elements := make([]object.Object, len(matches))
				for i, m := range matches {
					elements[i] = object.NewString(m)
				}
				return &object.List{Elements: elements}
			},
			HelpText: findPathHelp,
		},
	}, nil, "Find files and directories by name, type, mtime, and size")
}

// parseFindKwargs reads the keyword-only options for find.path.
func parseFindKwargs(kwargs object.Kwargs) (findOptions, object.Object) {
	opts := findOptions{
		recursive: true, // find descends by default, matching the find command
		entryType: "any",
	}

	if v := kwargs.Get("recursive"); v != nil {
		b, err := v.AsBool()
		if err != nil {
			return opts, err
		}
		opts.recursive = b
	}
	if v := kwargs.Get("type"); v != nil {
		s, err := v.AsString()
		if err != nil {
			return opts, err
		}
		switch s {
		case "any", "file", "dir":
			opts.entryType = s
		default:
			return opts, errors.NewError("find: type must be 'any', 'file', or 'dir', got '%s'", s)
		}
	}
	if v := kwargs.Get("name"); v != nil {
		s, err := v.AsString()
		if err != nil {
			return opts, err
		}
		opts.name = s
	}
	if v := kwargs.Get("include_hidden"); v != nil {
		b, err := v.AsBool()
		if err != nil {
			return opts, err
		}
		opts.includeHidden = b
	}
	if v := kwargs.Get("follow_links"); v != nil {
		b, err := v.AsBool()
		if err != nil {
			return opts, err
		}
		opts.followLinks = b
	}
	if v := kwargs.Get("max_depth"); v != nil {
		n, err := v.AsInt()
		if err != nil {
			return opts, err
		}
		opts.maxDepth = int(n)
	}
	if v := kwargs.Get("mtime_min"); v != nil {
		fl, err := v.AsFloat()
		if err != nil {
			return opts, err
		}
		opts.mtimeMin = fl
		opts.hasMtimeMin = true
	}
	if v := kwargs.Get("mtime_max"); v != nil {
		fl, err := v.AsFloat()
		if err != nil {
			return opts, err
		}
		opts.mtimeMax = fl
		opts.hasMtimeMax = true
	}
	if v := kwargs.Get("size_min"); v != nil {
		n, err := v.AsInt()
		if err != nil {
			return opts, err
		}
		opts.sizeMin = n
		opts.hasSizeMin = true
	}
	if v := kwargs.Get("size_max"); v != nil {
		n, err := v.AsInt()
		if err != nil {
			return opts, err
		}
		opts.sizeMax = n
		opts.hasSizeMax = true
	}

	return opts, nil
}

// findPaths walks searchPath applying the filters, returning matching paths.
// A single-file search path is checked directly; a directory is walked with a
// bounded worker pool so stat calls run concurrently (same model as grep).
func (f *findLibraryInstance) findPaths(ctx context.Context, searchPath string, opts findOptions) []string {
	info, err := os.Stat(searchPath)
	if err != nil {
		return nil
	}

	// Single-file root: check it directly against the filters.
	if !info.IsDir() {
		if opts.name != "" {
			matched, mErr := filepath.Match(opts.name, filepath.Base(searchPath))
			if mErr != nil || !matched {
				return nil
			}
		}
		switch opts.entryType {
		case "dir":
			return nil
		case "file":
			if !info.Mode().IsRegular() {
				return nil
			}
		}
		if matchSizeMtime(info, opts) {
			return []string{searchPath}
		}
		return nil
	}

	jobs := make(chan string, 64)
	resultsCh := make(chan []string, 64)

	var wg sync.WaitGroup
	for i := 0; i < workerCount(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				resultsCh <- filterEntry(path, opts)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	go walkEntries(ctx, searchPath, opts, f.config, jobs)

	var all []string
	object.RunBlocking(ctx, func() {
		for batch := range resultsCh {
			all = append(all, batch...)
		}
	})
	return all
}

// filterEntry applies the remaining size/mtime filters to an entry that has
// already passed the name and type checks in the walker. When no size/mtime
// filter is active, no stat is performed at all.
func filterEntry(path string, opts findOptions) []string {
	if !opts.hasSizeMin && !opts.hasSizeMax && !opts.hasMtimeMin && !opts.hasMtimeMax {
		return []string{path}
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if matchSizeMtime(info, opts) {
		return []string{path}
	}
	return nil
}

// matchSizeMtime checks the size and mtime filters against an already-statted
// entry. Name and type are assumed to have been applied by the caller.
func matchSizeMtime(info os.FileInfo, opts findOptions) bool {
	if opts.hasSizeMin && info.Size() < opts.sizeMin {
		return false
	}
	if opts.hasSizeMax && info.Size() > opts.sizeMax {
		return false
	}

	if opts.hasMtimeMin || opts.hasMtimeMax {
		mtime := float64(info.ModTime().UnixNano()) / 1e9
		if opts.hasMtimeMin && mtime < opts.mtimeMin {
			return false
		}
		if opts.hasMtimeMax && mtime > opts.mtimeMax {
			return false
		}
	}

	return true
}

// entryTypeMatches checks whether a DirEntry satisfies the type filter using
// the entry's own type bits — no stat for regular entries. Symlinks (which
// only reach here with follow_links=True) are stat'd to resolve the target.
func entryTypeMatches(d os.DirEntry, path, entryType string) bool {
	switch entryType {
	case "any":
		return true
	case "dir":
		return d.IsDir()
	case "file":
		if d.Type()&os.ModeSymlink != 0 {
			if info, err := os.Stat(path); err == nil {
				return info.Mode().IsRegular()
			}
			return false
		}
		return d.Type().IsRegular()
	}
	return true
}

// walkEntries walks root dispatching every entry path (files and directories)
// to jobs, applying security, symlink, hidden, name, recursion, and depth
// filters. The root directory itself is not emitted. jobs is closed when the
// walk ends.
func walkEntries(ctx context.Context, root string, opts findOptions, config fssecurity.Config, jobs chan<- string) {
	defer close(jobs)

	skippedRoot := false
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Skip the root entry itself but keep descending.
		if !skippedRoot {
			skippedRoot = true
			return nil
		}

		name := d.Name()

		// Hidden filter: skip dot-entries and their subtrees.
		if !opts.includeHidden && strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Symlink handling: WalkDir does not follow symlinks, so a symlinked
		// entry is yielded for the worker to stat (and thus follow) when
		// follow_links is set; otherwise it is skipped.
		if d.Type()&os.ModeSymlink != 0 {
			if !opts.followLinks {
				return nil
			}
			real, e := filepath.EvalSymlinks(path)
			if e != nil || !config.IsPathAllowed(real) {
				return nil
			}
		} else if !config.IsPathAllowed(path) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Name filter: applied here (cheap, no stat) so non-matching entries
		// never traverse the channel or trigger a worker stat. A non-matching
		// directory is still descended so its children can match.
		nameOk := opts.name == ""
		if !nameOk {
			matched, mErr := filepath.Match(opts.name, name)
			nameOk = mErr == nil && matched
		}

		// Type filter: applied here using DirEntry type bits, so no stat is
		// needed for the common case (regular files / dirs). Only symlinks
		// with follow_links require a stat to resolve the target type.
		typeOk := entryTypeMatches(d, path, opts.entryType)

		if nameOk && typeOk {
			select {
			case <-ctx.Done():
				return filepath.SkipAll
			case jobs <- path:
			}
		}

		// Decide whether to descend into this directory. Non-matching files
		// fall through here and are simply dropped (never sent to jobs).
		if d.IsDir() {
			rel, _ := filepath.Rel(root, path)
			depth := strings.Count(rel, string(filepath.Separator)) + 1
			if !opts.recursive || (opts.maxDepth > 0 && depth >= opts.maxDepth) {
				return filepath.SkipDir
			}
		}
		return nil
	})
}

const findPathHelp = `path(path, *, recursive=True, type="any", name="", mtime_min=None, mtime_max=None, size_min=None, size_max=None, include_hidden=False, follow_links=False, max_depth=None) -> list

Find files and directories under a path by name, type, modification time, and
size. Returns a list of matching path strings in arbitrary order.

Parameters:
  path           Directory (or file) to search under.
  recursive      Descend into subdirectories (default: True). When False, only
                 the immediate children of path are examined.
  type           Restrict to "file", "dir", or "any" (default: "any").
  name           Shell-style glob pattern matched against the entry's base
                 name, e.g. "*.md". Empty matches everything (default).
  mtime_min      Include only entries modified at or after this epoch time
                 (float seconds). None = no lower bound (default).
  mtime_max      Include only entries modified at or before this epoch time
                 (float seconds). None = no upper bound (default).
  size_min       Include only entries whose size in bytes is >= this value.
                 None = no lower bound (default).
  size_max       Include only entries whose size in bytes is <= this value.
                 None = no upper bound (default).
  include_hidden When True, entries whose name starts with "." are matched;
                 when False (the default) they are skipped.
  follow_links   Follow symlinks if they resolve within allowed paths
                 (default: False).
  max_depth      Maximum recursion depth (1 = immediate children only).
                 None = unlimited (default).

Recursive searches stat and filter entries concurrently using a bounded worker
pool, the same model as scriptling.grep.`
