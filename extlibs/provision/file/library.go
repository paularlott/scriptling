package file

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/provision/become"
	"github.com/paularlott/scriptling/object"
)

const (
	LibraryName = "scriptling.provision.file"
	LibraryDesc = "File provisioning utilities for creating and updating files with correct permissions and owners"
)

// libraryInstance holds the filesystem policy this library was registered
// with, and what runs its operations as root (nil: running as root is off).
type libraryInstance struct {
	paths  fssecurity.Config
	become *become.Become
}

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

// Option configures the library.
type Option func(*libraryInstance)

// WithBecome lets scripts run the library's operations as root with
// become=True, through b. Without it, become=True fails unless the process is
// already root.
func WithBecome(b *become.Become) Option {
	return func(i *libraryInstance) { i.become = b }
}

func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	RegisterConfigured(registrar, fssecurity.Config{})
}

// RegisterConfigured registers the library with a filesystem allow-path policy.
// It threads the same fssecurity.Config used by os/pathlib/shutil/zipfile so
// that scriptling.provision.file obeys --allowed-paths. Pass fssecurity.Config{}
// for no restriction.
//
// A fresh library is built on every call, so each registrar gets exactly the
// policy it passed — safe for an embedder that registers this library
// against interpreters with different per-request/per-tenant configs, not
// just a single process-wide policy set once at startup.
func RegisterConfigured(registrar interface{ RegisterLibrary(*object.Library) }, paths fssecurity.Config, opts ...Option) {
	if paths.AllowedPaths != nil {
		normalized := make([]string, 0, len(paths.AllowedPaths))
		for _, p := range paths.AllowedPaths {
			abs, err := filepath.Abs(p)
			if err != nil {
				continue
			}
			normalized = append(normalized, filepath.Clean(abs))
		}
		paths.AllowedPaths = normalized
	}
	inst := &libraryInstance{paths: paths}
	for _, o := range opts {
		o(inst)
	}
	registrar.RegisterLibrary(buildLibrary(inst))
}

// do runs the library's operation name, as root when asRoot is set.
func do[A any](ctx context.Context, inst *libraryInstance, asRoot bool, name string, a A) object.Object {
	var status string
	var err error
	object.RunBlocking(ctx, func() {
		status, err = become.Do[A, string](ctx, inst.become, asRoot, inst.paths, "file."+name, a)
	})
	if err != nil {
		return opError(name, err)
	}
	return object.NewString(status)
}

// opError is the script error for a failed operation. A path outside the
// allowed directories, or root where it is not allowed, is a PermissionError
// the script cannot catch, as in the os and pathlib libraries.
func opError(name string, err error) object.Object {
	if stderrors.Is(err, fssecurity.ErrPathNotAllowed) || stderrors.Is(err, become.ErrNotEnabled) {
		return errors.NewPermissionError("%s: %s", name, err.Error())
	}
	return &object.Error{Message: name + ": " + err.Error()}
}

// rootArgs reads the become, owner and group arguments.
func rootArgs(name string, kwargs object.Kwargs) (bool, become.Owner, object.Object) {
	asRoot, errObj := kwargs.GetBool("become", false)
	if errObj != nil {
		return false, become.Owner{}, errObj
	}
	var owner become.Owner
	for _, k := range []struct {
		arg string
		to  *string
	}{{"owner", &owner.User}, {"group", &owner.Group}} {
		v := kwargs.Get(k.arg)
		switch v := v.(type) {
		case nil, *object.Null:
		case *object.String:
			*k.to, _ = v.AsString()
		case *object.Integer:
			if v.IntValue() < 0 {
				return false, owner, &object.Error{Message: fmt.Sprintf("%s: %s must not be negative", name, k.arg)}
			}
			*k.to = strconv.FormatInt(v.IntValue(), 10)
		default:
			return false, owner, &object.Error{Message: fmt.Sprintf("%s: %s must be a name or a number", name, k.arg)}
		}
	}
	return asRoot, owner, nil
}

const rootHelp = `  owner (str or int): The file's owner, by name or uid. Default: a new file belongs to whoever writes it
    (root with become=True); an existing one keeps its owner
  group (str or int): The file's group, by name or gid. Default: as for owner
  become (bool): Run as root (default False), through sudo: asked once, without a password where sudo
    allows that. The scriptling CLI refuses it with --deny-become`

