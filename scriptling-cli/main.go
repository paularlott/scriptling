package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/paularlott/cli"
	"github.com/paularlott/cli/env"
	"github.com/paularlott/logger"
	logslog "github.com/paularlott/logger/slog"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"

	mcpcli "github.com/paularlott/scriptling/scriptling-cli/mcp"
)

var globalLogger logger.Logger

func main() {
	// Load .env from the current directory if it exists
	env.Load()

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
			&cli.StringFlag{
				Name:         "libdir",
				Usage:        "Directory to load libraries from",
				DefaultValue: "",
				Global:       true,
				EnvVars:      []string{"SCRIPTLING_LIBDIR"},
			},
			&cli.StringFlag{
				Name:         "log-level",
				Usage:        "Log level (trace|debug|info|warn|error)",
				DefaultValue: "info",
				Global:       true,
				EnvVars:      []string{"SCRIPTLING_LOG_LEVEL"},
			},
			&cli.StringFlag{
				Name:         "log-format",
				Usage:        "Log format (console|json)",
				DefaultValue: "console",
				Global:       true,
				EnvVars:      []string{"SCRIPTLING_LOG_FORMAT"},
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
		Commands: []*cli.Command{
			{
				Name:        "mcp",
				Usage:       "MCP server commands",
				Description: "Start and manage MCP server",
				Commands: []*cli.Command{
					{
						Name:        "serve",
						Usage:       "Start MCP server",
						Description: "Start MCP server to serve tools from a folder",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:         "address",
								Usage:        "Server address",
								DefaultValue: "127.0.0.1:8000",
								EnvVars:      []string{"SCRIPTLING_MCP_ADDRESS"},
							},
							&cli.StringFlag{
								Name:         "tools",
								Usage:        "Tools folder path",
								DefaultValue: "",
								EnvVars:      []string{"SCRIPTLING_MCP_TOOLS"},
							},
							&cli.StringFlag{
								Name:         "bearer-token",
								Usage:        "Bearer token for authentication (optional)",
								DefaultValue: "",
								EnvVars:      []string{"SCRIPTLING_MCP_BEARER_TOKEN"},
							},
							&cli.BoolFlag{
								Name:  "validate",
								Usage: "Validate tools without starting server",
							},
						},
						Run: mcpcli.RunMCPServe,
					},
				},
			},
		},
		PreRun: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			logLevel := cmd.GetString("log-level")
			logFormat := cmd.GetString("log-format")
			globalLogger = logslog.New(logslog.Config{
				Level:  logLevel,
				Format: logFormat,
				Writer: os.Stdout,
			})
			mcpcli.Log = globalLogger
			return ctx, nil
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

	// Set up all libraries
	libdir := cmd.GetString("libdir")
	mcpcli.SetupScriptling(p, libdir, true) // true = register interact library

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
