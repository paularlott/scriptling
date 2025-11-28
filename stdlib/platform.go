package stdlib

import (
	"context"
	"runtime"

	"github.com/paularlott/scriptling/build"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// PlatformLibrary provides system/platform information (Python's platform module)
var PlatformLibrary = object.NewLibrary(map[string]*object.Builtin{
	"system": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			// Return OS name similar to Python's platform.system()
			switch runtime.GOOS {
			case "darwin":
				return &object.String{Value: "Darwin"}
			case "linux":
				return &object.String{Value: "Linux"}
			case "windows":
				return &object.String{Value: "Windows"}
			case "freebsd":
				return &object.String{Value: "FreeBSD"}
			default:
				return &object.String{Value: runtime.GOOS}
			}
		},
		HelpText: `system() - Returns the system/OS name

Returns 'Darwin', 'Linux', 'Windows', etc.`,
	},
	"machine": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			return &object.String{Value: runtime.GOARCH}
		},
		HelpText: `machine() - Returns the machine type (architecture)

Returns 'amd64', 'arm64', 'arm', etc.`,
	},
	"scriptling_version": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			return &object.String{Value: build.Version}
		},
		HelpText: `scriptling_version() - Returns Scriptling version string

Returns the current version of Scriptling.`,
	},
}, nil, "Access to underlying platform's identifying data")
