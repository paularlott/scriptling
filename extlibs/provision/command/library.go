// Package command is scriptling.provision.command: run commands while
// provisioning a machine, as the script's user or as root.
package command

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/provision/become"
	"github.com/paularlott/scriptling/object"
)

const (
	LibraryName = "scriptling.provision.command"
	LibraryDesc = "Run commands while provisioning a machine, as the script's user or as root"
)

type libraryInstance struct {
	become *become.Become
}

// Option configures the library.
type Option func(*libraryInstance)

// WithBecome lets scripts run commands as root with become=True, through b.
// Without it, become=True fails unless the process is already root.
func WithBecome(b *become.Become) Option {
	return func(i *libraryInstance) { i.become = b }
}

// Register registers the library with running as root off.
func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	RegisterConfigured(registrar)
}

// RegisterConfigured registers the library with its options.
func RegisterConfigured(registrar interface{ RegisterLibrary(*object.Library) }, opts ...Option) {
	inst := &libraryInstance{}
	for _, o := range opts {
		o(inst)
	}
	registrar.RegisterLibrary(buildLibrary(inst))
}

// PermissionError reports err as the uncatchable PermissionError when it is
// running as root where that is not allowed.
func PermissionError(name string, err error) object.Object {
	if stderrors.Is(err, become.ErrNotEnabled) {
		return errors.NewPermissionError("%s: %s", name, err.Error())
	}
	return &object.Error{Message: name + ": " + err.Error()}
}

func buildLibrary(inst *libraryInstance) *object.Library {
	return object.NewLibrary(LibraryName, map[string]*object.Builtin{
		"run": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return run(ctx, inst, kwargs, args)
			},
			HelpText: `run(cmd, become=False, cwd=None, env=None, input=None, timeout=None, check=True) - Run a command

Runs cmd and waits for it, capturing its output. A list is a program and its
arguments, run as they are; a string is a command line for /bin/sh, so it may
use pipes, && and the like.

Parameters:
  cmd (list or str): The command
  become (bool): Run as root (default False), through sudo: asked once, without a password where sudo
    allows that. The scriptling CLI refuses it with --deny-become
  cwd (str): Working directory (supports ~ expansion). Default: the current one
  env (dict): Environment variables to add to the inherited environment
  input (str): Text for the command's standard input. Default: none
  timeout (float): Seconds to allow; the command is stopped after that and an error raised
  check (bool): Raise an error when the command exits with a non-zero status (default True)

Returns:
  Result: returncode (int), stdout (str), stderr (str), args (list), and ok (bool):
    True when returncode is 0

Example:
  import scriptling.provision.command as command

  command.run(["systemctl", "restart", "nginx"], become=True)

  r = command.run("id -un")
  print("running as", r.stdout.strip())

  r = command.run(["grep", "-q", "docker", "/etc/group"], check=False)
  if not r.ok:
      print("no docker group")`,
		},
		"is_root": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 0); err != nil {
					return err
				}
				return object.NewBoolean(become.IsRoot())
			},
			HelpText: `is_root() - Whether the script runs as root

Returns:
  bool: True when the script itself runs as root (with sudo scriptling, say)`,
		},
		"require_root": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 0); err != nil {
					return err
				}
				if !become.IsRoot() {
					return &object.Error{Message: "require_root: this script must run as root (sudo scriptling ...)"}
				}
				return &object.Null{}
			},
			HelpText: `require_root() - Stop the script unless it runs as root

For a script meant to run as a whole with sudo. A script that runs as a user
and needs root for some steps passes become=True to them instead.

Raises:
  An error when the script is not running as root

Example:
  import scriptling.provision.command as command
  command.require_root()`,
		},
	}, nil, LibraryDesc)
}

