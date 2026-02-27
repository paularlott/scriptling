package extlibs

import (
	"bufio"
	"context"
	"io"
	"os"
	"runtime"

	"github.com/paularlott/scriptling/build"
	"github.com/paularlott/scriptling/object"
)

type sysRegistrar interface {
	RegisterLibrary(*object.Library)
	SetObjectVar(string, object.Object) error
}

func RegisterSysLibrary(registrar sysRegistrar, argv []string, stdin io.Reader) {
	var br *bufio.Reader
	if stdin != nil {
		br = bufio.NewReader(stdin)
	}
	lib := newSysLibraryWithReader(argv, br)
	registrar.RegisterLibrary(lib)
	if br != nil {
		registrar.SetObjectVar("input", newInputBuiltinFromReader(br))
	}
}

// NewInputBuiltin returns an input() builtin backed by the given reader.
// Callers that manage their own Scriptling instance can use this to inject
// input() directly via SetObjectVar when the reader is known at a different
// point than RegisterSysLibrary.
func NewInputBuiltin(stdin io.Reader) *object.Builtin {
	return newInputBuiltinFromReader(bufio.NewReader(stdin))
}

func newInputBuiltinFromReader(r *bufio.Reader) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			line, err := r.ReadString('\n')
			if err != nil && line == "" {
				return &object.String{Value: ""}
			}
			if len(line) > 0 && line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
				if len(line) > 0 && line[len(line)-1] == '\r' {
					line = line[:len(line)-1]
				}
			}
			return &object.String{Value: line}
		},
		HelpText: `input([prompt]) - Read a line from stdin, stripping the trailing newline`,
	}
}

// NewSysLibrary creates a new sys library with the given argv and optional stdin reader.
func NewSysLibrary(argv []string, stdin io.Reader) *object.Library {
	var br *bufio.Reader
	if stdin != nil {
		br = bufio.NewReader(stdin)
	}
	return newSysLibraryWithReader(argv, br)
}

func newSysLibraryWithReader(argv []string, stdin *bufio.Reader) *object.Library {
	// Create argv list
	argvElements := make([]object.Object, len(argv))
	for i, arg := range argv {
		argvElements[i] = &object.String{Value: arg}
	}

	// Constants map
	constants := map[string]object.Object{
		// Platform identifier
		"platform": &object.String{Value: getPlatform()},

		// Version info
		"version": &object.String{Value: "Scriptling " + build.Version},

		// Maximum integer value
		"maxsize": object.NewInteger(9223372036854775807), // max int64

		// Path separator
		"path_sep": &object.String{Value: string(os.PathSeparator)},

		// argv
		"argv": &object.List{Elements: argvElements},
	}

	// stdin object
	if stdin != nil {
		constants["stdin"] = newStdinObject(stdin)
	}

	// SysLibrary provides system-specific parameters and functions
	lib := object.NewLibrary(SysLibraryName, map[string]*object.Builtin{
		"exit": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				code := 0
				message := ""
				if len(args) > 0 {
					switch arg := args[0].(type) {
					case *object.Integer:
						code = int(arg.Value)
					case *object.String:
						// Return an exception with the custom message and exit code 1
						return object.NewSystemExit(1, arg.Value)
					default:
						code = 1
					}
				}

				// Return a SystemExit exception that can be caught with try/except
				return object.NewSystemExit(code, message)
			},
			HelpText: `exit([code]) - Exit the interpreter immediately

Parameters:
  code - Exit status (default 0). If string, raises exception with that message.

IMPORTANT: sys.exit() CANNOT be caught by try/except blocks in your script.
The exception will bypass all except blocks and propagate to the caller (CLI, REPL, etc.).
However, finally blocks WILL execute before the exception propagates.

This behavior differs from most exceptions which can be caught. To handle errors gracefully,
use raise() with an exception message instead.

Returns:
  Does not return - propagates a SystemExit exception to the caller.

Example:
  import sys
  sys.exit()      # Clean exit with code 0
  sys.exit(1)     # Exit with error code 1
  sys.exit("Error message")  # Exit with error message and code 1

  # This will NOT catch sys.exit() - except blocks are bypassed:
  try:
      sys.exit(42)
  except Exception as e:
      print("This will never print - except is bypassed!")
  finally:
      print("This WILL print - finally executes")

  # To handle errors gracefully, use raise instead:
  try:
      if something_bad:
          raise("Something bad happened")
  except Exception as e:
      print("Caught:", e)  # This works
`,
		},
	}, constants, "System-specific parameters and functions (extended library)")
	return lib
}

