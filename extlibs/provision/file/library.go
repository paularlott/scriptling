package file

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/paularlott/scriptling/object"
)

const (
	LibraryName = "scriptling.provision.file"
	LibraryDesc = "File provisioning utilities for creating and updating files with correct permissions"
)

var (
	library     *object.Library
	libraryOnce sync.Once
)

const (
	defaultFileMode = 0o644
	defaultDirMode  = 0o755

	defaultBlockID = "managed"
	defaultComment = "#"
	positionEnd    = "end"
	positionStart  = "start"

	StatusCreated   = "created"
	StatusUpdated   = "updated"
	StatusUnchanged = "unchanged"
	StatusRemoved   = "removed"
	StatusAbsent    = "absent"
	StatusExists    = "exists"
)

func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	libraryOnce.Do(func() {
		library = buildLibrary()
	})
	registrar.RegisterLibrary(library)
}

func buildLibrary() *object.Library {
	return object.NewLibrary(LibraryName, map[string]*object.Builtin{
		"ensure": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 2 {
					return &object.Error{Message: fmt.Sprintf("ensure expected 2 arguments, got %d", len(args))}
				}

				path, coerceErr := args[0].CoerceString()
				if coerceErr != nil {
					return &object.Error{Message: "ensure: path must be a string"}
				}

				content, coerceErr := args[1].CoerceString()
				if coerceErr != nil {
					return &object.Error{Message: "ensure: content must be a string"}
				}

			mode := int(kwargs.MustGetInt("mode", defaultFileMode))
			createOnly := kwargs.MustGetBool("create_only", false)

			path = expandPath(path)

			existing, err := os.ReadFile(path)
			if err == nil && bytes.Equal(existing, []byte(content)) {
				return object.NewString(StatusUnchanged)
			}

			if err == nil && createOnly {
				return object.NewString(StatusUnchanged)
			}

			fileExisted := err == nil

				dir := filepath.Dir(path)
				if dir != "" && dir != "." {
					if err := os.MkdirAll(dir, defaultDirMode); err != nil {
						return &object.Error{Message: fmt.Sprintf("ensure: failed to create directory %s: %s", dir, err.Error())}
					}
				}

				if err := os.WriteFile(path, []byte(content), os.FileMode(mode)); err != nil {
					return &object.Error{Message: fmt.Sprintf("ensure: failed to write %s: %s", path, err.Error())}
				}

				if err := os.Chmod(path, os.FileMode(mode)); err != nil {
					return &object.Error{Message: fmt.Sprintf("ensure: failed to set mode on %s: %s", path, err.Error())}
				}

			if fileExisted {
				return object.NewString(StatusUpdated)
			}
			return object.NewString(StatusCreated)
			},
			HelpText: `ensure(path, content, mode=0o644, create_only=False) - Ensure a file exists with the given content

Creates parent directories if needed. If the file already exists with the
same content, it is left unchanged. Otherwise the file is written with the
specified mode.

When create_only is True, an existing file is never modified: the call
returns "unchanged" without writing, even if the content differs. New files
are still written normally.

Parameters:
  path (str): Path to the file (supports ~ expansion)
  content (str): File contents
  mode (int): File permission mode (default 0o644)
  create_only (bool): If True, never modify an existing file (default False)

Returns:
  str: "created" if the file was newly written,
       "updated" if the file existed but content differed,
       "unchanged" if the file existed with identical content,
       or if the file existed and create_only is True

Example:
  import scriptling.provision.file as file
  status = file.ensure("~/.gitconfig", "[user]\nname = Jane\n", mode=0o600)
  if status == file.CREATED:
      print("File created")
  elif status == file.UPDATED:
      print("File updated")`,
		},
		"absent": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: fmt.Sprintf("absent expected 1 argument, got %d", len(args))}
				}

				path, coerceErr := args[0].CoerceString()
				if coerceErr != nil {
					return &object.Error{Message: "absent: path must be a string"}
				}

				path = expandPath(path)

				if _, err := os.Stat(path); os.IsNotExist(err) {
					return object.NewString(StatusAbsent)
				}

				if err := os.Remove(path); err != nil {
					return &object.Error{Message: fmt.Sprintf("absent: failed to remove %s: %s", path, err.Error())}
				}

				return object.NewString(StatusRemoved)
			},
			HelpText: `absent(path) - Ensure a file does not exist

Removes the file if it exists. Does nothing if the file is already absent.

Parameters:
  path (str): Path to the file (supports ~ expansion)

Returns:
  str: file.REMOVED if the file was deleted,
       file.ABSENT if the file did not exist

Example:
  import scriptling.provision.file as file
  status = file.absent("~/.old_config")
  if status == file.REMOVED:
      print("File removed")`,
		},
		"ensure_directory": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: fmt.Sprintf("ensure_directory expected 1 argument, got %d", len(args))}
				}

				path, coerceErr := args[0].CoerceString()
				if coerceErr != nil {
					return &object.Error{Message: "ensure_directory: path must be a string"}
				}

				mode := int(kwargs.MustGetInt("mode", defaultDirMode))

				path = expandPath(path)

				info, err := os.Stat(path)
				if err == nil {
					if !info.IsDir() {
						return &object.Error{Message: fmt.Sprintf("ensure_directory: %s exists but is not a directory", path)}
					}
					return object.NewString(StatusExists)
				}

				if err := os.MkdirAll(path, os.FileMode(mode)); err != nil {
					return &object.Error{Message: fmt.Sprintf("ensure_directory: failed to create %s: %s", path, err.Error())}
				}

				return object.NewString(StatusCreated)
			},
			HelpText: `ensure_directory(path, mode=0o755) - Ensure a directory exists

Creates the directory and all parent directories if needed.

Parameters:
  path (str): Path to the directory (supports ~ expansion)
  mode (int): Directory permission mode (default 0o755)

Returns:
  str: file.CREATED if the directory was newly created,
       file.EXISTS if the directory already existed

Example:
  import scriptling.provision.file as file
  status = file.ensure_directory("~/.config/myapp", mode=0o700)
  if status == file.CREATED:
      print("Directory created")`,
		},
		"absent_directory": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: fmt.Sprintf("absent_directory expected 1 argument, got %d", len(args))}
				}

				path, coerceErr := args[0].CoerceString()
				if coerceErr != nil {
					return &object.Error{Message: "absent_directory: path must be a string"}
				}

				path = expandPath(path)

				info, err := os.Stat(path)
				if os.IsNotExist(err) {
					return object.NewString(StatusAbsent)
				}
				if err != nil {
					return &object.Error{Message: fmt.Sprintf("absent_directory: %s", err.Error())}
				}
				if !info.IsDir() {
					return &object.Error{Message: fmt.Sprintf("absent_directory: %s is not a directory", path)}
				}

				if err := os.Remove(path); err != nil {
					return &object.Error{Message: fmt.Sprintf("absent_directory: failed to remove %s: %s", path, err.Error())}
				}

				return object.NewString(StatusRemoved)
			},
			HelpText: `absent_directory(path) - Ensure an empty directory does not exist

Removes the directory if it exists and is empty. Returns an error if the
directory is not empty.

Parameters:
  path (str): Path to the directory (supports ~ expansion)

Returns:
  str: file.REMOVED if the directory was deleted,
       file.ABSENT if the directory did not exist

Example:
  import scriptling.provision.file as file
  status = file.absent_directory("~/old/empty/dir")
  if status == file.REMOVED:
      print("Directory removed")`,
		},
		"ensure_block": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 2 {
					return &object.Error{Message: fmt.Sprintf("ensure_block expected 2 arguments, got %d", len(args))}
				}

				path, coerceErr := args[0].CoerceString()
				if coerceErr != nil {
					return &object.Error{Message: "ensure_block: path must be a string"}
				}

				content, coerceErr := args[1].CoerceString()
				if coerceErr != nil {
					return &object.Error{Message: "ensure_block: content must be a string"}
				}

				id := kwargs.MustGetString("id", defaultBlockID)
				comment := kwargs.MustGetString("comment", defaultComment)
				position := kwargs.MustGetString("position", positionEnd)
				insertAfter := kwargs.MustGetString("insert_after", "")
				mode := int(kwargs.MustGetInt("mode", defaultFileMode))
				createOnly := kwargs.MustGetBool("create_only", false)

				if err := validateBlockParams(comment, id); err != nil {
					return &object.Error{Message: "ensure_block: " + err.Error()}
				}
				if insertAfter == "" && position != positionStart && position != positionEnd {
					return &object.Error{Message: fmt.Sprintf("ensure_block: position must be %q or %q, got %q", positionStart, positionEnd, position)}
				}

				beginLine, endLine := blockMarkerLines(comment, id)

				contentLines := splitContentLines(content)
				for _, l := range contentLines {
					if lineEqualsMarker(l, endLine) {
						return &object.Error{Message: "ensure_block: content contains the end marker"}
					}
					if lineEqualsMarker(l, beginLine) {
						return &object.Error{Message: "ensure_block: content contains the begin marker"}
					}
				}

				path = expandPath(path)

				lines, trailingNL, existed, err := readFileLines(path)
				if err != nil {
					return &object.Error{Message: fmt.Sprintf("ensure_block: failed to read %s: %s", path, err.Error())}
				}

				state, beginIdx, endIdx := findBlock(lines, beginLine, endLine)
				switch state {
				case blockOrphan:
					return &object.Error{Message: fmt.Sprintf("ensure_block: orphaned markers found in %s for id %q", path, id)}
				case blockValid:
					if createOnly {
						return object.NewString(StatusUnchanged)
					}
					existingInner := lines[beginIdx+1 : endIdx]
					if linesEqual(existingInner, contentLines) {
						return object.NewString(StatusUnchanged)
					}
					newLines := make([]string, 0, len(lines)-(endIdx-beginIdx-1)+len(contentLines))
					newLines = append(newLines, lines[:beginIdx+1]...)
					newLines = append(newLines, contentLines...)
					newLines = append(newLines, lines[endIdx:]...)
					if err := writeFileLines(path, newLines, trailingNL, mode); err != nil {
						return &object.Error{Message: fmt.Sprintf("ensure_block: failed to write %s: %s", path, err.Error())}
					}
					return object.NewString(StatusUpdated)
				case blockAbsent:
					blockLines := make([]string, 0, 2+len(contentLines))
					blockLines = append(blockLines, beginLine)
					blockLines = append(blockLines, contentLines...)
					blockLines = append(blockLines, endLine)

					newLines, insErr := insertBlock(lines, blockLines, position, insertAfter, path)
					if insErr != nil {
						return &object.Error{Message: "ensure_block: " + insErr.Error()}
					}

					writeTrailing := trailingNL
					if !existed {
						writeTrailing = true
					}
					if err := writeFileLines(path, newLines, writeTrailing, mode); err != nil {
						return &object.Error{Message: fmt.Sprintf("ensure_block: failed to write %s: %s", path, err.Error())}
					}
					return object.NewString(StatusCreated)
				}
				return object.NewString(StatusUnchanged)
			},
			HelpText: `ensure_block(path, content, id="managed", comment="#", position="end", insert_after="", mode=0o644, create_only=False) - Maintain a managed block within a file

Wraps the given content in distinctive markers and replaces only the text
between them on each run. Everything outside the markers is left untouched.
If the markers are not present, the block is inserted at the chosen position.

When position is "end" (default) the block is appended; "start" prepends it.
If insert_after is a non-empty string, the block is inserted immediately after
the first line containing that substring (insert_after takes precedence over
position). If the anchor is not found, an error is returned.

A unique id allows multiple independent blocks to coexist in the same file.
The markers look like:

  # >>> scriptling managed: myid >>>
  <content>
  # <<< scriptling managed: myid <<<

Parameters:
  path (str): Path to the file (supports ~ expansion)
  content (str): Block contents to maintain between the markers
  id (str): Block identifier embedded in the markers (default "managed")
  comment (str): Comment prefix used to build markers (default "#")
  position (str): Where to insert a new block: "end" (default) or "start"
  insert_after (str): Substring anchor; new block inserted after first match
  mode (int): File permission mode used when creating the file (default 0o644)
  create_only (bool): If True, never modify an existing block (default False)

Returns:
  str: "created" if the block was newly inserted,
       "updated" if the block existed but content differed,
       "unchanged" if the block existed with identical content,
       or if the block existed and create_only is True

Example:
  import scriptling.provision.file as file
  status = file.ensure_block("~/.bashrc", "export EDITOR=vim\n", id="editor")
  status = file.ensure_block("/etc/hosts", "127.0.0.1 myapp\n", insert_after="localhost")`,
		},
		"absent_block": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: fmt.Sprintf("absent_block expected 1 argument, got %d", len(args))}
				}

				path, coerceErr := args[0].CoerceString()
				if coerceErr != nil {
					return &object.Error{Message: "absent_block: path must be a string"}
				}

				id := kwargs.MustGetString("id", defaultBlockID)
				comment := kwargs.MustGetString("comment", defaultComment)

				if err := validateBlockParams(comment, id); err != nil {
					return &object.Error{Message: "absent_block: " + err.Error()}
				}

				beginLine, endLine := blockMarkerLines(comment, id)

				path = expandPath(path)

				lines, trailingNL, existed, err := readFileLines(path)
				if err != nil {
					return &object.Error{Message: fmt.Sprintf("absent_block: failed to read %s: %s", path, err.Error())}
				}
				if !existed {
					return object.NewString(StatusUnchanged)
				}

				mode := defaultFileMode
				if info, statErr := os.Stat(path); statErr == nil {
					mode = int(info.Mode().Perm())
				}

				state, beginIdx, endIdx := findBlock(lines, beginLine, endLine)
				switch state {
				case blockOrphan:
					return &object.Error{Message: fmt.Sprintf("absent_block: orphaned markers found in %s for id %q", path, id)}
				case blockAbsent:
					return object.NewString(StatusUnchanged)
				case blockValid:
					newLines := make([]string, 0, len(lines)-(endIdx-beginIdx+1))
					newLines = append(newLines, lines[:beginIdx]...)
					newLines = append(newLines, lines[endIdx+1:]...)
					if err := writeFileLines(path, newLines, trailingNL, mode); err != nil {
						return &object.Error{Message: fmt.Sprintf("absent_block: failed to write %s: %s", path, err.Error())}
					}
					return object.NewString(StatusRemoved)
				}
				return object.NewString(StatusUnchanged)
			},
			HelpText: `absent_block(path, id="managed", comment="#") - Remove a managed block

Removes the marker-delimited block (markers and all content between them) for
the given id. Everything else in the file is left untouched. If no such block
exists, nothing happens.

Parameters:
  path (str): Path to the file (supports ~ expansion)
  id (str): Block identifier embedded in the markers (default "managed")
  comment (str): Comment prefix used to build markers (default "#")

Returns:
  str: file.REMOVED if the block was deleted,
       file.UNCHANGED if the block was not present

Example:
  import scriptling.provision.file as file
  status = file.absent_block("~/.bashrc", id="editor")
  if status == file.REMOVED:
      print("Block removed")`,
		},
	}, map[string]object.Object{
		"CREATED":   object.NewString(StatusCreated),
		"UPDATED":   object.NewString(StatusUpdated),
		"UNCHANGED": object.NewString(StatusUnchanged),
		"REMOVED":   object.NewString(StatusRemoved),
		"ABSENT":    object.NewString(StatusAbsent),
		"EXISTS":    object.NewString(StatusExists),
	}, LibraryDesc)
}

