package extlibs

import (
	"context"
	"os"
	"runtime"

	"github.com/paularlott/scriptling/build"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// SysExitCode is used to communicate exit codes from sys.exit()
type SysExitCode struct {
	Code int
}

func (s *SysExitCode) Error() string {
	return "sys.exit called"
}

// SysExitCallback can be set to customize sys.exit() behavior
var SysExitCallback func(code int)

// SysArgv contains command-line arguments (set by the CLI)
var SysArgv []string

// sysConstants holds the mutable constants map
var sysConstants = map[string]object.Object{
	// Platform identifier
	"platform": &object.String{Value: getPlatform()},

	// Version info
	"version": &object.String{Value: "Scriptling " + build.Version},

	// Maximum integer value
	"maxsize": object.NewInteger(9223372036854775807), // max int64

	// Path separator
	"path_sep": &object.String{Value: string(os.PathSeparator)},

	// argv - will be populated by CLI
	"argv": &object.List{Elements: []object.Object{}},
}

// SysLibrary provides system-specific parameters and functions
// NOTE: This is an extended library and not enabled by default
var SysLibrary = object.NewLibrary(map[string]*object.Builtin{
	"exit": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			code := 0
			if len(args) > 0 {
				switch arg := args[0].(type) {
				case *object.Integer:
					code = int(arg.Value)
				case *object.String:
					// Print the message to stderr and exit with code 1
					if SysExitCallback != nil {
						SysExitCallback(1)
					}
					return errors.NewError("%s", arg.Value)
				default:
					code = 1
				}
			}

			if SysExitCallback != nil {
				SysExitCallback(code)
			}

			// Return a special error that can be caught by the runtime
			return errors.NewError("SystemExit: %d", code)
		},
		HelpText: `exit([code]) - Exit the interpreter with optional status code

Parameters:
  code - Exit status (default 0). If string, prints message and exits with 1.

Example:
  import sys
  sys.exit()      # Exit with code 0
  sys.exit(1)     # Exit with code 1
  sys.exit("Error message")  # Print error and exit with 1`,
	},
}, sysConstants, "System-specific parameters and functions (extended library)")

// GetSysArgv returns the argv list as an Object
func GetSysArgv() *object.List {
	elements := make([]object.Object, len(SysArgv))
	for i, arg := range SysArgv {
		elements[i] = &object.String{Value: arg}
	}
	return &object.List{Elements: elements}
}

// SetupSysLibrary configures the sys library with argv
func SetupSysLibrary(argv []string) {
	SysArgv = argv
	// Update the argv constant
	sysConstants["argv"] = GetSysArgv()
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
