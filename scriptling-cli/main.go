package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	"github.com/paularlott/scriptling/libloader"
	"github.com/paularlott/scriptling/lint"
	"github.com/paularlott/scriptling/object"

	mcpcli "github.com/paularlott/scriptling/scriptling-cli/mcp"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
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
		Commands: []*cli.Command{
			helpCmd(),
			packCmd(),
			unpackCmd(),
			cacheCmd(),
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "interactive",
				Usage:   "Start interactive mode",
				Aliases: []string{"i"},
			},
			&cli.StringSliceFlag{
				Name:    "package",
				Usage:   "Package (.zip) or file (.py) path or URL to load (can be repeated)",
				Aliases: []string{"p"},
			},
			&cli.BoolFlag{
				Name:    "insecure",
				Usage:   "Allow self-signed/insecure HTTPS certificates for package URLs",
				Aliases: []string{"k"},
			},
			&cli.StringFlag{
				Name:    "cache-dir",
				Usage:   "Override default OS cache directory for remote packages",
				EnvVars: []string{"SCRIPTLING_CACHE_DIR"},
			},
			&cli.StringFlag{
				Name:    "code",
				Usage:   "Execute inline code string",
				Aliases: []string{"c"},
			},
			&cli.StringSliceFlag{
				Name:    "libpath",
				Usage:   "Additional directories to search for libraries (script dir / cwd is always searched first)",
				Aliases: []string{"L"},
				Global:  true,
				EnvVars: []string{"SCRIPTLING_LIBPATH"},
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
				Name:         "kv-storage",
				Usage:        "Directory for persistent KV store (empty = in-memory only)",
				DefaultValue: "",
				EnvVars:      []string{"SCRIPTLING_KV_STORAGE"},
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

	// Determine the implicit base dir: script dir, or cwd for interactive/stdin
	file := cmd.GetStringArg("file")
	interactive := cmd.GetBool("interactive")

	baseDir := ""
	if file != "" {
		baseDir = filepath.Dir(file)
	} else {
		baseDir, _ = os.Getwd()
	}

	// Build lib dirs: implicit base dir first, then any --libpath entries
	libDirs := buildLibDirs(baseDir, cmd.GetStringSlice("libpath"))

	// Set up all libraries and factories
	mcpcli.SetupFactories(libDirs, allowedPaths, globalLogger)
	mcpcli.SetupScriptling(p, libDirs, true, allowedPaths, globalLogger)

	// Load packages and wire up loader
	packages := cmd.GetStringSlice("package")
	insecure := cmd.GetBool("insecure")
	var packLoader *pack.Loader
	if len(packages) > 0 {
		packLoader = pack.NewLoader()
		packLoader.SetCacheDir(cmd.GetString("cache-dir"))
		for _, src := range packages {
			if err := packLoader.AddFromPath(src, insecure); err != nil {
				return fmt.Errorf("failed to load package %s: %w", src, err)
			}
		}
		packLoader.SetFallback(nil)
		p.SetLibraryLoader(libloader.NewChain(p.GetLibraryLoader(), packLoader))
	}

	// Set up sys.argv with all arguments
	argv := []string{file}
	if file != "" {
		argv = append(argv, cmd.GetArgs()...)
	}

	// Initialize KV store (memory-only if no path specified)
	kvStoragePath := cmd.GetString("kv-storage")
	if err := extlibs.InitKVStore(kvStoragePath); err != nil {
		return fmt.Errorf("failed to initialize KV store: %w", err)
	}
	defer extlibs.CloseKVStore()

	// Pass os.Stdin when running a file so scripts can read piped data.
	// When running from stdin, stdin is consumed as source so pass nil.
	var stdinReader io.Reader
	if file != "" {
		stdinReader = os.Stdin
	}
	extlibs.RegisterSysLibrary(p, argv, stdinReader)

	// Release background tasks for script mode
	extlibs.ReleaseBackgroundTasks()

	// Inline code
	if code := cmd.GetString("code"); code != "" {
		return evalAndCheckExit(p, code)
	}

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
	// No script: run main entry from last package that defines one
	if packLoader != nil {
		if mod, fn, ok := packLoader.GetMainEntry(); ok {
			return evalAndCheckExit(p, fmt.Sprintf("import %s\n%s.%s()", mod, mod, fn))
		}
	}
	cmd.ShowHelp()
	return nil
}