// stdinHolder wraps a *bufio.Reader so it can live in an Instance's Fields.
type stdinHolder struct{ r *bufio.Reader }

func (h *stdinHolder) Type() object.ObjectType                           { return object.BUILTIN_OBJ }
func (h *stdinHolder) Inspect() string                                   { return "<stdin>" }
func (h *stdinHolder) AsString() (string, object.Object)                 { return "", &object.Error{Message: object.ErrMustBeString} }
func (h *stdinHolder) AsInt() (int64, object.Object)                     { return 0, &object.Error{Message: object.ErrMustBeInteger} }
func (h *stdinHolder) AsFloat() (float64, object.Object)                 { return 0, &object.Error{Message: object.ErrMustBeNumber} }
func (h *stdinHolder) AsBool() (bool, object.Object)                     { return true, nil }
func (h *stdinHolder) AsList() ([]object.Object, object.Object)          { return nil, &object.Error{Message: object.ErrMustBeList} }
func (h *stdinHolder) AsDict() (map[string]object.Object, object.Object) { return nil, &object.Error{Message: object.ErrMustBeDict} }
func (h *stdinHolder) CoerceString() (string, object.Object)             { return h.Inspect(), nil }
func (h *stdinHolder) CoerceInt() (int64, object.Object)                 { return 0, &object.Error{Message: object.ErrMustBeInteger} }
func (h *stdinHolder) CoerceFloat() (float64, object.Object)             { return 0, &object.Error{Message: object.ErrMustBeNumber} }

const stdinKey = "__stdin__"

func getStdinReader(inst *object.Instance) (*bufio.Reader, bool) {
	h, ok := inst.Fields[stdinKey]
	if !ok {
		return nil, false
	}
	sh, ok := h.(*stdinHolder)
	if !ok {
		return nil, false
	}
	return sh.r, true
}

var stdinClass = &object.Class{
	Name: "stdin",
	Methods: map[string]object.Object{
		"read": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 1 {
					return &object.Error{Message: "read() requires self"}
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return &object.Error{Message: "read(): invalid self"}
				}
				r, ok := getStdinReader(inst)
				if !ok {
					return &object.Error{Message: "read(): invalid stdin"}
				}
				data, _ := io.ReadAll(r)
				return &object.String{Value: string(data)}
			},
			HelpText: `read() - Read all remaining data from stdin`,
		},
		"readline": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 1 {
					return &object.Error{Message: "readline() requires self"}
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return &object.Error{Message: "readline(): invalid self"}
				}
				r, ok := getStdinReader(inst)
				if !ok {
					return &object.Error{Message: "readline(): invalid stdin"}
				}
				line, err := r.ReadString('\n')
				if err != nil && line == "" {
					return &object.String{Value: ""}
				}
				return &object.String{Value: line}
			},
			HelpText: `readline() - Read one line from stdin (includes newline)`,
		},
		"__iter__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 1 {
					return &object.Error{Message: "__iter__() requires self"}
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return &object.Error{Message: "__iter__(): invalid self"}
				}
				r, ok := getStdinReader(inst)
				if !ok {
					return &object.Error{Message: "__iter__(): invalid stdin"}
				}
				return object.NewIterator(func() (object.Object, bool) {
					line, err := r.ReadString('\n')
					if err != nil && line == "" {
						return nil, false
					}
					return &object.String{Value: line}, true
				})
			},
		},
	},
}

func newStdinObject(r *bufio.Reader) *object.Instance {
	return &object.Instance{
		Class:  stdinClass,
		Fields: map[string]object.Object{stdinKey: &stdinHolder{r: r}},
	}
}

func getPlatform() string {
	switch runtime.GOOS {
	case "darwin":
		return "darwin"
	case "linux":
		return "linux"
	case "windows":
		return "win32"
	default:
		return runtime.GOOS
	}
}
