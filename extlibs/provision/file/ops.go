package file

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/provision/become"
	"github.com/paularlott/scriptling/object"
)

// The library's operations run here, or as root in the become helper: the
// same code either way, confined to the allowed paths it is given.
func init() {
	become.Handle("file.ensure", opEnsure)
	become.Handle("file.absent", opAbsent)
	become.Handle("file.ensure_directory", opEnsureDirectory)
	become.Handle("file.absent_directory", opAbsentDirectory)
	become.Handle("file.ensure_block", opEnsureBlock)
	become.Handle("file.absent_block", opAbsentBlock)
}

type ensureArgs struct {
	Path       string
	Content    string
	Mode       int
	ModeGiven  bool
	CreateOnly bool
	Owner      become.Owner
}

type pathArgs struct {
	Path string
}

type dirArgs struct {
	Path      string
	Mode      int
	ModeGiven bool
	Owner     become.Owner
}

type blockArgs struct {
	Path        string
	Content     string
	ID          string
	Comment     string
	Position    string
	InsertAfter string
	Mode        int
	CreateOnly  bool
	Owner       become.Owner
}

// settle gives an existing path the mode (when it was asked for) and the
// owner it should have, and reports whether either changed.
func settle(paths fssecurity.Config, path string, info os.FileInfo, mode int, modeGiven bool, owner become.Owner) (bool, error) {
	changed := false
	if modeGiven && info.Mode().Perm() != os.FileMode(mode).Perm() {
		if err := os.Chmod(path, os.FileMode(mode)); err != nil {
			return false, fmt.Errorf("failed to set mode on %s: %s", path, err.Error())
		}
		changed = true
	}
	owned, err := owner.Apply(paths, path)
	return changed || owned, err
}

func opEnsure(_ context.Context, paths fssecurity.Config, a ensureArgs) (string, error) {
	if err := paths.CheckPath(a.Path); err != nil {
		return "", err
	}
	if err := a.Owner.Check(); err != nil {
		return "", err
	}
	info, err := os.Stat(a.Path)
	fileExisted := err == nil
	if fileExisted && a.CreateOnly {
		return StatusUnchanged, nil
	}
	if fileExisted && object.FileEquals(a.Path, []byte(a.Content)) {
		changed, err := settle(paths, a.Path, info, a.Mode, a.ModeGiven, a.Owner)
		if err != nil {
			return "", err
		}
		if changed {
			return StatusUpdated, nil
		}
		return StatusUnchanged, nil
	}

	// RawDir, not filepath.Dir: the cleaned parent of an unclean path can
	// resolve somewhere the checked path does not walk through.
	dir := fssecurity.RawDir(a.Path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, defaultDirMode); err != nil {
			return "", fmt.Errorf("failed to create directory %s: %s", dir, err.Error())
		}
	}
	if err := os.WriteFile(a.Path, []byte(a.Content), os.FileMode(a.Mode)); err != nil {
		return "", fmt.Errorf("failed to write %s: %s", a.Path, err.Error())
	}
	if err := os.Chmod(a.Path, os.FileMode(a.Mode)); err != nil {
		return "", fmt.Errorf("failed to set mode on %s: %s", a.Path, err.Error())
	}
	if _, err := a.Owner.Apply(paths, a.Path); err != nil {
		return "", err
	}
	if fileExisted {
		return StatusUpdated, nil
	}
	return StatusCreated, nil
}

func opAbsent(_ context.Context, paths fssecurity.Config, a pathArgs) (string, error) {
	if err := paths.CheckPath(a.Path); err != nil {
		return "", err
	}
	if _, err := os.Stat(a.Path); os.IsNotExist(err) {
		return StatusAbsent, nil
	}
	if err := os.Remove(a.Path); err != nil {
		return "", fmt.Errorf("failed to remove %s: %s", a.Path, err.Error())
	}
	return StatusRemoved, nil
}

func opEnsureDirectory(_ context.Context, paths fssecurity.Config, a dirArgs) (string, error) {
	if err := paths.CheckPath(a.Path); err != nil {
		return "", err
	}
	if err := a.Owner.Check(); err != nil {
		return "", err
	}
	info, err := os.Stat(a.Path)
	if err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("%s exists but is not a directory", a.Path)
		}
		changed, err := settle(paths, a.Path, info, a.Mode, a.ModeGiven, a.Owner)
		if err != nil {
			return "", err
		}
		if changed {
			return StatusUpdated, nil
		}
		return StatusExists, nil
	}
	// Parents get the default mode, as with mkdir -p -m: a mode meant for
	// the directory could otherwise lock its owner out of the way to it.
	if parent := fssecurity.RawDir(a.Path); parent != "" && parent != "." {
		if err := os.MkdirAll(parent, defaultDirMode); err != nil {
			return "", fmt.Errorf("failed to create %s: %s", parent, err.Error())
		}
	}
	if err := os.Mkdir(a.Path, os.FileMode(a.Mode)); err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("failed to create %s: %s", a.Path, err.Error())
	}
	if a.ModeGiven { // exactly the mode asked for, whatever the umask
		if err := os.Chmod(a.Path, os.FileMode(a.Mode)); err != nil {
			return "", fmt.Errorf("failed to set mode on %s: %s", a.Path, err.Error())
		}
	}
	if _, err := a.Owner.Apply(paths, a.Path); err != nil {
		return "", err
	}
	return StatusCreated, nil
}

