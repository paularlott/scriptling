package extlibs

import (
	"bufio"
	"context"
	"io"
	"os"
	"runtime"

	"github.com/paularlott/scriptling/build"
	"github.com/paularlott/scriptling/evaluator"
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
			var line string
			var err error
			object.RunBlocking(ctx, func() { line, err = r.ReadString('\n') })
			if err != nil && line == "" {
				return object.NewString("")
			}
			if len(line) > 0 && line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
				if len(line) > 0 && line[len(line)-1] == '\r' {
					line = line[:len(line)-1]
				}
			}
			return object.NewString(line)
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
		argvElements[i] = object.NewString(arg)
	}

	// Constants map
	constants := map[string]object.Object{
		// Platform identifier
		"platform": object.NewString(getPlatform()),

		// Version info
		"version": object.NewString("Scriptling " + build.Version),

		// Maximum integer value
		"maxsize": object.NewInteger(9223372036854775807), // max int64

		// Path separator
		"path_sep": object.NewString(string(os.PathSeparator)),

		// argv
		"argv": &object.List{Elements: argvElements},
	}

	// stdin object
	if stdin != nil {
		constants["stdin"] = newStdinObject(stdin)
	}

	// stdout/stderr objects. Their writers are resolved from the calling
	// environment at write time, so they follow output capture, custom
	// writers and sandbox output discarding exactly like print().
	constants["stdout"] = newStreamObject(false)
	constants["stderr"] = newStreamObject(true)

	// SysLibrary provides system-specific parameters and functions
	lib := object.NewLibrary(SysLibraryName, map[string]*object.Builtin{
		"exit": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				code := 0
				message := ""
				if len(args) > 0 {
					switch arg := args[0].(type) {
					case *object.Integer:
						code = int(arg.IntValue())
					case *object.String:
						return object.NewSystemExit(1, arg.StringValue())
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

func (h *stdinHolder) Type() object.ObjectType { return object.BUILTIN_OBJ }
func (h *stdinHolder) Inspect() string         { return "<stdin>" }
func (h *stdinHolder) AsString() (string, object.Object) {
	return "", &object.Error{Message: object.ErrMustBeString}
}
func (h *stdinHolder) AsInt() (int64, object.Object) {
	return 0, &object.Error{Message: object.ErrMustBeInteger}
}
func (h *stdinHolder) AsFloat() (float64, object.Object) {
	return 0, &object.Error{Message: object.ErrMustBeNumber}
}
func (h *stdinHolder) AsBool() (bool, object.Object) { return true, nil }
func (h *stdinHolder) AsList() ([]object.Object, object.Object) {
	return nil, &object.Error{Message: object.ErrMustBeList}
}
func (h *stdinHolder) AsDict() (map[string]object.Object, object.Object) {
	return nil, &object.Error{Message: object.ErrMustBeDict}
}
func (h *stdinHolder) CoerceString() (string, object.Object) { return h.Inspect(), nil }
func (h *stdinHolder) CoerceInt() (int64, object.Object) {
	return 0, &object.Error{Message: object.ErrMustBeInteger}
}
func (h *stdinHolder) CoerceFloat() (float64, object.Object) {
	return 0, &object.Error{Message: object.ErrMustBeNumber}
}

const stdinKey = "__stdin__"

func getStdinReader(inst *object.Instance) (*bufio.Reader, bool) {
	h, ok := inst.GetField(stdinKey)
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
				var data []byte
				object.RunBlocking(ctx, func() { data, _ = io.ReadAll(r) })
				return object.NewString(string(data))
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
				var line string
				var err error
				object.RunBlocking(ctx, func() { line, err = r.ReadString('\n') })
				if err != nil && line == "" {
					return object.NewString("")
				}
				return object.NewString(line)
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
					var line string
					var err error
					object.RunBlocking(ctx, func() { line, err = r.ReadString('\n') })
					if err != nil && line == "" {
						return nil, false
					}
					return object.NewString(line), true
				})
			},
		},
	},
}

func newStdinObject(r *bufio.Reader) *object.Instance {
	return object.NewInstanceWithFields(stdinClass, map[string]object.Object{stdinKey: &stdinHolder{r: r}})
}

// streamHolder marks an Instance as an environment-bound output stream
// (sys.stdout or sys.stderr) and records which of the environment's two
// writers it resolves to.
type streamHolder struct{ stderr bool }

func (h *streamHolder) Type() object.ObjectType { return object.BUILTIN_OBJ }
func (h *streamHolder) Inspect() string {
	if h.stderr {
		return "<stderr>"
	}
	return "<stdout>"
}
func (h *streamHolder) AsString() (string, object.Object) {
	return "", &object.Error{Message: object.ErrMustBeString}
}
func (h *streamHolder) AsInt() (int64, object.Object) {
	return 0, &object.Error{Message: object.ErrMustBeInteger}
}
func (h *streamHolder) AsFloat() (float64, object.Object) {
	return 0, &object.Error{Message: object.ErrMustBeNumber}
}
func (h *streamHolder) AsBool() (bool, object.Object) { return true, nil }
func (h *streamHolder) AsList() ([]object.Object, object.Object) {
	return nil, &object.Error{Message: object.ErrMustBeList}
}
func (h *streamHolder) AsDict() (map[string]object.Object, object.Object) {
	return nil, &object.Error{Message: object.ErrMustBeDict}
}
func (h *streamHolder) CoerceString() (string, object.Object) { return h.Inspect(), nil }
func (h *streamHolder) CoerceInt() (int64, object.Object) {
	return 0, &object.Error{Message: object.ErrMustBeInteger}
}
func (h *streamHolder) CoerceFloat() (float64, object.Object) {
	return 0, &object.Error{Message: object.ErrMustBeNumber}
}

