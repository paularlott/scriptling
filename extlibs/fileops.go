// Package extlibs provides external libraries that need explicit registration
package extlibs

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

// fileopsWorkerPoolSize is the minimum number of concurrent goroutines for file operations.
// The actual pool size is max(fileopsWorkerPoolSize, NumCPU/2).
const fileopsWorkerPoolSize = 4

// fileopsDefaultMaxFileSize is the default maximum file size (1 MiB, matching rg).
// Pass max_size=None to disable the limit.
const fileopsDefaultMaxFileSize int64 = 1 * 1024 * 1024

// fileopsBinarySniffLen is the number of bytes read to detect binary files.
const fileopsBinarySniffLen = 8000

// fileopsOptions holds parsed kwargs common to grep and text operations.
type fileopsOptions struct {
	recursive   bool
	ignoreCase  bool
	glob        string
	maxSize     int64
	followLinks bool
}

// workerCount returns the bounded worker pool size.
func workerCount() int {
	n := runtime.NumCPU() / 2
	if n < fileopsWorkerPoolSize {
		n = fileopsWorkerPoolSize
	}
	return n
}

// compilePattern compiles a search pattern into a *regexp.Regexp.
// If literal is true the pattern is quoted before compiling.
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

// parseFileopsKwargs parses the common kwargs shared by grep and text functions.
func parseFileopsKwargs(kwargs object.Kwargs) (fileopsOptions, object.Object) {
	opts := fileopsOptions{
		maxSize: fileopsDefaultMaxFileSize,
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

// isBinary reports whether the file looks like binary by sniffing for null bytes.
// Returns true (skip file) on any read error.
func isBinary(f *os.File) bool {
	sniff := make([]byte, fileopsBinarySniffLen)
	n, _ := f.Read(sniff)
	return bytes.IndexByte(sniff[:n], 0) >= 0
}

// checkFileSize returns true if the file exceeds maxSize and should be skipped.
func checkFileSize(f *os.File, maxSize int64) bool {
	if maxSize <= 0 {
		return false
	}
	info, err := f.Stat()
	return err != nil || info.Size() > maxSize
}

// walkFiles walks root dispatching file paths to the jobs channel, applying
// security, symlink, glob, and recursion filters. Closes jobs when done.
func walkFiles(ctx context.Context, root string, opts fileopsOptions, config fssecurity.Config, jobs chan<- string) {
	defer close(jobs)
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if !opts.recursive && path != root {
				return filepath.SkipDir
			}
			return nil
		}

		// Symlink handling
		if d.Type()&os.ModeSymlink != 0 {
			if !opts.followLinks {
				return nil
			}
			real, err := filepath.EvalSymlinks(path)
			if err != nil || !config.IsPathAllowed(real) {
				return nil
			}
		} else if !config.IsPathAllowed(path) {
			return nil
		}

		// Skip our own temp files
		if strings.HasPrefix(filepath.Base(path), ".scriptling-text-") {
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
		case jobs <- path:
		}
		return nil
	})
}

// openTextFile opens a file, checks size and binary content, and seeks back to
// the start ready for reading. Returns nil if the file should be skipped.
func openTextFile(path string, maxSize int64) (*os.File, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	if checkFileSize(f, maxSize) {
		f.Close()
		return nil, false
	}
	if isBinary(f) {
		f.Close()
		return nil, false
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return nil, false
	}
	return f, true
}