func opAbsentDirectory(_ context.Context, paths fssecurity.Config, a pathArgs) (string, error) {
	if err := paths.CheckPath(a.Path); err != nil {
		return "", err
	}
	info, err := os.Stat(a.Path)
	if os.IsNotExist(err) {
		return StatusAbsent, nil
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", a.Path)
	}
	if err := os.Remove(a.Path); err != nil {
		return "", fmt.Errorf("failed to remove %s: %s", a.Path, err.Error())
	}
	return StatusRemoved, nil
}

func opEnsureBlock(_ context.Context, paths fssecurity.Config, a blockArgs) (string, error) {
	if err := paths.CheckPath(a.Path); err != nil {
		return "", err
	}
	if err := a.Owner.Check(); err != nil {
		return "", err
	}
	beginLine, endLine := blockMarkerLines(a.Comment, a.ID)
	contentLines := splitContentLines(a.Content)

	lines, trailingNL, existed, err := readFileLines(a.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %s", a.Path, err.Error())
	}

	// owned settles the owner of a file whose block is as it should be.
	owned := func() (string, error) {
		changed, err := a.Owner.Apply(paths, a.Path)
		if err != nil {
			return "", err
		}
		if changed {
			return StatusUpdated, nil
		}
		return StatusUnchanged, nil
	}

	state, beginIdx, endIdx := findBlock(lines, beginLine, endLine)
	switch state {
	case blockOrphan:
		return "", fmt.Errorf("orphaned markers found in %s for id %q", a.Path, a.ID)
	case blockValid:
		if a.CreateOnly {
			return StatusUnchanged, nil
		}
		if linesEqual(lines[beginIdx+1:endIdx], contentLines) {
			return owned()
		}
		newLines := make([]string, 0, len(lines)-(endIdx-beginIdx-1)+len(contentLines))
		newLines = append(newLines, lines[:beginIdx+1]...)
		newLines = append(newLines, contentLines...)
		newLines = append(newLines, lines[endIdx:]...)
		if err := writeFileLines(a.Path, newLines, trailingNL, a.Mode); err != nil {
			return "", fmt.Errorf("failed to write %s: %s", a.Path, err.Error())
		}
		if _, err := a.Owner.Apply(paths, a.Path); err != nil {
			return "", err
		}
		return StatusUpdated, nil
	case blockAbsent:
		blockLines := make([]string, 0, 2+len(contentLines))
		blockLines = append(blockLines, beginLine)
		blockLines = append(blockLines, contentLines...)
		blockLines = append(blockLines, endLine)

		newLines, err := insertBlock(lines, blockLines, a.Position, a.InsertAfter, a.Path)
		if err != nil {
			return "", err
		}
		writeTrailing := trailingNL
		if !existed {
			writeTrailing = true
		}
		if err := writeFileLines(a.Path, newLines, writeTrailing, a.Mode); err != nil {
			return "", fmt.Errorf("failed to write %s: %s", a.Path, err.Error())
		}
		if _, err := a.Owner.Apply(paths, a.Path); err != nil {
			return "", err
		}
		return StatusCreated, nil
	}
	return StatusUnchanged, nil
}

func opAbsentBlock(_ context.Context, paths fssecurity.Config, a blockArgs) (string, error) {
	if err := paths.CheckPath(a.Path); err != nil {
		return "", err
	}
	beginLine, endLine := blockMarkerLines(a.Comment, a.ID)
	lines, trailingNL, existed, err := readFileLines(a.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %s", a.Path, err.Error())
	}
	if !existed {
		return StatusUnchanged, nil
	}
	mode := defaultFileMode
	if info, statErr := os.Stat(a.Path); statErr == nil {
		mode = int(info.Mode().Perm())
	}
	state, beginIdx, endIdx := findBlock(lines, beginLine, endLine)
	switch state {
	case blockOrphan:
		return "", fmt.Errorf("orphaned markers found in %s for id %q", a.Path, a.ID)
	case blockValid:
		newLines := make([]string, 0, len(lines)-(endIdx-beginIdx+1))
		newLines = append(newLines, lines[:beginIdx]...)
		newLines = append(newLines, lines[endIdx+1:]...)
		if err := writeFileLines(a.Path, newLines, trailingNL, mode); err != nil {
			return "", fmt.Errorf("failed to write %s: %s", a.Path, err.Error())
		}
		return StatusRemoved, nil
	}
	return StatusUnchanged, nil
}

// readFileLines reads a file and splits it into lines. existed is false when the
// file does not exist; trailingNL reports whether the original ended with "\n".
func readFileLines(path string) (lines []string, trailingNL, existed bool, err error) {
	raw, readErr := object.ReadFileLimitedErr(path)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return nil, true, false, nil
		}
		return nil, true, false, readErr
	}
	existed = true
	s := string(raw)
	trailingNL = strings.HasSuffix(s, "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil, trailingNL, existed, nil
	}
	return strings.Split(s, "\n"), trailingNL, existed, nil
}

// writeFileLines joins lines and writes them, creating parent directories and
// applying mode. An empty lines slice writes an empty file.
func writeFileLines(path string, lines []string, trailingNL bool, mode int) error {
	// RawDir, not filepath.Dir: only directories that checking path walked
	// may be created.
	dir := fssecurity.RawDir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, defaultDirMode); err != nil {
			return err
		}
	}
	s := ""
	if len(lines) > 0 {
		s = strings.Join(lines, "\n")
		if trailingNL {
			s += "\n"
		}
	}
	if err := os.WriteFile(path, []byte(s), os.FileMode(mode)); err != nil {
		return err
	}
	return os.Chmod(path, os.FileMode(mode))
}