func buildLibrary(inst *libraryInstance) *object.Library {
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
				asRoot, owner, errObj := rootArgs("ensure", kwargs)
				if errObj != nil {
					return errObj
				}
				return do(ctx, inst, asRoot, "ensure", ensureArgs{
					Path:       expandPath(path),
					Content:    content,
					Mode:       int(kwargs.MustGetInt("mode", defaultFileMode)),
					ModeGiven:  kwargs.Has("mode"),
					CreateOnly: kwargs.MustGetBool("create_only", false),
					Owner:      owner,
				})
			},
			HelpText: `ensure(path, content, mode=0o644, create_only=False, owner=None, group=None, become=False) - Ensure a file exists with the given content

Creates parent directories if needed. If the file already exists with the
same content, it is left unchanged. Otherwise the file is written with the
specified mode.

When mode, owner or group is given, an existing file with the right content
is given them too, and the call returns "updated" if that changed anything.

When create_only is True, an existing file is never modified: the call
returns "unchanged" without writing, even if the content differs. New files
are still written normally.

Parameters:
  path (str): Path to the file (supports ~ expansion)
  content (str): File contents
  mode (int): File permission mode (default 0o644)
  create_only (bool): If True, never modify an existing file (default False)
` + rootHelp + `

Returns:
  str: "created" if the file was newly written,
       "updated" if the file existed but its content, mode or owner differed,
       "unchanged" if the file existed as it should be,
       or if the file existed and create_only is True

Example:
  import scriptling.provision.file as file
  status = file.ensure("~/.gitconfig", "[user]\nname = Jane\n", mode=0o600)
  if status == file.CREATED:
      print("File created")
  elif status == file.UPDATED:
      print("File updated")

  file.ensure("/etc/app/app.conf", "port = 8080\n", become=True)`,
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
				asRoot, errObj := kwargs.GetBool("become", false)
				if errObj != nil {
					return errObj
				}
				return do(ctx, inst, asRoot, "absent", pathArgs{Path: expandPath(path)})
			},
			HelpText: `absent(path, become=False) - Ensure a file does not exist

Removes the file if it exists. Does nothing if the file is already absent.

Parameters:
  path (str): Path to the file (supports ~ expansion)
  become (bool): Run as root (default False)

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
				asRoot, owner, errObj := rootArgs("ensure_directory", kwargs)
				if errObj != nil {
					return errObj
				}
				return do(ctx, inst, asRoot, "ensure_directory", dirArgs{
					Path:      expandPath(path),
					Mode:      int(kwargs.MustGetInt("mode", defaultDirMode)),
					ModeGiven: kwargs.Has("mode"),
					Owner:     owner,
				})
			},
			HelpText: `ensure_directory(path, mode=0o755, owner=None, group=None, become=False) - Ensure a directory exists

Creates the directory and all parent directories if needed. When mode, owner
or group is given, the directory gets exactly them, whether it is new or not;
parent directories it creates get the defaults.

Parameters:
  path (str): Path to the directory (supports ~ expansion)
  mode (int): Directory permission mode (default 0o755)
` + rootHelp + `

Returns:
  str: file.CREATED if the directory was newly created,
       file.UPDATED if it existed and its mode or owner was changed,
       file.EXISTS if the directory already existed as it should be

Example:
  import scriptling.provision.file as file
  status = file.ensure_directory("~/.config/myapp", mode=0o700)
  if status == file.CREATED:
      print("Directory created")

  file.ensure_directory("/srv/app", owner="www-data", group="www-data", become=True)`,
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
				asRoot, errObj := kwargs.GetBool("become", false)
				if errObj != nil {
					return errObj
				}
				return do(ctx, inst, asRoot, "absent_directory", pathArgs{Path: expandPath(path)})
			},
			HelpText: `absent_directory(path, become=False) - Ensure an empty directory does not exist

Removes the directory if it exists and is empty. Returns an error if the
directory is not empty.

Parameters:
  path (str): Path to the directory (supports ~ expansion)
  become (bool): Run as root (default False)

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

				if err := validateBlockParams(comment, id); err != nil {
					return &object.Error{Message: "ensure_block: " + err.Error()}
				}
				if insertAfter == "" && position != positionStart && position != positionEnd {
					return &object.Error{Message: fmt.Sprintf("ensure_block: position must be %q or %q, got %q", positionStart, positionEnd, position)}
				}
				beginLine, endLine := blockMarkerLines(comment, id)
				for _, l := range splitContentLines(content) {
					if lineEqualsMarker(l, endLine) {
						return &object.Error{Message: "ensure_block: content contains the end marker"}
					}
					if lineEqualsMarker(l, beginLine) {
						return &object.Error{Message: "ensure_block: content contains the begin marker"}
					}
				}
				asRoot, owner, errObj := rootArgs("ensure_block", kwargs)
				if errObj != nil {
					return errObj
				}
				return do(ctx, inst, asRoot, "ensure_block", blockArgs{
					Path:        expandPath(path),
					Content:     content,
					ID:          id,
					Comment:     comment,
					Position:    position,
					InsertAfter: insertAfter,
					Mode:        int(kwargs.MustGetInt("mode", defaultFileMode)),
					CreateOnly:  kwargs.MustGetBool("create_only", false),
					Owner:       owner,
				})
			},
			HelpText: `ensure_block(path, content, id="managed", comment="#", position="end", insert_after="", mode=0o644, create_only=False, owner=None, group=None, become=False) - Maintain a managed block within a file

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
  mode (int): File permission mode used when writing the file (default 0o644)
  create_only (bool): If True, never modify an existing block (default False)
` + rootHelp + `

Returns:
  str: "created" if the block was newly inserted,
       "updated" if the block existed but content (or the file's owner) differed,
       "unchanged" if the block existed with identical content,
       or if the block existed and create_only is True

Example:
  import scriptling.provision.file as file
  status = file.ensure_block("~/.bashrc", "export EDITOR=vim\n", id="editor")
  status = file.ensure_block("/etc/hosts", "127.0.0.1 myapp\n", insert_after="localhost", become=True)`,
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
				asRoot, errObj := kwargs.GetBool("become", false)
				if errObj != nil {
					return errObj
				}
				return do(ctx, inst, asRoot, "absent_block", blockArgs{Path: expandPath(path), ID: id, Comment: comment})
			},
			HelpText: `absent_block(path, id="managed", comment="#", become=False) - Remove a managed block

Removes the marker-delimited block (markers and all content between them) for
the given id. Everything else in the file is left untouched. If no such block
exists, nothing happens.

Parameters:
  path (str): Path to the file (supports ~ expansion)
  id (str): Block identifier embedded in the markers (default "managed")
  comment (str): Comment prefix used to build markers (default "#")
  become (bool): Run as root (default False)

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
