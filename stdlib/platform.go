package stdlib

import (
	"context"
	"os"
	"runtime"

	"github.com/paularlott/scriptling/build"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// PlatformLibrary provides system/platform information (Python's platform module)
var PlatformLibrary = object.NewLibrary(map[string]*object.Builtin{
	"python_version": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			// Return Scriptling version as python_version for compatibility
			return &object.String{Value: build.Version}
		},
		HelpText: `python_version() - Returns the Python version

Returns the Python version (Scriptling version for compatibility).`,
	},
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
	"architecture": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			// Return architecture info similar to Python's platform.architecture()
			// For simplicity, return 64bit and empty linkage
			arch := "64bit"
			if runtime.GOARCH == "386" || runtime.GOARCH == "arm" {
				arch = "32bit"
			}
			// Return as a tuple-like list
			return &object.List{Elements: []object.Object{&object.String{Value: arch}, &object.String{Value: ""}}}
		},
		HelpText: `architecture() - Returns the architecture

Returns a list like ['64bit', ''] indicating bits and linkage.`,
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
	"platform": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			// Return platform string similar to Python's platform.platform()
			return &object.String{Value: runtime.GOOS + "-" + runtime.GOARCH}
		},
		HelpText: `platform() - Returns a string identifying the platform

Returns a string like 'darwin-amd64', 'linux-amd64', etc.`,
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
	"processor": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			return &object.String{Value: runtime.GOARCH}
		},
		HelpText: `processor() - Returns the processor name

Returns the processor name, often same as machine.`,
	},
	"node": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			if hostname, err := os.Hostname(); err == nil {
				return &object.String{Value: hostname}
			}
			return &object.String{Value: ""}
		},
		HelpText: `node() - Returns the network name (hostname)

Returns the computer's network name.`,
	},
	"release": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			// Return Scriptling version for compatibility
			return &object.String{Value: build.Version}
		},
		HelpText: `release() - Returns the system release

Returns the system release (Scriptling version for compatibility).`,
	},
	"version": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			// Return Scriptling version for compatibility
			return &object.String{Value: build.Version}
		},
		HelpText: `version() - Returns the system version

Returns the system version (Scriptling version for compatibility).`,
	},
	"uname": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			// Return uname info similar to Python's platform.uname()
			pairs := make(map[string]object.DictPair)

			// Get system name (capitalized, matching system() function)
			var systemName string
			switch runtime.GOOS {
			case "darwin":
				systemName = "Darwin"
			case "linux":
				systemName = "Linux"
			case "windows":
				systemName = "Windows"
			case "freebsd":
				systemName = "FreeBSD"
			default:
				systemName = runtime.GOOS
			}

			pairs["system"] = object.DictPair{Key: &object.String{Value: "system"}, Value: &object.String{Value: systemName}}
			pairs["machine"] = object.DictPair{Key: &object.String{Value: "machine"}, Value: &object.String{Value: runtime.GOARCH}}
			pairs["processor"] = object.DictPair{Key: &object.String{Value: "processor"}, Value: &object.String{Value: runtime.GOARCH}}

			// Get hostname
			node := ""
			if hostname, err := os.Hostname(); err == nil {
				node = hostname
			}
			pairs["node"] = object.DictPair{Key: &object.String{Value: "node"}, Value: &object.String{Value: node}}

			// For release and version, use Scriptling version for compatibility
			pairs["release"] = object.DictPair{Key: &object.String{Value: "release"}, Value: &object.String{Value: build.Version}}
			pairs["version"] = object.DictPair{Key: &object.String{Value: "version"}, Value: &object.String{Value: build.Version}}

			return &object.Dict{Pairs: pairs}
		},
		HelpText: `uname() - Returns system information

Returns a dict with keys: system, node, release, version, machine, processor`,
	},
}, nil, "Access to underlying platform's identifying data")