func runServer(ctx context.Context, cmd *cli.Command, address string) error {
	file := cmd.GetStringArg("file")
	baseDir := ""
	if file != "" {
		baseDir = filepath.Dir(file)
	} else {
		baseDir, _ = os.Getwd()
	}
	return server.RunServer(ctx, server.ServerConfig{
		Address:       address,
		ScriptFile:    file,
		LibDirs:       buildLibDirs(baseDir, cmd.GetStringSlice("libpath")),
		Packages:      cmd.GetStringSlice("package"),
		Insecure:      cmd.GetBool("insecure"),
		CacheDir:      cmd.GetString("cache-dir"),
		BearerToken:   cmd.GetString("bearer-token"),
		AllowedPaths:  parseAllowedPaths(cmd.GetString("allowed-paths")),
		MCPToolsDir:   cmd.GetString("mcp-tools"),
		MCPExecTool:   cmd.GetBool("mcp-exec-script"),
		KVStoragePath: cmd.GetString("kv-storage"),
		TLSCert:       cmd.GetString("tls-cert"),
		TLSKey:        cmd.GetString("tls-key"),
		TLSGenerate:   cmd.GetBool("tls-generate"),
	})
}

// buildLibDirs constructs the ordered list of library search directories.
// baseDir (script dir or cwd) is always first; extra dirs are appended.
// Empty strings are skipped.
func buildLibDirs(baseDir string, extra []string) []string {
	var dirs []string
	if baseDir != "" {
		dirs = append(dirs, baseDir)
	}
	for _, d := range extra {
		if d != "" {
			dirs = append(dirs, d)
		}
	}
	return dirs
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
	p.SetSourceFile(filename)
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

func helpCmd() *cli.Command {
	return &cli.Command{
		Name:  "help",
		Usage: "Show help for a module or function",
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:     "topic",
				Usage:    "Module or module.function to show help for (e.g. mymod or mymod.func)",
				Required: true,
			},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			topic := cmd.GetStringArg("topic")
			allowedPaths := parseAllowedPaths(cmd.GetString("allowed-paths"))
			cwd, _ := os.Getwd()
			libDirs := buildLibDirs(cwd, cmd.GetStringSlice("libpath"))

			p := scriptling.New()
			mcpcli.SetupScriptling(p, libDirs, false, allowedPaths, globalLogger)

			// Load packages if provided
			packages := cmd.GetStringSlice("package")
			if len(packages) > 0 {
				l := pack.NewLoader()
				l.SetCacheDir(cmd.GetString("cache-dir"))
				for _, src := range packages {
					if err := l.AddFromPath(src, cmd.GetBool("insecure")); err != nil {
						return fmt.Errorf("failed to load package %s: %w", src, err)
					}
				}
				l.SetFallback(nil)
				p.SetLibraryLoader(libloader.NewChain(p.GetLibraryLoader(), l))
			}

			// Try importing the full topic first, then progressively shorter prefixes.
			for t := topic; t != ""; {
				if err := p.Import(t); err == nil {
					break
				}
				if i := strings.LastIndex(t, "."); i >= 0 {
					t = t[:i]
				} else {
					break
				}
			}
			_, err := p.Eval(fmt.Sprintf("help(%q)", topic))
			return err
		},
	}
}

func packCmd() *cli.Command {
	return &cli.Command{
		Name:  "pack",
		Usage: "Pack a directory into a package, or manage packages",
		Commands: []*cli.Command{
			manifestCmd(),
			docsCmd(),
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "output",
				Usage:    "Output package path",
				Aliases:  []string{"o"},
				Required: false,
			},
			&cli.BoolFlag{
				Name:    "force",
				Usage:   "Overwrite existing package",
				Aliases: []string{"f"},
			},
			&cli.BoolFlag{
				Name:    "hash",
				Usage:   "Print the sha256 hash of an existing package file",
				Aliases: []string{"H"},
			},
		},
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:     "dir",
				Usage:    "Source directory to pack, or package file when using --hash",
				Required: true,
			},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.GetBool("hash") {
				data, err := os.ReadFile(cmd.GetStringArg("dir"))
				if err != nil {
					return fmt.Errorf("failed to read file: %w", err)
				}
				fmt.Printf("sha256:%s\n", pack.HashBytes(data))
				return nil
			}
			output := cmd.GetString("output")
			if output == "" {
				return fmt.Errorf("--output is required when packing")
			}
			hash, err := pack.Pack(
				cmd.GetStringArg("dir"),
				output,
				cmd.GetBool("force"),
			)
			if err != nil {
				return err
			}
			fmt.Printf("sha256:%s\n", hash)
			return nil
		},
	}
}

