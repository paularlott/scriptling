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

func RegisterSubprocessLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(SubprocessLibrary)
}

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
func (cp *CompletedProcess) AsBool() (bool, object.Object)                             { return true, nil }
func (cp *CompletedProcess) AsString() (string, object.Object)                 { return cp.Inspect(), nil }
func (cp *CompletedProcess) AsInt() (int64, object.Object)                     { return int64(cp.Returncode), nil }
func (cp *CompletedProcess) AsFloat() (float64, object.Object)                 { return float64(cp.Returncode), nil }
func (cp *CompletedProcess) AsDict() (map[string]object.Object, object.Object) { return nil, &object.Error{Message: object.ErrMustBeDict} }
func (cp *CompletedProcess) AsList() ([]object.Object, object.Object)          { return nil, &object.Error{Message: object.ErrMustBeList} }

// CompletedProcessClass defines the CompletedProcess class
var CompletedProcessClass = &object.Class{
	Name: "CompletedProcess",
	Methods: map[string]object.Object{
		"check_returncode": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil { return err }
				if instance, ok := args[0].(*object.Instance); ok {
					if returncode, ok := instance.Field("returncode").(*object.Integer); ok {
					if returncode.IntValue() != 0 {
						return errors.NewError("Command returned non-zero exit status %d", returncode.IntValue())
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

var SubprocessLibrary = object.NewLibrary(SubprocessLibraryName, map[string]*object.Builtin{
	"run": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil { return err }

			// Parse args - can be string or list
			var cmdArgs []string
			var cmdStr string
			if args[0].Type() == object.STRING_OBJ {
				cmdStr, _ = args[0].AsString()
				// Check if shell mode is enabled
				shell := false
				if sh, exists := kwargs.Kwargs["shell"]; exists {
					if b, ok := sh.(*object.Boolean); ok {
						shell = b.BoolValue()
					}
				}
				if shell {
					// In shell mode, pass string as-is to shell
					cmdArgs = []string{cmdStr}
				} else {
					// In non-shell mode, split string into arguments
					cmdArgs = strings.Fields(cmdStr)
				}
			} else if args[0].Type() == object.LIST_OBJ {
				list, _ := args[0].AsList()
				cmdArgs = make([]string, len(list))
				for i, arg := range list {
					if str, err := arg.AsString(); err == nil {
						cmdArgs[i] = str
					} else {
						return errors.NewTypeError("STRING", arg.Type().String())
					}
				}
			} else {
				return errors.NewTypeError("STRING or LIST", args[0].Type().String())
			}

			// Default options (matching Python's defaults)
			captureOutput := false
			shell := false
			cwd := ""
			timeout := 0.0
			check := false
			text := false
			encoding := "utf-8"
			inputData := ""
			env := make(map[string]string)

			// Parse kwargs (Python-style keyword arguments)
			if capture, exists := kwargs.Kwargs["capture_output"]; exists {
				if b, ok := capture.(*object.Boolean); ok {
					captureOutput = b.BoolValue()
				}
			}
			if sh, exists := kwargs.Kwargs["shell"]; exists {
				if b, ok := sh.(*object.Boolean); ok {
					shell = b.BoolValue()
				}
			}
			if wd, exists := kwargs.Kwargs["cwd"]; exists {
				if s, ok := wd.(*object.String); ok {
					cwd = s.StringValue()
				}
			}
			if to, exists := kwargs.Kwargs["timeout"]; exists {
				if f, ok := to.(*object.Float); ok {
					timeout = f.FloatValue()
				} else if i, ok := to.(*object.Integer); ok {
					timeout = float64(i.IntValue())
				}
			}
			if ch, exists := kwargs.Kwargs["check"]; exists {
				if b, ok := ch.(*object.Boolean); ok {
					check = b.BoolValue()
				}
			}
			if txt, exists := kwargs.Kwargs["text"]; exists {
				if b, ok := txt.(*object.Boolean); ok {
					text = b.BoolValue()
				}
			}
			if enc, exists := kwargs.Kwargs["encoding"]; exists {
				if s, ok := enc.(*object.String); ok {
					encoding = s.StringValue()
				}
			}
			if inp, exists := kwargs.Kwargs["input"]; exists {
				if s, ok := inp.(*object.String); ok {
					inputData = s.StringValue()
				}
			}
			if envDict, exists := kwargs.Kwargs["env"]; exists {
				if d, ok := envDict.(*object.Dict); ok {
					for _, pair := range d.Pairs {
						if valStr, ok := pair.Value.(*object.String); ok {
							env[pair.StringKey()] = valStr.StringValue()
						}
					}
				}
			}

			// Handle string args with shell=True
			if shell && args[0].Type() == object.STRING_OBJ {
				cmdArgs = []string{"sh", "-c", cmdStr}
			}

			// Execute command
			var cmd *exec.Cmd
			if shell && args[0].Type() == object.STRING_OBJ {
				cmd = exec.Command(cmdArgs[0], cmdArgs[1:]...)
			} else {
				cmd = exec.Command(cmdArgs[0], cmdArgs[1:]...)
			}

			if cwd != "" {
				cmd.Dir = cwd
			}

			// Set environment if provided
			if len(env) > 0 {
				cmd.Env = make([]string, 0, len(env))
				for k, v := range env {
					cmd.Env = append(cmd.Env, k+"="+v)
				}
			}

			// Set input if provided
			if inputData != "" {
				cmd.Stdin = strings.NewReader(inputData)
			}

			if timeout > 0 {
				ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout*float64(time.Second)))
				defer cancel()
				cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
				cmd.Dir = cwd
				if len(env) > 0 {
					cmd.Env = make([]string, 0, len(env))
					for k, v := range env {
						cmd.Env = append(cmd.Env, k+"="+v)
					}
				}
				if inputData != "" {
					cmd.Stdin = strings.NewReader(inputData)
				}
			}

			var stdout, stderr []byte
			var err error

			// Release the interpreter lock while the child process runs.
			object.RunBlocking(ctx, func() {
				if captureOutput {
					stdout, err = cmd.Output()
					if exitErr, ok := err.(*exec.ExitError); ok {
						stderr = exitErr.Stderr
					}
				} else {
					err = cmd.Run()
				}
			})

			returncode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					returncode = exitErr.ExitCode()
				} else {
					return errors.NewError("Command execution failed: %v", err)
				}
			}

			// Convert output based on text/encoding settings
			var stdoutStr, stderrStr string
			if text {
				// Decode using specified encoding (for now assume UTF-8, encoding param not yet implemented)
				_ = encoding
				stdoutStr = string(stdout)
				stderrStr = string(stderr)
			} else {
				// Return raw bytes as strings for compatibility
				stdoutStr = string(stdout)
				stderrStr = string(stderr)
			} // Create CompletedProcess instance
			instance := object.NewInstanceWithFields(CompletedProcessClass, map[string]object.Object{
					"args":       &object.List{Elements: make([]object.Object, len(cmdArgs))},
					"returncode": object.NewInteger(int64(returncode)),
					"stdout":     object.NewString(stdoutStr),
					"stderr":     object.NewString(stderrStr),
				}) // Fill args list
			for i, arg := range cmdArgs {
				instance.Field("args").(*object.List).Elements[i] = object.NewString(arg)
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
