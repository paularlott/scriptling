package extlibs

import (
	"bufio"
	"context"
	"fmt"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// RegisterConsoleLibrary registers the console library with a scriptling instance
func RegisterConsoleLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(NewConsoleLibrary())
}

// NewConsoleLibrary creates a new console library instance.
// The library reads from the environment's input reader (defaults to os.Stdin)
// and writes prompts to the environment's output writer (defaults to os.Stdout).
// Note: Each call to input() creates a new scanner, so the reader must maintain
// its position between calls (e.g., strings.Reader, os.Stdin, etc.)
func NewConsoleLibrary() *object.Library {
	return object.NewLibrary(ConsoleLibraryName, map[string]*object.Builtin{
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

				// Get environment from context
				env := getEnvFromContext(ctx)

				// Print prompt if provided
				if prompt != "" {
					fmt.Fprint(env.GetWriter(), prompt)
				}

				// Read from environment's input reader
				// Note: Creating a new scanner each time works because the underlying
				// reader (e.g., strings.Reader, os.Stdin) maintains its position
				scanner := bufio.NewScanner(env.GetReader())
				if !scanner.Scan() {
					if err := scanner.Err(); err != nil {
						return errors.NewError("input error: %s", err.Error())
					}
					return errors.NewError("EOF")
				}

				return &object.String{Value: scanner.Text()}
			},
			HelpText: `input([prompt]) -> str

Read a line from input. If prompt is present, it is written to output
without a trailing newline.

Returns the line read from input, without the trailing newline.`,
		},
	}, nil, "Console input/output functions")
}
