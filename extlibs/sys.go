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

// SysExitCode is used to communicate exit codes from sys.exit()
type SysExitCode struct {
	Code int
}

func (s *SysExitCode) Error() string {
	return "sys.exit called"
}

// IsSysExitCode checks if an error is a SysExitCode error.
// This is a helper for handling sys.exit() errors from Eval/CallFunction.
func IsSysExitCode(err error) bool {
	_, ok := err.(*SysExitCode)
	return ok
}

// GetSysExitCode extracts the SysExitCode from an error if it is one.
// Returns the SysExitCode and true if the error is a SysExitCode, nil and false otherwise.
func GetSysExitCode(err error) (*SysExitCode, bool) {
	sysExit, ok := err.(*SysExitCode)
	return sysExit, ok
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
				if len(args) > 0 {
					switch arg := args[0].(type) {
					case *object.Integer:
						code = int(arg.Value)
					case *object.String:
						// Return an exception with the custom message
						return &object.Exception{Message: arg.Value, ExceptionType: "SystemExit"}
					default:
						code = 1
					}
				}

				// Return a SystemExit exception that can be caught with try/except
				return &object.Exception{
					Message:       "SystemExit: " + object.NewInteger(int64(code)).Inspect(),
					ExceptionType: "SystemExit",
				}
			},
			HelpText: `exit([code]) - Raise SystemExit exception to exit the interpreter

Parameters:
  code - Exit status (default 0). If string, raises exception with that message.

The SystemExit exception can be caught with try/except to prevent termination.

Example:
  import sys
  sys.exit()      # Raise SystemExit: 0
  sys.exit(1)     # Raise SystemExit: 1
  sys.exit("Error message")  # Raise exception with custom message

  try:
      sys.exit(42)
  except Exception as e:
      print("Caught: " + str(e))  # "Caught: SystemExit: 42"`,
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
