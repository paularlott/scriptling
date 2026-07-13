package extlibs

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

func normalizeFileIOAllowedPaths(config fssecurity.Config) fssecurity.Config {
	if config.AllowedPaths == nil {
		return config
	}

	normalizedPaths := make([]string, 0, len(config.AllowedPaths))
	for _, p := range config.AllowedPaths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		normalizedPaths = append(normalizedPaths, filepath.Clean(absPath))
	}
	config.AllowedPaths = normalizedPaths
	return config
}

func parseFileMode(args []object.Object, kwargs object.Kwargs, index int, defaultMode os.FileMode) (os.FileMode, object.Object) {
	mode := int64(defaultMode)
	if len(args) > index {
		if kwargs.Has("mode") {
			return 0, errors.NewError("mode specified both positionally and by keyword")
		}
		var err object.Object
		mode, err = args[index].AsInt()
		if err != nil {
			return 0, errors.NewTypeError("INTEGER", args[index].Type().String())
		}
	} else if val := kwargs.Get("mode"); val != nil {
		var err object.Object
		mode, err = val.AsInt()
		if err != nil {
			return 0, errors.NewTypeError("INTEGER", val.Type().String())
		}
	}
	if mode < 0 {
		return 0, errors.NewError("mode must be non-negative")
	}
	return os.FileMode(mode), nil
}

func checkPathSecurity(config fssecurity.Config, path string) object.Object {
	if !config.IsPathAllowed(path) {
		return errors.NewPermissionError("access denied: path '%s' is outside allowed directories", path)
	}
	return nil
}

func readFileBytes(ctx context.Context, config fssecurity.Config, path string) ([]byte, object.Object) {
	if err := checkPathSecurity(config, path); err != nil {
		return nil, err
	}
	var content []byte
	var err error
	object.RunBlocking(ctx, func() { content, err = os.ReadFile(path) })
	if err != nil {
		return nil, errors.NewError("cannot read file: %s", err.Error())
	}
	return content, nil
}

func writeFileBytes(ctx context.Context, config fssecurity.Config, path string, data []byte, mode os.FileMode) object.Object {
	if err := checkPathSecurity(config, path); err != nil {
		return err
	}
	var err error
	object.RunBlocking(ctx, func() { err = os.WriteFile(path, data, mode) })
	if err != nil {
		return errors.NewError("cannot write file: %s", err.Error())
	}
	return &object.Null{}
}

func appendFileBytes(ctx context.Context, config fssecurity.Config, path string, data []byte, mode os.FileMode) object.Object {
	if err := checkPathSecurity(config, path); err != nil {
		return err
	}
	var err error
	object.RunBlocking(ctx, func() {
		var f *os.File
		f, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, mode)
		if err != nil {
			return
		}
		defer f.Close()
		_, err = f.Write(data)
	})
	if err != nil {
		return errors.NewError("cannot append to file: %s", err.Error())
	}
	return &object.Null{}
}

func readFileBytesAt(ctx context.Context, config fssecurity.Config, path string, offset, length, maxLength int64) ([]byte, object.Object) {
	if offset < 0 {
		return nil, errors.NewError("read_bytes: offset must be non-negative")
	}
	if length < 0 {
		return nil, errors.NewError("read_bytes: length must be non-negative")
	}
	if maxLength > 0 && length > maxLength {
		return nil, errors.NewError("read_bytes: length exceeds maximum of %d bytes", maxLength)
	}
	if err := checkPathSecurity(config, path); err != nil {
		return nil, err
	}

	var buf []byte
	var n int
	var err error
	object.RunBlocking(ctx, func() {
		var file *os.File
		file, err = os.Open(path)
		if err != nil {
			return
		}
		defer file.Close()
		buf = make([]byte, length)
		n, err = file.ReadAt(buf, offset)
	})
	if err != nil && n == 0 {
		return nil, errors.NewError("read_bytes: cannot read file: %s", err.Error())
	}
	return buf[:n], nil
}

func writeFileBytesAt(ctx context.Context, config fssecurity.Config, path string, offset int64, data []byte, mode os.FileMode) object.Object {
	if offset < 0 {
		return errors.NewError("write_bytes: offset must be non-negative")
	}
	if err := checkPathSecurity(config, path); err != nil {
		return err
	}

	var err error
	object.RunBlocking(ctx, func() {
		var file *os.File
		file, err = os.OpenFile(path, os.O_CREATE|os.O_RDWR, mode)
		if err != nil {
			return
		}
		defer file.Close()
		_, err = file.WriteAt(data, offset)
	})
	if err != nil {
		return errors.NewError("write_bytes: cannot write to file: %s", err.Error())
	}
	return &object.Null{}
}