const streamKey = "__stream__"

// getStreamHolder returns the stream holder stored on an Instance.
func getStreamHolder(inst *object.Instance) (*streamHolder, bool) {
	f, ok := inst.GetField(streamKey)
	if !ok {
		return nil, false
	}
	h, ok := f.(*streamHolder)
	if !ok {
		return nil, false
	}
	return h, true
}

// streamSelf validates the receiver of a stream method call.
func streamSelf(args []object.Object, method string) (*object.Instance, *streamHolder, object.Object) {
	if len(args) < 1 {
		return nil, nil, &object.Error{Message: method + "() requires self"}
	}
	inst, ok := args[0].(*object.Instance)
	if !ok {
		return nil, nil, &object.Error{Message: method + "(): invalid self"}
	}
	h, ok := getStreamHolder(inst)
	if !ok {
		return nil, nil, &object.Error{Message: method + "(): invalid stream"}
	}
	return inst, h, nil
}

// streamWriter resolves the io.Writer a stream object should write to,
// based on the environment of the calling code. This is evaluated per call
// so the stream follows later SetOutputWriter/EnableOutputCapture changes.
func streamWriter(ctx context.Context, h *streamHolder) io.Writer {
	env := evaluator.GetEnvFromContext(ctx)
	if h.stderr {
		return env.GetErrorWriter()
	}
	return env.GetWriter()
}

// streamName returns the display name of a stream for error messages.
func streamName(h *streamHolder) string {
	if h.stderr {
		return "stderr"
	}
	return "stdout"
}

// flusher is implemented by writers that buffer output.
type flusher interface{ Flush() error }

// newStreamClass builds the class backing sys.stdout and sys.stderr. The
// methods are shared; which environment writer they target comes from the
// instance's stream holder.
func newStreamClass(name string) *object.Class {
	return &object.Class{
		Name: name,
		Methods: map[string]object.Object{
			"write": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					_, h, errObj := streamSelf(args, "write")
					if errObj != nil {
						return errObj
					}
					if len(args) < 2 {
						return &object.Error{Message: "write() requires a string argument"}
					}
					s, err := args[1].AsString()
					if err != nil {
						return &object.Error{Message: streamName(h) + ".write() requires a string argument"}
					}
					w := streamWriter(ctx, h)
					n, werr := io.WriteString(w, s)
					if werr != nil {
						return &object.Error{Message: streamName(h) + ".write() failed: " + werr.Error()}
					}
					return object.NewInteger(int64(n))
				},
				HelpText: `write(s) - Write string s to the stream; returns number of characters written`,
			},
			"writelines": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					_, h, errObj := streamSelf(args, "writelines")
					if errObj != nil {
						return errObj
					}
					if len(args) < 2 {
						return &object.Error{Message: "writelines() requires a list of strings"}
					}
					lines, err := args[1].AsList()
					if err != nil {
						return err
					}
					w := streamWriter(ctx, h)
					for _, line := range lines {
						s, serr := line.AsString()
						if serr != nil {
							return &object.Error{Message: streamName(h) + ".writelines() requires a list of strings"}
						}
						if _, werr := io.WriteString(w, s); werr != nil {
							return &object.Error{Message: streamName(h) + ".writelines() failed: " + werr.Error()}
						}
					}
					return &object.Null{}
				},
				HelpText: `writelines(lines) - Write each string in lines to the stream (no separators added)`,
			},
			"flush": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					_, h, errObj := streamSelf(args, "flush")
					if errObj != nil {
						return errObj
					}
					w := streamWriter(ctx, h)
					if f, ok := w.(flusher); ok {
						if ferr := f.Flush(); ferr != nil {
							return &object.Error{Message: streamName(h) + ".flush() failed: " + ferr.Error()}
						}
					}
					return &object.Null{}
				},
				HelpText: `flush() - Flush the stream's write buffer (no-op for unbuffered streams)`,
			},
			"isatty": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					_, h, errObj := streamSelf(args, "isatty")
					if errObj != nil {
						return errObj
					}
					w := streamWriter(ctx, h)
					tty := false
					if f, ok := w.(*os.File); ok {
						if fi, statErr := f.Stat(); statErr == nil {
							tty = fi.Mode()&os.ModeCharDevice != 0
						}
					}
					return object.NewBoolean(tty)
				},
				HelpText: `isatty() - Return true if the stream is a terminal`,
			},
			"__enter__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) < 1 {
						return &object.Error{Message: "__enter__() requires self"}
					}
					return args[0]
				},
			},
			"__exit__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) < 1 {
						return object.NewBoolean(false)
					}
					if inst, ok := args[0].(*object.Instance); ok {
						if h, ok := getStreamHolder(inst); ok {
							if f, ok := streamWriter(ctx, h).(flusher); ok {
								f.Flush()
							}
						}
					}
					return object.NewBoolean(false)
				},
			},
		},
	}
}

var (
	stdoutClass = newStreamClass("stdout")
	stderrClass = newStreamClass("stderr")
)

func newStreamObject(stderr bool) *object.Instance {
	class := stdoutClass
	if stderr {
		class = stderrClass
	}
	return object.NewInstanceWithFields(class, map[string]object.Object{streamKey: &streamHolder{stderr: stderr}})
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
