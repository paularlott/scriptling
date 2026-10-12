package main

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/paularlott/cli"
)

// Everything after the script file goes to the script: flags included,
// and words that happen to be subcommand names. A leading "--" is dropped,
// and scriptling's own flags still work before the file.
func TestScriptArgumentsPassThrough(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		ran     string // which command's Run was called
		file    string
		argv    []string
		libpath []string
	}{
		{"flags after the file", []string{"scriptling", "s.py", "a", "--verbose", "-L", "x"}, "scriptling", "s.py", []string{"a", "--verbose", "-L", "x"}, nil},
		{"leading double dash dropped", []string{"scriptling", "s.py", "--", "api.json", "--generate"}, "scriptling", "s.py", []string{"api.json", "--generate"}, nil},
		{"only the first double dash", []string{"scriptling", "s.py", "--", "x", "--", "y"}, "scriptling", "s.py", []string{"x", "--", "y"}, nil},
		{"own flags before the file", []string{"scriptling", "-L", "/tmp/lib", "s.py", "one"}, "scriptling", "s.py", []string{"one"}, []string{"/tmp/lib"}},
		{"subcommand name is a script argument", []string{"scriptling", "s.py", "cache"}, "scriptling", "s.py", []string{"cache"}, nil},
		{"help is a script argument", []string{"scriptling", "s.py", "help"}, "scriptling", "s.py", []string{"help"}, nil},
		{"bundle mode args after double dash", []string{"scriptling", "--package", "app", "--", "--allow-write"}, "scriptling", "--allow-write", []string{}, nil},
		{"subcommands still match first", []string{"scriptling", "cache", "clear"}, "clear", "", nil, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := buildRootCommand()
			var ran, file string
			var argv, libpath []string
			forEachCommand(root, func(c *cli.Command) {
				name := c.Name
				c.Run = func(ctx context.Context, cmd *cli.Command) error {
					ran = name
					if name == "scriptling" {
						file = cmd.GetStringArg("file")
						argv = scriptArgs(cmd, file)
						libpath = cmd.GetStringSlice("libpath")
					}
					return nil
				}
			})
			root.PreRun = nil
			root.PostRun = nil

			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()
			os.Args = tc.args
			if err := root.Execute(context.Background()); err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if ran != tc.ran || file != tc.file {
				t.Fatalf("ran %q with file %q, want %q with %q", ran, file, tc.ran, tc.file)
			}
			if tc.ran == "scriptling" {
				if len(argv) == 0 && len(tc.argv) == 0 {
					argv = tc.argv
				}
				if !reflect.DeepEqual(argv, tc.argv) {
					t.Fatalf("script args = %q, want %q", argv, tc.argv)
				}
				if tc.libpath != nil && !reflect.DeepEqual(libpath, tc.libpath) {
					t.Fatalf("libpath = %q, want %q", libpath, tc.libpath)
				}
			}
		})
	}
}
