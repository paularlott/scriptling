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
	"github.com/paularlott/scriptling/scriptling-cli/server"
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
			// Server flags
			&cli.StringFlag{
				Name:         "server",
				Usage:        "Enable HTTP server mode with address (host:port)",
				Aliases:      []string{"S"},
				DefaultValue: "",
				EnvVars:      []string{"SCRIPTLING_SERVER"},
			},
			&cli.StringFlag{
				Name:         "mcp-tools",
				Usage:        "Enable MCP server with tools from directory",
				DefaultValue: "",
				EnvVars:      []string{"SCRIPTLING_MCP_TOOLS"},
			},
			&cli.StringFlag{
				Name:         "bearer-token",
				Usage:        "Bearer token for authentication",
				DefaultValue: "",
				EnvVars:      []string{"SCRIPTLING_BEARER_TOKEN"},
			},
			&cli.StringFlag{
				Name:         "script-mode",
				Usage:        "Script mode: safe or full",
				DefaultValue: "full",
				EnvVars:      []string{"SCRIPTLING_SCRIPT_MODE"},
				ValidateFlag: func(c *cli.Command) error {
					mode := c.GetString("script-mode")
					if mode != "safe" && mode != "full" {
						return fmt.Errorf("invalid value for --script-mode: %s (must be 'safe' or 'full')", mode)
					}
					return nil
				},
			},
			&cli.StringFlag{
				Name:         "allowed-paths",
				Usage:        "Comma-separated list of allowed filesystem paths (restricts os, pathlib, glob, sandbox)",
				DefaultValue: "",
				EnvVars:      []string{"SCRIPTLING_ALLOWED_PATHS"},
			},
			&cli.StringFlag{
				Name:    "tls-cert",
				Usage:   "TLS certificate file",
				EnvVars: []string{"SCRIPTLING_TLS_CERT"},
			},
			&cli.StringFlag{
				Name:    "tls-key",
				Usage:   "TLS key file",
				EnvVars: []string{"SCRIPTLING_TLS_KEY"},
			},
			&cli.BoolFlag{
				Name:  "tls-generate",
				Usage: "Generate self-signed certificate in memory",
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
		PreRun: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			logLevel := cmd.GetString("log-level")
			logFormat := cmd.GetString("log-format")
			globalLogger = logslog.New(logslog.Config{
				Level:  logLevel,
				Format: logFormat,
				Writer: os.Stdout,
			})
			server.Log = globalLogger
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
	// Check if server mode is enabled
	serverAddr := cmd.GetString("server")
	if serverAddr != "" {
		return runServer(ctx, cmd, serverAddr)
	}

	// Parse allowed paths
	allowedPaths := parseAllowedPaths(cmd.GetString("allowed-paths"))

	// Create Scriptling interpreter
	p := scriptling.New()

	// Set up all libraries and factories
	libdir := cmd.GetString("libdir")
	safeMode := cmd.GetString("script-mode") == "safe"
	mcpcli.SetupFactories(libdir, safeMode, allowedPaths, globalLogger)
	mcpcli.SetupScriptling(p, libdir, true, safeMode, allowedPaths, globalLogger)

	file := cmd.GetStringArg("file")
	interactive := cmd.GetBool("interactive")

	// Set up sys.argv with all arguments
	argv := []string{file}
	if file != "" {
		argv = append(argv, cmd.GetArgs()...)
	}
	extlibs.RegisterSysLibrary(p, argv)

	// Release background tasks for script mode
	extlibs.ReleaseBackgroundTasks()

	// Determine execution mode
	if interactive {
		return runInteractive(p)
	}
	if file != "" {
		return runFile(p, file)
	}
	if !isStdinEmpty() {
		return runStdin(p)
	}
	cmd.ShowHelp()
	return nil
}

func runServer(ctx context.Context, cmd *cli.Command, address string) error {
	return server.RunServer(ctx, server.ServerConfig{
		Address:      address,
		ScriptFile:   cmd.GetStringArg("file"),
		LibDir:       cmd.GetString("libdir"),
		BearerToken:  cmd.GetString("bearer-token"),
		ScriptMode:   cmd.GetString("script-mode"),
		AllowedPaths: parseAllowedPaths(cmd.GetString("allowed-paths")),
		MCPToolsDir:  cmd.GetString("mcp-tools"),
		TLSCert:      cmd.GetString("tls-cert"),
		TLSKey:       cmd.GetString("tls-key"),
		TLSGenerate:  cmd.GetBool("tls-generate"),
	})
}

// parseAllowedPaths parses a comma-separated list of paths into a slice
func parseAllowedPaths(paths string) []string {
	if paths == "" {
		return nil
	}
	result := []string{}
	for _, p := range strings.Split(paths, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func runFile(p *scriptling.Scriptling, filename string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filename, err)
	}
	return evalAndCheckExit(p, string(content))
}

func runStdin(p *scriptling.Scriptling) error {
	content, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read from stdin: %w", err)
	}
	return evalAndCheckExit(p, string(content))
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

func evalAndCheckExit(p *scriptling.Scriptling, code string) error {
	result, err := p.Eval(code)
	if ex, ok := object.AsException(result); ok && ex.IsSystemExit() {
		os.Exit(ex.GetExitCode())
	}
	return err
}

func isStdinEmpty() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return true
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}
