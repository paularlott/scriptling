package extlibs

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// CompletedProcess represents the result of a subprocess.run call
type CompletedProcess struct {
	Args       []string
	Returncode int
	Stdout     string
	Stderr     string
}

func (cp *CompletedProcess) Type() object.ObjectType { return object.INSTANCE_OBJ }
func (cp *CompletedProcess) Inspect() string {
	return fmt.Sprintf("CompletedProcess(args=%v, returncode=%d)", cp.Args, cp.Returncode)
}
func (cp *CompletedProcess) AsBool() bool                             { return true }
func (cp *CompletedProcess) AsString() (string, bool)                 { return cp.Inspect(), true }
func (cp *CompletedProcess) AsInt() (int64, bool)                     { return 0, false }
func (cp *CompletedProcess) AsFloat() (float64, bool)                 { return 0, false }
func (cp *CompletedProcess) AsDict() (map[string]object.Object, bool) { return nil, false }
func (cp *CompletedProcess) AsList() ([]object.Object, bool)          { return nil, false }

// CompletedProcessClass defines the CompletedProcess class
var CompletedProcessClass = &object.Class{
	Name: "CompletedProcess",
	Methods: map[string]object.Object{
		"check_returncode": &object.Builtin{
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				if instance, ok := args[0].(*object.Instance); ok {
					if returncode, ok := instance.Fields["returncode"].(*object.Integer); ok {
						if returncode.Value != 0 {
							return errors.NewError("Command returned non-zero exit status %d", returncode.Value)
						}
						return args[0]
					}
				}
				return errors.NewError("Invalid CompletedProcess instance")
			},
			HelpText: `check_returncode() - Check if the process returned successfully

Raises an exception if returncode is non-zero.`,
		},
	},
}

var SubprocessLibrary = object.NewLibrary(map[string]*object.Builtin{
	"run": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) < 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			// Parse args - can be string or list
			var cmdArgs []string
			if args[0].Type() == object.STRING_OBJ {
				cmdStr, _ := args[0].AsString()
				cmdArgs = strings.Fields(cmdStr)
			} else if args[0].Type() == object.LIST_OBJ {
				list, _ := args[0].AsList()
				cmdArgs = make([]string, len(list))
				for i, arg := range list {
					if str, ok := arg.AsString(); ok {
						cmdArgs[i] = str
					} else {
						return errors.NewTypeError("STRING", arg.Type().String())
					}
				}
			} else {
				return errors.NewTypeError("STRING or LIST", args[0].Type().String())
			}

			// Default options
			captureOutput := false
			shell := false
			cwd := ""
			timeout := 0
			check := false

			// Parse kwargs
			if len(args) > 1 {
				if options, ok := args[1].(*object.Dict); ok {
					if capture, exists := options.Pairs["capture_output"]; exists {
						if b, ok := capture.Value.AsBool(); ok {
							captureOutput = b
						}
					}
					if sh, exists := options.Pairs["shell"]; exists {
						if b, ok := sh.Value.AsBool(); ok {
							shell = b
						}
					}
					if wd, exists := options.Pairs["cwd"]; exists {
						if s, ok := wd.Value.AsString(); ok {
							cwd = s
						}
					}
					if to, exists := options.Pairs["timeout"]; exists {
						if i, ok := to.Value.AsInt(); ok {
							timeout = int(i)
						}
					}
					if ch, exists := options.Pairs["check"]; exists {
						if b, ok := ch.Value.AsBool(); ok {
							check = b
						}
					}
				}
			}

			// Execute command
			var cmd *exec.Cmd
			if shell {
				cmd = exec.Command("sh", "-c", strings.Join(cmdArgs, " "))
			} else {
				cmd = exec.Command(cmdArgs[0], cmdArgs[1:]...)
			}

			if cwd != "" {
				cmd.Dir = cwd
			}

			if timeout > 0 {
				ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
				defer cancel()
				cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
				cmd.Dir = cwd
			}

			var stdout, stderr []byte
			var err error

			if captureOutput {
				stdout, err = cmd.Output()
				if exitErr, ok := err.(*exec.ExitError); ok {
					stderr = exitErr.Stderr
				}
			} else {
				err = cmd.Run()
			}

			returncode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					returncode = exitErr.ExitCode()
				} else {
					return errors.NewError("Command execution failed: %v", err)
				}
			}

			// Create CompletedProcess instance
			instance := &object.Instance{
				Class: CompletedProcessClass,
				Fields: map[string]object.Object{
					"args":       &object.List{Elements: make([]object.Object, len(cmdArgs))},
					"returncode": &object.Integer{Value: int64(returncode)},
					"stdout":     &object.String{Value: string(stdout)},
					"stderr":     &object.String{Value: string(stderr)},
				},
			}

			// Fill args list
			for i, arg := range cmdArgs {
				instance.Fields["args"].(*object.List).Elements[i] = &object.String{Value: arg}
			}

			if check && returncode != 0 {
				return errors.NewError("Command returned non-zero exit status %d", returncode)
			}

			return instance
		},
		HelpText: `run(args, options={}) - Run a command

Runs a command and returns a CompletedProcess instance.

Parameters:
  args (string or list): Command to run. If string, split on spaces. If list, each element is an argument.
  options (dict, optional): Options
    - capture_output (bool): Capture stdout and stderr (default: false)
    - shell (bool): Run command through shell (default: false)
    - cwd (string): Working directory for command
    - timeout (int): Timeout in seconds
    - check (bool): Raise exception if returncode is non-zero

Returns:
  CompletedProcess instance with args, returncode, stdout, stderr`,
	},
}, map[string]object.Object{}, "Subprocess library for running external commands")
