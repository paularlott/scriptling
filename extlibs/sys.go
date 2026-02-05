package extlibs

import (
	"context"
	"os"
	"runtime"

	"github.com/paularlott/scriptling/build"
	"github.com/paularlott/scriptling/object"
)

func RegisterSysLibrary(registrar interface{ RegisterLibrary(*object.Library) }, argv []string) {
	registrar.RegisterLibrary(NewSysLibrary(argv))
}

// NewSysLibrary creates a new sys library with the given argv
func NewSysLibrary(argv []string) *object.Library {
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

	// SysLibrary provides system-specific parameters and functions
	return object.NewLibrary(SysLibraryName, map[string]*object.Builtin{
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