func expandPath(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			if len(path) == 1 || path[1] == '/' {
				return filepath.Join(home, path[1:])
			}
		}
	}
	return path
}

// blockMarkerLines builds the distinctive begin/end marker lines for a managed block.
func blockMarkerLines(comment, id string) (begin, end string) {
	begin = comment + " >>> scriptling managed: " + id + " >>>"
	end = comment + " <<< scriptling managed: " + id + " <<<"
	return
}

// validateBlockParams checks that comment and id are usable in marker lines.
func validateBlockParams(comment, id string) error {
	if comment == "" {
		return fmt.Errorf("comment must not be empty")
	}
	if strings.ContainsAny(comment, "\n\r") {
		return fmt.Errorf("comment must not contain newlines")
	}
	if id == "" {
		return fmt.Errorf("id must not be empty")
	}
	if strings.ContainsAny(id, "\n\r") {
		return fmt.Errorf("id must not contain newlines")
	}
	return nil
}

// lineEqualsMarker reports whether a file line is exactly the marker, ignoring
// a trailing carriage return (so CRLF files parse correctly).
func lineEqualsMarker(line, marker string) bool {
	return strings.TrimRight(line, "\r") == marker
}

// splitContentLines normalises content into the lines stored between markers.
// A single trailing newline (if present) is dropped so "foo\n" and "foo" are
// treated identically; empty content yields no lines.
func splitContentLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func linesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type blockState int