func unpackCmd() *cli.Command {
	return &cli.Command{
		Name:  "unpack",
		Usage: "Unpack a package to a directory",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:         "dir",
				Usage:        "Destination directory",
				Aliases:      []string{"d"},
				DefaultValue: ".",
			},
			&cli.BoolFlag{
				Name:    "force",
				Usage:   "Overwrite existing files",
				Aliases: []string{"f"},
			},
			&cli.BoolFlag{
				Name:  "remove",
				Usage: "Remove previously unpacked files instead of extracting",
				Aliases: []string{"r"},
			},
			&cli.BoolFlag{
				Name:  "list",
				Usage: "List contents only, don't extract",
			},
			&cli.BoolFlag{
				Name:    "insecure",
				Usage:   "Allow self-signed/insecure HTTPS certificates",
				Aliases: []string{"k"},
			},
		},
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:     "src",
				Usage:    "Package path or URL",
				Required: true,
			},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.GetBool("remove") {
				return pack.UnpackRemove(cmd.GetStringArg("src"), cmd.GetBool("insecure"), cmd.GetString("dir"))
			}
			return pack.Unpack(cmd.GetStringArg("src"), pack.UnpackOptions{
				DestDir:  cmd.GetString("dir"),
				Force:    cmd.GetBool("force"),
				List:     cmd.GetBool("list"),
				Insecure: cmd.GetBool("insecure"),
			})
		},
	}
}

func manifestCmd() *cli.Command {
	return &cli.Command{
		Name:  "manifest",
		Usage: "Show manifest from a package or source directory",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "json",
				Usage: "Output as JSON",
			},
			&cli.BoolFlag{
				Name:    "insecure",
				Usage:   "Allow self-signed/insecure HTTPS certificates",
				Aliases: []string{"k"},
			},
		},
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:     "src",
				Usage:    "Package path, URL, or source directory",
				Required: true,
			},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			src := cmd.GetStringArg("src")
			insecure := cmd.GetBool("insecure")

			var manifest pack.Manifest
			if pack.IsURL(src) || strings.HasSuffix(src, pack.Extension) {
				data, err := pack.Fetch(src, insecure)
				if err != nil {
					return err
				}
				p, err := pack.Open(bytesReaderAt(data), int64(len(data)))
				if err != nil {
					return err
				}
				manifest = p.Manifest
			} else {
				// Source directory: read manifest.toml directly
				m, err := pack.ReadManifestFromDir(src)
				if err != nil {
					return err
				}
				manifest = m
			}

			if cmd.GetBool("json") {
				out, err := json.MarshalIndent(manifest, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			}

			fmt.Printf("Name:        %s\n", manifest.Name)
			fmt.Printf("Version:     %s\n", manifest.Version)
			if manifest.Description != "" {
				fmt.Printf("Description: %s\n", manifest.Description)
			}
			if manifest.Main != "" {
				fmt.Printf("Main:        %s\n", manifest.Main)
			}
			return nil
		},
	}
}

// bytesReaderAt wraps a byte slice as an io.ReaderAt for use with pack.Open.
type bytesReaderAt []byte

func (b bytesReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(b)) {
		return 0, nil
	}
	return copy(p, b[off:]), nil
}

func cacheCmd() *cli.Command {
	return &cli.Command{
		Name:  "cache",
		Usage: "Manage the package download cache",
		Commands: []*cli.Command{
			{
				Name:  "clear",
				Usage: "Remove all cached remote packages",
				Run: func(ctx context.Context, cmd *cli.Command) error {
					cacheDir := cmd.GetString("cache-dir")
					if err := pack.ClearCache(cacheDir); err != nil {
						return err
					}
					if cacheDir == "" {
						cacheDir, _ = pack.DefaultCacheDir()
					}
					fmt.Printf("Cache cleared: %s\n", cacheDir)
					return nil
				},
			},
		},
	}
}
