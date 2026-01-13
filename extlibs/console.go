package extlibs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// RegisterConsoleLibrary registers the console library with a scriptling instance
func RegisterConsoleLibrary(registrar interface{ RegisterLibrary(string, *object.Library) }) {
	registrar.RegisterLibrary(ConsoleLibraryName, NewConsoleLibrary(os.Stdin))
}

// NewConsoleLibrary creates a new console library instance with its own scanner.
// Each scriptling environment should have its own console library to maintain
// proper buffering state for the input reader.
func NewConsoleLibrary(reader io.Reader) *object.Library {
	// Create a scanner that persists for the lifetime of this library instance
	scanner := bufio.NewScanner(reader)

	return object.NewLibrary(map[string]*object.Builtin{
		"input": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				prompt := ""
				if len(args) > 0 {
					p, err := args[0].AsString()
					if err != nil {
						return err
					}
					prompt = p
				}

				// Print prompt if provided
				if prompt != "" {
					fmt.Print(prompt)
				}

				// Read from stdin using this library's scanner
				if !scanner.Scan() {
					if err := scanner.Err(); err != nil {
						return errors.NewError("input error: %s", err.Error())
					}
					return errors.NewError("EOF")
				}

				return &object.String{Value: scanner.Text()}
			},
			HelpText: `input([prompt]) -> str

Read a line from input. If prompt is present, it is written to stdout
without a trailing newline.

Returns the line read from input, without the trailing newline.`,
		},
	}, nil, "Console input/output functions")
}
