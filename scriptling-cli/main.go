package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/paularlott/cli"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/agent"
	"github.com/paularlott/scriptling/extlibs/ai"
	"github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

func main() {
	cmd := &cli.Command{
		Name:        "scriptling",
		Version:     "1.0.0",
		Usage:       "Scriptling interpreter",
		Description: "Run Scriptling scripts from files, stdin, or interactively",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "interactive",
				Usage:   "Start interactive mode",
				Aliases: []string{"i"},
			},
		},
		MaxArgs: cli.UnlimitedArgs,
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:     "file",
				Usage:    "Script file to execute",
				Required: false,
			},
		},
		Run: runScriptling,
	}

	err := cmd.Execute(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runScriptling(ctx context.Context, cmd *cli.Command) error {
	// Create Scriptling interpreter
	p := scriptling.New()

	// Register all standard libraries
	stdlib.RegisterAll(p)

	// Register extended libraries
	extlibs.RegisterRequestsLibrary(p)
	extlibs.RegisterSecretsLibrary(p)
	extlibs.RegisterSubprocessLibrary(p)
	extlibs.RegisterHTMLParserLibrary(p)
	extlibs.RegisterThreadsLibrary(p)
	extlibs.RegisterOSLibrary(p, []string{})
	extlibs.RegisterPathlibLibrary(p, []string{})
	extlibs.RegisterGlobLibrary(p, []string{})
	extlibs.RegisterWaitForLibrary(p)
	extlibs.RegisterConsoleLibrary(p)
	p.RegisterLibrary(extlibs.YAMLLibrary)

	// Register AI and MCP libraries
	ai.Register(p)
	agent.Register(p)
	agent.RegisterInteract(p)

	// Register MCP library
	mcp.Register(p)
	mcp.RegisterToon(p)

	// Set up on-demand library loading for local .py files
	p.SetOnDemandLibraryCallback(func(p *scriptling.Scriptling, libName string) bool {
		// Try to load from current directory
		filename := libName + ".py"
		content, err := os.ReadFile(filename)
		if err == nil {
			return p.RegisterScriptLibrary(libName, string(content)) == nil
		}
		return false
	})

	file := cmd.GetStringArg("file")
	interactive := cmd.GetBool("interactive")

	// Set up sys.argv with all arguments
	var argv []string
	if file != "" {
		// When running a file, argv[0] is the script name, followed by remaining args
		argv = append([]string{file}, cmd.GetArgs()...)
	} else {
		argv = []string{""}
	}
	extlibs.RegisterSysLibrary(p, argv)

	// Determine execution mode
	if interactive {
		return runInteractive(p)
	} else if file != "" {
		return runFile(p, file)
	} else if !isStdinEmpty() {
		return runStdin(p)
	} else {
		// No input provided, show help
		cmd.ShowHelp()
		return nil
	}
}

func runFile(p *scriptling.Scriptling, filename string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	result, err := p.Eval(string(content))
	// Check for SystemExit to exit with the appropriate code
	if ex, ok := object.AsException(result); ok && ex.IsSystemExit() {
		os.Exit(ex.GetExitCode())
	}
	return err
}

func runStdin(p *scriptling.Scriptling) error {
	content, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read from stdin: %w", err)
	}

	result, err := p.Eval(string(content))
	// Check for SystemExit to exit with the appropriate code
	if ex, ok := object.AsException(result); ok && ex.IsSystemExit() {
		os.Exit(ex.GetExitCode())
	}
	return err
}

func runInteractive(p *scriptling.Scriptling) error {
	fmt.Println("Scriptling Interactive Mode")
	fmt.Println("Type 'exit' or 'quit' to exit")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(">>> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "exit" || line == "quit" {
			break
		}

		if line == "" {
			continue
		}

		// Try to evaluate the line
		result, err := p.Eval(line)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else if result != nil {
			fmt.Printf("%v\n", result.Inspect())
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	return nil
}

func isStdinEmpty() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return true
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}