func run(ctx context.Context, inst *libraryInstance, kwargs object.Kwargs, args []object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: fmt.Sprintf("run expected 1 argument, got %d", len(args))}
	}
	var ra become.RunArgs
	var display []string
	switch cmd := args[0].(type) {
	case *object.String:
		ra.Shell, _ = cmd.AsString()
		if strings.TrimSpace(ra.Shell) == "" {
			return errors.NewValueError("run: the command is empty")
		}
		display = []string{ra.Shell}
	case *object.List:
		for _, el := range cmd.Elements {
			s, ok := el.(*object.String)
			if !ok {
				return errors.NewTypeErrorTagged("run: each part of the command must be a string, not %s", el.Inspect())
			}
			v, _ := s.AsString()
			ra.Argv = append(ra.Argv, v)
		}
		if len(ra.Argv) == 0 {
			return errors.NewValueError("run: the command is empty")
		}
		display = ra.Argv
	default:
		return &object.Error{Message: "run: cmd must be a list of strings or a string"}
	}

	asRoot, errObj := kwargs.GetBool("become", false)
	if errObj != nil {
		return errObj
	}
	check, errObj := kwargs.GetBool("check", true)
	if errObj != nil {
		return errObj
	}
	if v := kwargs.Get("cwd"); v != nil {
		if _, null := v.(*object.Null); !null {
			cwd, errObj := v.AsString()
			if errObj != nil {
				return &object.Error{Message: "run: cwd must be a string"}
			}
			ra.Dir = expandPath(cwd)
		}
	}
	if v := kwargs.Get("env"); v != nil {
		if _, null := v.(*object.Null); !null {
			d, ok := v.(*object.Dict)
			if !ok {
				return &object.Error{Message: "run: env must be a dict"}
			}
			for _, pair := range d.Pairs {
				val, errObj := pair.Value.AsString()
				if errObj != nil {
					val = pair.Value.Inspect()
				}
				ra.Env = append(ra.Env, pair.StringKey()+"="+val)
			}
			sort.Strings(ra.Env)
		}
	}
	if v := kwargs.Get("input"); v != nil {
		if _, null := v.(*object.Null); !null {
			in, errObj := v.AsString()
			if errObj != nil {
				return &object.Error{Message: "run: input must be a string"}
			}
			ra.Input = []byte(in)
		}
	}
	if v := kwargs.Get("timeout"); v != nil {
		if _, null := v.(*object.Null); !null {
			secs, errObj := v.AsFloat()
			if errObj != nil || secs <= 0 {
				return errors.NewValueError("run: timeout must be a number of seconds above 0")
			}
			ra.Timeout = time.Duration(secs * float64(time.Second))
		}
	}

	var res become.RunResult
	var err error
	object.RunBlocking(ctx, func() { res, err = become.Run(ctx, inst.become, asRoot, ra) })
	if err != nil {
		return PermissionError("run", err)
	}
	if check && res.Code != 0 {
		return &object.Error{Message: "run: " + Failure(display, res)}
	}
	argList := make([]object.Object, len(display))
	for i, a := range display {
		argList[i] = object.NewString(a)
	}
	return object.NewInstanceWithFields(resultClass(), map[string]object.Object{
		"args":       &object.List{Elements: argList},
		"returncode": object.NewInteger(int64(res.Code)),
		"stdout":     object.NewString(string(res.Stdout)),
		"stderr":     object.NewString(string(res.Stderr)),
		"ok":         object.NewBoolean(res.Code == 0),
	})
}

// Failure describes a command that exited with a non-zero status, with the
// end of what it said.
func Failure(cmd []string, res become.RunResult) string {
	msg := fmt.Sprintf("%s exited with status %d", Quote(cmd), res.Code)
	said := strings.TrimSpace(string(res.Stderr))
	if said == "" {
		said = strings.TrimSpace(string(res.Stdout))
	}
	if said == "" {
		return msg
	}
	lines := strings.Split(said, "\n")
	if len(lines) > 20 {
		lines = append([]string{"..."}, lines[len(lines)-20:]...)
	}
	return msg + ":\n" + strings.Join(lines, "\n")
}

// Quote shows a command as a shell would take it.
func Quote(cmd []string) string {
	if len(cmd) == 1 {
		return cmd[0] // a shell command line, as given
	}
	parts := make([]string, len(cmd))
	for i, a := range cmd {
		if a != "" && !strings.ContainsAny(a, " \t\n'\"\\$`|&;<>()*?[]{}~#!") {
			parts[i] = a
		} else {
			parts[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
	}
	return strings.Join(parts, " ")
}

var (
	resultClassOnce sync.Once
	resultClassObj  *object.Class
)

func resultClass() *object.Class {
	resultClassOnce.Do(func() {
		cb := object.NewClassBuilder("Result")
		cb.MethodWithHelp("__repr__", func(self *object.Instance) object.Object {
			code, _ := self.Field("returncode").AsInt()
			return object.NewString(fmt.Sprintf("Result(args=%s, returncode=%d)", self.Field("args").Inspect(), code))
		}, `__repr__() - The result, briefly`)
		resultClassObj = cb.Build()
	})
	return resultClassObj
}

func expandPath(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}
