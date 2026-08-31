package pack

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// UnpackOptions configures extraction behaviour.
type UnpackOptions struct {
	DestDir  string
	Force    bool
	List     bool
	Insecure bool
}

// Unpack extracts a package from a local path or URL.
func Unpack(src string, opts UnpackOptions) error {
	data, err := Fetch(src, opts.Insecure)
	if err != nil {
		return err
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ErrInvalidPackage
	}
	if err := checkExpansionBudget(zr); err != nil {
		return err
	}

	if opts.List {
		for _, f := range zr.File {
			switch {
			case strings.HasPrefix(f.Name, LibDir+"/"):
				fmt.Println(f.Name)
			case strings.HasPrefix(f.Name, DocsDir+"/"):
				fmt.Println(f.Name)
			}
		}
		return nil
	}

	destDir := opts.DestDir
	if destDir == "" {
		destDir = "."
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	// Anchor containment at the resolved destination: the path the user
	// named may itself contain symlinks (on macOS even the temp dir does,
	// /var -> /private/var), and those are the user's choice. Everything
	// strictly below the resolved root is defended.
	destRoot, err := filepath.EvalSymlinks(destDir)
	if err != nil {
		return err
	}

	for _, f := range zr.File {
		// Only extract lib/ and docs/ contents, stripping the prefix so multiple
		// packages can be unpacked into the same destination directory.
		name := f.Name
		var prefix string
		switch {
		case strings.HasPrefix(name, LibDir+"/"):
			prefix = LibDir + "/"
		case strings.HasPrefix(name, DocsDir+"/"):
			prefix = DocsDir + "/"
		default:
			continue
		}
		f.Name = name[len(prefix):]
		if f.Name == "" {
			continue
		}
		if err := extractFile(f, filepath.Join(destRoot, prefix[:len(prefix)-1]), opts.Force); err != nil {
			return err
		}
	}
	return nil
}

// checkExpansionBudget refuses archives whose declared contents exceed the
// extraction budgets, using the zip's own header sizes so a bomb fails
// before a single entry is written.
func checkExpansionBudget(zr *zip.Reader) error {
	if len(zr.File) > maxUnpackEntries {
		return fmt.Errorf("package holds %d entries (max %d)", len(zr.File), maxUnpackEntries)
	}
	var total uint64
	for _, f := range zr.File {
		if f.UncompressedSize64 > maxUnpackEntryBytes {
			return fmt.Errorf("entry %s declares %d uncompressed bytes (max %d)", f.Name, f.UncompressedSize64, maxUnpackEntryBytes)
		}
		total += f.UncompressedSize64
	}
	if total > maxUnpackTotalBytes {
		return fmt.Errorf("package declares %d uncompressed bytes in total (max %d)", total, maxUnpackTotalBytes)
	}
	return nil
}

// UnpackRemove removes the files that would be extracted from a package.
func UnpackRemove(src string, insecure bool, destDir string) error {
	data, err := Fetch(src, insecure)
	if err != nil {
		return err
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ErrInvalidPackage
	}

	if destDir == "" {
		destDir = "."
	}
	destRoot, rootErr := filepath.EvalSymlinks(destDir)
	if rootErr != nil {
		return nil // nothing there, so nothing to remove
	}

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := f.Name
		var prefix string
		switch {
		case strings.HasPrefix(name, LibDir+"/"):
			prefix = LibDir + "/"
		case strings.HasPrefix(name, DocsDir+"/"):
			prefix = DocsDir + "/"
		default:
			continue
		}
		rel := filepath.FromSlash(name[len(prefix):])
		if rel == "" || strings.Contains(rel, "..") {
			continue
		}
		root := filepath.Join(destRoot, prefix[:len(prefix)-1])
		if err := ensureNoSymlinkPrefix(root, rel); err != nil {
			return err
		}
		dst := filepath.Join(root, rel)
		if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// Extraction budgets: a zip well under the download size cap can expand to
// anything at all. Entries past these bounds are refused rather than filling
// the disk.
const (
	maxUnpackEntries    = 10000
	maxUnpackEntryBytes = 256 << 20 // 256 MiB per entry
	maxUnpackTotalBytes = 1 << 30   // 1 GiB across the archive
)

// extractFile writes one archive entry under destDir. Lexical traversal is
// rejected, and so is a pre-existing symlink anywhere along the destination
// path: MkdirAll and os.Create both follow symlinks, so a planted link (or a
// link written by an earlier entry of a hostile archive) would redirect the
// extraction outside destDir.
func extractFile(f *zip.File, destDir string, force bool) error {
	// Prevent path traversal
	rel := filepath.FromSlash(f.Name)
	if strings.Contains(rel, "..") {
		return fmt.Errorf("invalid path in package: %s", f.Name)
	}

	dst := filepath.Join(destDir, rel)

	if err := ensureNoSymlinkPrefix(destDir, rel); err != nil {
		return err
	}

	if f.FileInfo().IsDir() {
		return mkdirAllNoSymlinks(dst)
	}

	if !force {
		if info, err := os.Lstat(dst); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("refusing to follow symlink: %s", dst)
			}
			return fmt.Errorf("file already exists (use -f to overwrite): %s", dst)
		}
	}

	if err := mkdirAllNoSymlinks(filepath.Dir(dst)); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	_, err = io.Copy(out, io.LimitReader(rc, maxUnpackEntryBytes+1))
	if err != nil {
		return err
	}
	if f.UncompressedSize64 > maxUnpackEntryBytes {
		return fmt.Errorf("entry %s expands past %d bytes", f.Name, maxUnpackEntryBytes)
	}
	return nil
}

// ensureNoSymlinkPrefix checks every directory component of rel (relative to
// destDir) that already exists on disk, refusing symlinks so extraction
// cannot be redirected through a pre-existing link.
func ensureNoSymlinkPrefix(destDir, rel string) error {
	// destDir arrives already resolved; only components strictly below it
	// are defended (the user's own path choices are not ours to police).
	current := destDir
	parts := strings.Split(filepath.FromSlash(rel), string(os.PathSeparator))
	for i := 0; i < len(parts)-1; i++ {
		current = filepath.Join(current, parts[i])
		if info, err := os.Lstat(current); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("refusing to extract through symlink: %s", current)
			}
			if !info.IsDir() {
				return fmt.Errorf("destination component is not a directory: %s", current)
			}
		}
	}
	return nil
}

// mkdirAllNoSymlinks creates dir and its parents one component at a time,
// refusing to traverse a symlink, which os.MkdirAll would happily do.
// mkdirAllNoSymlinks creates dir and its parents one component at a time,
// refusing to traverse a symlink, which os.MkdirAll would happily do. The
// path is built from the resolved destination root, so every ancestor is
// real; a symlink appearing mid-walk is someone racing the extraction.
func mkdirAllNoSymlinks(dir string) error {
	parent := filepath.Dir(dir)
	if parent != dir {
		if err := mkdirAllNoSymlinks(parent); err != nil {
			return err
		}
	}
	if info, err := os.Lstat(dir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to extract through symlink: %s", dir)
		}
		if info.IsDir() {
			return nil
		}
		return fmt.Errorf("destination exists and is not a directory: %s", dir)
	}
	return os.Mkdir(dir, 0755)
}
