package main

import (
	"context"
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
	"github.com/paularlott/scriptling/object"

	mcpcli "github.com/paularlott/scriptling/scriptling-cli/mcp"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
	"github.com/paularlott/scriptling/scriptling-cli/server"
)

var globalLogger logger.Logger

func main() {
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
				Usage:   "Package (.zip) path or URL to load (can be repeated)",
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
			globalLogger = logslog.New(logslog.Config{
				Level:  cmd.GetString("log-level"),
				Format: cmd.GetString("log-format"),
				Writer: os.Stdout,
			})
			server.Log = globalLogger
			return ctx, nil
		},
		Run: runScriptling,
	}

	if err := cmd.Execute(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runScriptling(ctx context.Context, cmd *cli.Command) error {
	if serverAddr := cmd.GetString("server"); serverAddr != "" {
		return runServer(ctx, cmd, serverAddr)
	}

	if cmd.GetBool("lint") {
		return runLint(cmd)
	}

	allowedPaths := parseAllowedPaths(cmd.GetString("allowed-paths"))
	p := scriptling.New()

	file := cmd.GetStringArg("file")
	interactive := cmd.GetBool("interactive")

	baseDir := ""
	if file != "" {
		baseDir = filepath.Dir(file)
	} else {
		baseDir, _ = os.Getwd()
	}

	kvStoragePath := cmd.GetString("kv-storage")
	if err := extlibs.InitKVStore(kvStoragePath); err != nil {
		return fmt.Errorf("failed to initialize KV store: %w", err)
	}
	defer extlibs.CloseKVStore()

	libDirs := buildLibDirs(baseDir, cmd.GetStringSlice("libpath"))
	mcpcli.SetupFactories(libDirs, allowedPaths, globalLogger)
	mcpcli.SetupScriptling(p, libDirs, true, allowedPaths, globalLogger)

	packages := cmd.GetStringSlice("package")
	insecure := cmd.GetBool("insecure")
	var packLoader *pack.Loader
	if len(packages) > 0 {
		go pack.PruneCache(cmd.GetString("cache-dir"), 0) // async, best-effort
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

	argv := []string{file}
	if file != "" {
		argv = append(argv, cmd.GetArgs()...)
	}

	var stdinReader io.Reader
	if file != "" {
		stdinReader = os.Stdin
	}
	extlibs.RegisterSysLibrary(p, argv, stdinReader)
	extlibs.ReleaseBackgroundTasks()

	if code := cmd.GetString("code"); code != "" {
		return evalAndCheckExit(p, code)
	}
	if interactive {
		return runInteractive(p)
	}
	if file != "" {
		return runFile(p, file)
	}
	if !isStdinEmpty() {
		return runStdin(p)
	}
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

type streamWriter struct{ t *tui.TUI }

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

// buildLibDirs constructs the ordered list of library search directories.
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

// parseAllowedPaths parses a comma-separated list of paths.
// Returns nil for no restrictions, empty slice for deny-all (paths == "-").
func parseAllowedPaths(paths string) []string {
	if paths == "" {
		return nil
	}
	if paths == "-" {
		return []string{}
	}
	var result []string
	for _, p := range strings.Split(paths, ",") {
		if p = strings.TrimSpace(p); p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// readFile reads a local file, used by packCmd --hash.
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