const (
	blockAbsent blockState = iota
	blockValid
	blockOrphan
)

// findBlock locates a managed block in lines. Returns blockOrphan unless there
// is exactly one begin marker followed by exactly one end marker.
func findBlock(lines []string, beginLine, endLine string) (blockState, int, int) {
	var begins, ends []int
	for i, l := range lines {
		if lineEqualsMarker(l, beginLine) {
			begins = append(begins, i)
		} else if lineEqualsMarker(l, endLine) {
			ends = append(ends, i)
		}
	}
	if len(begins) == 0 && len(ends) == 0 {
		return blockAbsent, -1, -1
	}
	if len(begins) == 1 && len(ends) == 1 && begins[0] < ends[0] {
		return blockValid, begins[0], ends[0]
	}
	return blockOrphan, -1, -1
}

// insertBlock places blockLines into lines according to position/insert_after.
// insert_after (non-empty) takes precedence and inserts after the first line
// containing the substring; an unmatched anchor is an error.
func insertBlock(lines []string, blockLines []string, position, insertAfter, path string) ([]string, error) {
	if insertAfter != "" {
		idx := -1
		for i, l := range lines {
			if strings.Contains(strings.TrimRight(l, "\r"), insertAfter) {
				idx = i
				break
			}
		}
		if idx == -1 {
			return nil, fmt.Errorf("insert_after anchor %q not found in %s", insertAfter, path)
		}
		newLines := make([]string, 0, len(lines)+len(blockLines))
		newLines = append(newLines, lines[:idx+1]...)
		newLines = append(newLines, blockLines...)
		newLines = append(newLines, lines[idx+1:]...)
		return newLines, nil
	}
	if position == positionStart {
		newLines := make([]string, 0, len(lines)+len(blockLines))
		newLines = append(newLines, blockLines...)
		newLines = append(newLines, lines...)
		return newLines, nil
	}
	// default: end (append)
	newLines := make([]string, 0, len(lines)+len(blockLines))
	newLines = append(newLines, lines...)
	newLines = append(newLines, blockLines...)
	return newLines, nil
}

// readFileLines reads a file and splits it into lines. existed is false when the
// file does not exist; trailingNL reports whether the original ended with "\n".
func readFileLines(path string) (lines []string, trailingNL, existed bool, err error) {
	raw, readErr := os.ReadFile(path)
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
	dir := filepath.Dir(path)
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