func chmodPath(config fssecurity.Config, path string, mode os.FileMode) object.Object {
	if err := checkPathSecurity(config, path); err != nil {
		return err
	}
	if err := os.Chmod(path, mode); err != nil {
		return errors.NewError("cannot change mode: %s", err.Error())
	}
	return &object.Null{}
}

func removePath(config fssecurity.Config, path string, target string, missingOk bool) object.Object {
	if err := checkPathSecurity(config, path); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if missingOk && os.IsNotExist(err) {
			return &object.Null{}
		}
		return errors.NewError("cannot remove %s: %s", target, err.Error())
	}
	return &object.Null{}
}

func copyPath(config fssecurity.Config, src string, dst string) object.Object {
	if err := checkPathSecurity(config, src); err != nil {
		return err
	}
	if err := checkPathSecurity(config, dst); err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		return errors.NewError("cannot copy: %s", err.Error())
	}

	if info.IsDir() {
		return copyDir(config, src, dst, info.Mode())
	}
	return copyFile(src, dst, info.Mode())
}

func copyFile(src string, dst string, mode os.FileMode) object.Object {
	srcFile, err := os.Open(src)
	if err != nil {
		return errors.NewError("cannot copy: %s", err.Error())
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return errors.NewError("cannot copy: %s", err.Error())
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return errors.NewError("cannot copy: %s", err.Error())
	}
	return nil
}

func copyDir(config fssecurity.Config, src string, dst string, mode os.FileMode) object.Object {
	if err := os.MkdirAll(dst, mode); err != nil {
		return errors.NewError("cannot copy directory: %s", err.Error())
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return errors.NewError("cannot copy directory: %s", err.Error())
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		info, err := entry.Info()
		if err != nil {
			return errors.NewError("cannot copy directory: %s", err.Error())
		}

		if entry.IsDir() {
			if result := copyDir(config, srcPath, dstPath, info.Mode()); result != nil {
				return result
			}
		} else {
			if result := copyFile(srcPath, dstPath, info.Mode()); result != nil {
				return result
			}
		}
	}
	return nil
}

func renamePath(config fssecurity.Config, oldPath string, newPath string) object.Object {
	if err := checkPathSecurity(config, oldPath); err != nil {
		return err
	}
	if err := checkPathSecurity(config, newPath); err != nil {
		return err
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return errors.NewError("cannot rename: %s", err.Error())
	}
	return &object.Null{}
}

func statPath(config fssecurity.Config, path string, action string) (os.FileInfo, object.Object) {
	if err := checkPathSecurity(config, path); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if action == "" {
			return nil, nil
		}
		return nil, errors.NewError("%s: %s", action, err.Error())
	}
	return info, nil
}

func mkdirPath(config fssecurity.Config, path string, mode os.FileMode, parents bool, existOk bool) object.Object {
	if err := checkPathSecurity(config, path); err != nil {
		return err
	}

	var err error
	if parents {
		if !existOk {
			if _, statErr := os.Stat(path); statErr == nil {
				return errors.NewError("cannot create directory: file exists")
			}
		}
		err = os.MkdirAll(path, mode)
	} else {
		err = os.Mkdir(path, mode)
		if existOk && os.IsExist(err) {
			if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
				return &object.Null{}
			}
		}
	}
	if err != nil {
		return errors.NewError("cannot create directory: %s", err.Error())
	}
	return &object.Null{}
}

func removeDirs(config fssecurity.Config, path string) object.Object {
	if err := checkPathSecurity(config, path); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return errors.NewError("cannot remove directory: %s", err.Error())
	}

	parent := filepath.Dir(filepath.Clean(path))
	for parent != "." && parent != string(os.PathSeparator) {
		if isAllowedRoot(config, parent) {
			break
		}
		if !config.IsPathAllowed(parent) {
			break
		}
		if err := os.Remove(parent); err != nil {
			break
		}
		next := filepath.Dir(parent)
		if next == parent {
			break
		}
		parent = next
	}

	return &object.Null{}
}

func isAllowedRoot(config fssecurity.Config, path string) bool {
	if config.AllowedPaths == nil {
		return false
	}
	cleanPath := filepath.Clean(path)
	for _, allowedPath := range config.AllowedPaths {
		if cleanPath == filepath.Clean(allowedPath) {
			return true
		}
	}
	return false
}

