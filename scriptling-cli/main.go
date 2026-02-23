package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/paularlott/cli"
	"github.com/paularlott/cli/env"
	"github.com/paularlott/cli/tui"
	"github.com/paularlott/logger"
	logslog "github.com/paularlott/logger/slog"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/build"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/lint"
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
		Version:     build.Version,
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
			&cli.BoolFlag{
				Name:    "mcp-exec-script",
				Usage:   "Enable MCP server with script execution tool",
				EnvVars: []string{"SCRIPTLING_MCP_EXEC_SCRIPT"},
			},
			&cli.StringFlag{
				Name:         "bearer-token",
				Usage:        "Bearer token for authentication",
				DefaultValue: "",
				EnvVars:      []string{"SCRIPTLING_BEARER_TOKEN"},
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
			&cli.BoolFlag{
				Name:    "lint",
				Usage:   "Lint script files without executing them",
				Aliases: []string{"l"},
			},
			&cli.StringFlag{
				Name:         "lint-format",
				Usage:        "Output format for lint results (text|json)",
				DefaultValue: "text",
				EnvVars:      []string{"SCRIPTLING_LINT_FORMAT"},
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

	// Check if lint mode is enabled
	if cmd.GetBool("lint") {
		return runLint(cmd)
	}

	// Parse allowed paths
	allowedPaths := parseAllowedPaths(cmd.GetString("allowed-paths"))

	// Create Scriptling interpreter
	p := scriptling.New()

	// Set up all libraries and factories
	libdir := cmd.GetString("libdir")
	mcpcli.SetupFactories(libdir, allowedPaths, globalLogger)
	mcpcli.SetupScriptling(p, libdir, true, allowedPaths, globalLogger)

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
		AllowedPaths: parseAllowedPaths(cmd.GetString("allowed-paths")),
		MCPToolsDir:  cmd.GetString("mcp-tools"),
		MCPExecTool:  cmd.GetBool("mcp-exec-script"),
		TLSCert:      cmd.GetString("tls-cert"),
		TLSKey:       cmd.GetString("tls-key"),
		TLSGenerate:  cmd.GetBool("tls-generate"),
	})
}

// parseAllowedPaths parses a comma-separated list of paths into a slice.
// Returns nil for no restrictions, empty slice for deny all (when paths is "-").
func parseAllowedPaths(paths string) []string {
	if paths == "" {
		return nil
	}
	if paths == "-" {
		return []string{} // Empty slice means deny all
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
	var (
		t         *tui.TUI
		cancel    context.CancelFunc
		runningMu sync.Mutex
	)

	t = tui.New(tui.Config{
		HideHeaders: true,
		StatusRight: "Ctrl+C to exit",
		Commands: []*tui.Command{
			{
				Name:        "exit",
				Description: "Exit interactive mode",
				Handler:     func(_ string) { t.Exit() },
			},
			{
				Name:        "clear",
				Description: "Clear output",
				Handler:     func(_ string) { t.ClearOutput() },
			},
		},
		OnEscape: func() {
			runningMu.Lock()
			if cancel != nil {
				cancel()
			}
			runningMu.Unlock()
		},
		OnSubmit: func(line string) {
			t.AddMessage(tui.RoleUser, line)

			ctx, c := context.WithCancel(context.Background())
			runningMu.Lock()
			cancel = c
			runningMu.Unlock()

			t.StartStreaming()
			t.StartSpinner("Esc to stop")
			p.SetOutputWriter(&streamWriter{t: t})

			go func() {
				defer func() {
					p.SetOutputWriter(nil)
					runningMu.Lock()
					cancel = nil
					runningMu.Unlock()
					c()
					t.StopSpinner()
					t.StreamComplete()
				}()
				result, err := p.EvalWithContext(ctx, line)
				if err != nil {
					if ctx.Err() == nil {
						t.StreamChunk(err.Error())
					}
					return
				}
				if result != nil && result.Inspect() != "None" && !t.IsStreaming() {
					t.AddMessage(tui.RoleAssistant, result.Inspect())
				}
			}()
		},
	})

	t.AddMessage(tui.RoleSystem, tui.Styled(t.Theme().Text, "scriptling")+"\n"+tui.Styled(t.Theme().Primary, "v"+build.Version))

	return t.Run(context.Background())
}

// streamWriter forwards script output chunks to the TUI streaming message.
type streamWriter struct {
	t *tui.TUI
}

func (w *streamWriter) Write(p []byte) (int, error) {
	w.t.StreamChunk(string(p))
	return len(p), nil
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

func runLint(cmd *cli.Command) error {
	format := cmd.GetString("lint-format")
	if format != "text" && format != "json" {
		return fmt.Errorf("invalid value for --lint-format: %s (must be 'text' or 'json')", format)
	}

	file := cmd.GetStringArg("file")

	// Lint from file
	if file != "" {
		result, err := lint.LintFile(file)
		if err != nil {
			return err
		}
		return outputLintResult(result, format)
	}

	// Lint from stdin
	if !isStdinEmpty() {
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		result := lint.Lint(string(content), &lint.Options{Filename: "stdin"})
		return outputLintResult(result, format)
	}

	cmd.ShowHelp()
	return nil
}

func outputLintResult(result *lint.Result, format string) error {
	if format == "json" {
		output, err := formatLintJSON(result)
		if err != nil {
			return fmt.Errorf("failed to format JSON output: %w", err)
		}
		fmt.Println(output)
	} else {
		if result.HasIssues() {
			fmt.Println(result.String())
		} else {
			fmt.Println("No issues found")
		}
	}

	// Exit with error code if there are errors
	if result.HasErrors {
		os.Exit(1)
	}
	return nil
}

func formatLintJSON(result *lint.Result) (string, error) {
	// Simple JSON formatting without external dependencies
	var sb strings.Builder
	sb.WriteString("{\n")
	fmt.Fprintf(&sb, "  \"files_checked\": %d,\n", result.FilesChecked)
	fmt.Fprintf(&sb, "  \"has_errors\": %t,\n", result.HasErrors)
	sb.WriteString("  \"errors\": [")

	if len(result.Errors) > 0 {
		sb.WriteString("\n")
		for i, err := range result.Errors {
			sb.WriteString("    {\n")
			if err.File != "" {
				fmt.Fprintf(&sb, "      \"file\": %q,\n", err.File)
			}
			fmt.Fprintf(&sb, "      \"line\": %d,\n", err.Line)
			if err.Column > 0 {
				fmt.Fprintf(&sb, "      \"column\": %d,\n", err.Column)
			}
			fmt.Fprintf(&sb, "      \"message\": %q,\n", err.Message)
			fmt.Fprintf(&sb, "      \"severity\": %q", err.Severity)
			if err.Code != "" {
				fmt.Fprintf(&sb, ",\n      \"code\": %q", err.Code)
			}
			sb.WriteString("\n    }")
			if i < len(result.Errors)-1 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}
		sb.WriteString("  ")
	}
	sb.WriteString("]\n")
	sb.WriteString("}")
	return sb.String(), nil
}
