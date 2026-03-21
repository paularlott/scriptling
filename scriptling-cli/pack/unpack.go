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
		if err := extractFile(f, filepath.Join(destDir, prefix[:len(prefix)-1]), opts.Force); err != nil {
			return err
		}
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
		dst := filepath.Join(destDir, prefix[:len(prefix)-1], rel)
		if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func extractFile(f *zip.File, destDir string, force bool) error {
	// Prevent path traversal
	rel := filepath.FromSlash(f.Name)
	if strings.Contains(rel, "..") {
		return fmt.Errorf("invalid path in package: %s", f.Name)
	}

	dst := filepath.Join(destDir, rel)

	if f.FileInfo().IsDir() {
		return os.MkdirAll(dst, 0755)
	}

	if !force {
		if _, err := os.Stat(dst); err == nil {
			return fmt.Errorf("file already exists (use -f to overwrite): %s", dst)
		}
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	_, err = io.Copy(out, rc)
	return err
}