// globMatches finds all paths matching pattern relative to rootDir.
//
// When recursive is true and the pattern contains "**", a bounded parallel
// directory walk is used (the same worker-pool model as scriptling.grep).
// When recursive is false, any "**" in the pattern is collapsed to "*" to
// match Python's glob semantics. includeHidden controls whether entries whose
// name starts with "." are matched; when false such entries are skipped, which
// matches Python's default include_hidden=False behaviour.
func globMatches(ctx context.Context, config fssecurity.Config, pattern, rootDir string, recursive, includeHidden bool) []string {
	if recursive && strings.Contains(pattern, "**") {
		return globRecursive(ctx, config, pattern, rootDir, includeHidden)
	}

	// Non-recursive: collapse "**" to "*" so it behaves as a single-level match.
	effective := pattern
	if !recursive {
		effective = strings.ReplaceAll(pattern, "**", "*")
	}
	matches, _ := filepath.Glob(filepath.Join(rootDir, effective))

	filtered := make([]string, 0, len(matches))
	for _, match := range matches {
		if !config.IsPathAllowed(match) {
			continue
		}
		if !includeHidden && strings.HasPrefix(filepath.Base(match), ".") {
			continue
		}
		filtered = append(filtered, match)
	}
	return filtered
}

// globRecursive expands a "**" pattern against rootDir using a bounded pool of
// goroutines so that directory reads run concurrently. The pattern is split at
// the first "**" into a prefix (resolved literally) and a suffix (matched in
// every visited directory). includeHidden gates descent into hidden ("."-prefixed)
// directories and matching of hidden suffix results, mirroring Python.
func globRecursive(ctx context.Context, config fssecurity.Config, pattern, rootDir string, includeHidden bool) []string {
	parts := strings.SplitN(pattern, "**", 2)
	prefixPart := parts[0]
	suffixPart := ""
	if len(parts) == 2 {
		suffixPart = parts[1]
	}

	prefix := strings.TrimSuffix(filepath.Join(rootDir, prefixPart), string(filepath.Separator))
	suffix := strings.TrimPrefix(suffixPart, string(filepath.Separator))

	prefixMatches, _ := filepath.Glob(prefix)
	if len(prefixMatches) == 0 {
		prefixMatches = []string{prefix}
	}

	// Seed with allowed prefix directories. The prefix is literal, so it is
	// followed regardless of includeHidden.
	roots := make([]string, 0, len(prefixMatches))
	for _, p := range prefixMatches {
		if config.IsPathAllowed(p) {
			roots = append(roots, p)
		}
	}
	if len(roots) == 0 {
		return nil
	}

	q := newDirQueue()
	var (
		mu      sync.Mutex
		all     []string
		pending int64
	)
	for _, r := range roots {
		atomic.AddInt64(&pending, 1)
		q.push(r)
	}

	// Best-effort cancellation: closing the queue wakes blocked workers.
	doneCh := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			q.close()
		case <-doneCh:
		}
	}()

	var doneOnce sync.Once
	finish := func() {
		doneOnce.Do(func() {
			close(doneCh)
			q.close()
		})
	}

	workers := workerCount()
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				base, ok := q.pop()
				if !ok {
					return
				}

				// Match the suffix in this directory.
				var localMatches []string
				directPath := filepath.Join(base, suffix)
				matches, _ := filepath.Glob(directPath)
				for _, m := range matches {
					if !config.IsPathAllowed(m) {
						continue
					}
					if !includeHidden && suffix != "" && strings.HasPrefix(filepath.Base(m), ".") {
						continue
					}
					localMatches = append(localMatches, m)
				}

				// Discover subdirectories for the "**" expansion.
				entries, _ := os.ReadDir(base)
				for _, e := range entries {
					if !e.IsDir() {
						continue
					}
					name := e.Name()
					if !includeHidden && strings.HasPrefix(name, ".") {
						continue
					}
					full := filepath.Join(base, name)
					if !config.IsPathAllowed(full) {
						continue
					}
					atomic.AddInt64(&pending, 1)
					q.push(full)
				}

				if len(localMatches) > 0 {
					mu.Lock()
					all = append(all, localMatches...)
					mu.Unlock()
				}

				if atomic.AddInt64(&pending, -1) == 0 {
					finish()
					return
				}
			}
		}()
	}

	object.RunBlocking(ctx, func() {
		wg.Wait()
	})
	finish()
	return all
}

// dirQueue is a thread-safe, unbounded FIFO of directory paths used to
// distribute work among the globRecursive worker pool. pop blocks until an
// item is available or the queue is closed.
type dirQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  []string
	closed bool
}

func newDirQueue() *dirQueue {
	q := &dirQueue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *dirQueue) push(s string) {
	q.mu.Lock()
	q.items = append(q.items, s)
	q.cond.Signal()
	q.mu.Unlock()
}

func (q *dirQueue) pop() (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.items) == 0 {
		return "", false
	}
	s := q.items[0]
	q.items = q.items[1:]
	return s, true
}

func (q *dirQueue) close() {
	q.mu.Lock()
	q.closed = true
	q.cond.Broadcast()
	q.mu.Unlock()
}
