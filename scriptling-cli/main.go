package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/paularlott/cli"
	"github.com/paularlott/cli/env"
	cli_toml "github.com/paularlott/cli/toml"
	"github.com/paularlott/cli/tui"
	"github.com/paularlott/logger"
	logslog "github.com/paularlott/logger/slog"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/build"
	"github.com/paularlott/scriptling/extlibs"
	scriptlingcontainer "github.com/paularlott/scriptling/extlibs/container"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	"github.com/paularlott/scriptling/object"
	scriptlingplugin "github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/scriptling-cli/bootstrap"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
	"github.com/paularlott/scriptling/scriptling-cli/pluginpack"
	"github.com/paularlott/scriptling/scriptling-cli/secretconfig"
	"github.com/paularlott/scriptling/scriptling-cli/server"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
)

var globalLogger logger.Logger

const (
	configFile = "scriptling.toml"
	configDir  = "scriptling"
)

// mustLoadPolicy loads the --network-policy file. An empty path means no
// policy; a bad file aborts startup rather than running unrestricted.
func mustLoadPolicy(cmd *cli.Command) *netsecurity.Config {
	cfg, err := bootstrap.LoadNetworkPolicy(cmd.GetString("network-policy"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	return cfg
}

func main() {
	env.Load()

	cfgFile := configFile

	cmd := &cli.Command{
		Name:        "scriptling",
		Version:     build.Version,
		Usage:       "Scriptling interpreter",
		Description: "Run Scriptling scripts from files, stdin, or interactively",
		ConfigFile: cli_toml.NewConfigFile(&cfgFile, func() []string {
			paths := []string{"."}
			home, err := os.UserHomeDir()
			if err == nil {
				paths = append(paths, home)
				paths = append(paths, filepath.Join(home, ".config", configDir))
			}
			return paths
		}),
		Commands: []*cli.Command{
			helpCmd(),
			packCmd(),
			unpackCmd(),
			cacheCmd(),
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config",
				Aliases:     []string{"C"},
				Usage:       "Path to configuration file",
				DefaultText: configFile + " in ., $HOME/, $HOME/.config/" + configDir + "/",
				EnvVars:     []string{"SCRIPTLING_CONFIG"},
				AssignTo:    &cfgFile,
				Global:      true,
			},
			&cli.BoolFlag{
				Name:    "interactive",
				Usage:   "Start interactive mode",
				Aliases: []string{"i"},
			},
			&cli.StringSliceFlag{
				Name:       "package",
				Usage:      "Package (.zip) path or URL to load (can be repeated)",
				Aliases:    []string{"p"},
				Global:     true,
				ConfigPath: []string{"packages"},
			},
			&cli.BoolFlag{
				Name:       "insecure",
				Usage:      "Allow self-signed/insecure HTTPS certificates for package URLs",
				Aliases:    []string{"k"},
				ConfigPath: []string{"insecure"},
			},
			&cli.StringFlag{
				Name:       "cache-dir",
				Usage:      "Override default OS cache directory for remote packages",
				Global:     true,
				EnvVars:    []string{"SCRIPTLING_CACHE_DIR"},
				ConfigPath: []string{"cache.dir"},
			},
			&cli.StringFlag{
				Name:    "code",
				Usage:   "Execute inline code string",
				Aliases: []string{"c"},
			},
			&cli.StringSliceFlag{
				Name:       "libpath",
				Usage:      "Additional directories to search for libraries (script dir / cwd is always searched first)",
				Aliases:    []string{"L"},
				Global:     true,
				EnvVars:    []string{"SCRIPTLING_LIBPATH"},
				ConfigPath: []string{"libpath"},
			},
			&cli.StringSliceFlag{
				Name:       "plugin-dir",
				Usage:      "Directory containing plugin executables (can be repeated)",
				Global:     true,
				EnvVars:    []string{"SCRIPTLING_PLUGIN_DIR"},
				ConfigPath: []string{"plugins.dirs"},
			},
			&cli.StringSliceFlag{
				Name:       "plugin",
				Usage:      "Plugin executable to load, optionally with arguments (can be repeated)",
				Global:     true,
				EnvVars:    []string{"SCRIPTLING_PLUGIN"},
				ConfigPath: []string{"plugins.paths"},
			},
			&cli.StringFlag{
				Name:         "log-level",
				Usage:        "Log level (trace|debug|info|warn|error)",
				DefaultValue: "info",
				Global:       true,
				EnvVars:      []string{"SCRIPTLING_LOG_LEVEL"},
				ConfigPath:   []string{"log.level"},
			},
			&cli.StringFlag{
				Name:         "log-format",
				Usage:        "Log format (console|json|null)",
				DefaultValue: "console",
				Global:       true,
				EnvVars:      []string{"SCRIPTLING_LOG_FORMAT"},
				ConfigPath:   []string{"log.format"},
			},
			&cli.StringFlag{
				Name:         "server",
				Usage:        "Enable HTTP server mode with address (host:port)",
				Aliases:      []string{"S"},
				DefaultValue: "",
				EnvVars:      []string{"SCRIPTLING_SERVER"},
				ConfigPath:   []string{"server.address"},
			},
			&cli.BoolFlag{
				Name:    "json-rpc",
				Usage:   "Enable JSON-RPC 2.0 server mode (stdio by default, HTTP /json-rpc with --server)",
				EnvVars: []string{"SCRIPTLING_JSONRPC"},
			},
			&cli.StringFlag{
				Name:         "mcp-tools",
				Usage:        "Run an MCP server exposing tools from this directory (stdio by default, HTTP /mcp with --server)",
				DefaultValue: "",
				EnvVars:      []string{"SCRIPTLING_MCP_TOOLS"},
				ConfigPath:   []string{"mcp.tools"},
			},
			&cli.BoolFlag{
				Name:       "mcp-exec-script",
				Usage:      "Run an MCP server exposing the script execution tool (stdio by default, HTTP /mcp with --server)",
				EnvVars:    []string{"SCRIPTLING_MCP_EXEC_SCRIPT"},
				ConfigPath: []string{"mcp.exec_script"},
			},
			&cli.StringFlag{
				Name:         "mcp-resources",
				Usage:        "Expose MCP resources (and resource templates) from this directory (one .toml + .py per resource)",
				DefaultValue: "",
				EnvVars:      []string{"SCRIPTLING_MCP_RESOURCES"},
				ConfigPath:   []string{"mcp.resources"},
			},
			&cli.StringFlag{
				Name:         "mcp-prompts",
				Usage:        "Expose MCP prompts from this directory (one .toml + .py per prompt)",
				DefaultValue: "",
				EnvVars:      []string{"SCRIPTLING_MCP_PROMPTS"},
				ConfigPath:   []string{"mcp.prompts"},
			},
			&cli.StringFlag{
				Name:         "bearer-token",
				Usage:        "Bearer token for authentication",
				DefaultValue: "",
				EnvVars:      []string{"SCRIPTLING_BEARER_TOKEN"},
				ConfigPath:   []string{"server.bearer_token"},
			},
			&cli.StringFlag{
				Name:         "allowed-paths",
				Usage:        "Comma-separated list of allowed filesystem paths (restricts os, pathlib, glob, sandbox)",
				DefaultValue: "",
				Global:       true,
				EnvVars:      []string{"SCRIPTLING_ALLOWED_PATHS"},
				ConfigPath:   []string{"security.allowed_paths"},
			},
			&cli.StringFlag{
				Name:       "network-policy",
				Usage:      "Path to a TOML network policy file restricting script outbound network access (requests, wait_for, websocket)",
				Global:     true,
				EnvVars:    []string{"SCRIPTLING_NETWORK_POLICY"},
				ConfigPath: []string{"security.network_policy"},
			},
			&cli.BoolFlag{
				Name:       "no-subprocess",
				Usage:      "Do not register the subprocess library",
				EnvVars:    []string{"SCRIPTLING_NO_SUBPROCESS"},
				ConfigPath: []string{"security.no_subprocess"},
			},
			&cli.StringSliceFlag{
				Name:       "disable-lib",
				Usage:      "Disable a built-in library by name (can be repeated)",
				Global:     true,
				EnvVars:    []string{"SCRIPTLING_DISABLE_LIB"},
				ConfigPath: []string{"security.disable_libs"},
			},
			&cli.BoolFlag{
				Name:  "list-libs",
				Usage: "List available built-in libraries and exit",
			},
			&cli.StringFlag{
				Name:         "kv-storage",
				Usage:        "Directory for persistent KV store (empty = in-memory only)",
				DefaultValue: "",
				EnvVars:      []string{"SCRIPTLING_KV_STORAGE"},
				ConfigPath:   []string{"kv.storage"},
			},
			&cli.StringFlag{
				Name:         "docker-host",
				Usage:        "Docker endpoint (Unix socket path, unix://, tcp://, or https://)",
				DefaultValue: scriptlingcontainer.DefaultDockerSocket,
				Global:       true,
				EnvVars:      []string{"DOCKER_HOST"},
				ConfigPath:   []string{"container.docker_host"},
			},
			&cli.StringFlag{
				Name:         "podman-host",
				Usage:        "Podman endpoint (Unix socket path or unix:// URI)",
				DefaultValue: scriptlingcontainer.DefaultPodmanSocket,
				Global:       true,
				EnvVars:      []string{"CONTAINER_HOST"},
				ConfigPath:   []string{"container.podman_host"},
			},
			&cli.StringFlag{
				Name:       "secret-config",
				Usage:      "TOML file that defines host-owned secret provider aliases for scriptling.secret",
				EnvVars:    []string{"SCRIPTLING_SECRET_CONFIG"},
				ConfigPath: []string{"secret.config"},
			},
			&cli.StringFlag{
				Name:       "tls-cert",
				Usage:      "TLS certificate file",
				EnvVars:    []string{"SCRIPTLING_TLS_CERT"},
				ConfigPath: []string{"tls.cert"},
			},
			&cli.StringFlag{
				Name:       "tls-key",
				Usage:      "TLS key file",
				EnvVars:    []string{"SCRIPTLING_TLS_KEY"},
				ConfigPath: []string{"tls.key"},
			},
			&cli.BoolFlag{
				Name:       "tls-generate",
				Usage:      "Generate self-signed certificate in memory",
				ConfigPath: []string{"tls.generate"},
			},
			&cli.StringFlag{
				Name:         "web-root",
				Usage:        "Directory to serve static files from (served when no route matches)",
				DefaultValue: "",
				EnvVars:      []string{"SCRIPTLING_WEB_ROOT"},
				ConfigPath:   []string{"server.web_root"},
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
				ConfigPath:   []string{"lint.format"},
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
			// JSON-RPC stdio mode uses stdout as the protocol stream, so logs
			// must go to stderr to avoid corrupting responses.
			// Stdio protocol modes (JSON-RPC, or MCP without --server) use stdout
			// as the protocol stream, so logs must go to stderr to avoid
			// corrupting responses.
			logWriter := os.Stdout
			mcpStdio := (cmd.GetString("mcp-tools") != "" || cmd.GetString("mcp-resources") != "" || cmd.GetString("mcp-prompts") != "" || cmd.GetBool("mcp-exec-script")) && cmd.GetString("server") == ""
			if cmd.GetBool("json-rpc") || mcpStdio {
				logWriter = os.Stderr
			}

			// Load plugins before --package sources open: fetcher plugins register
			// custom source schemes (knot://…, demo://…) that the package opening
			// below may have to resolve. Explicit --plugin entries load first, so
			// they win over the same executable discovered via --plugin-dir.
			manager, err := loadPluginManager(ctx, cmd.GetStringSlice("plugin-dir"), cmd.GetStringSlice("plugin"))
			if err != nil {
				return ctx, err
			}
			pluginManager = manager
			if err := pluginpack.Register(manager); err != nil {
				return ctx, err
			}

			// Open any --package sources up front: an app bundle (manifest with
			// a serve list) in a stdio protocol mode also needs the pure stdout
			// stream, and Run reuses the opened bundles.
			if packages := cmd.GetStringSlice("package"); len(packages) > 0 {
				app, libs, err := openBundles(packages, cmd.GetBool("insecure"), cmd.GetString("cache-dir"))
				if err != nil {
					return ctx, err
				}
				pendingApp = app
				pendingLibs = libs
				if app != nil && cmd.GetString("server") == "" {
					logWriter = os.Stderr
				}
			}
			format := cmd.GetString("log-format")
			if format == "null" {
				globalLogger = logger.NewNullLogger()
			} else {
				globalLogger = logslog.New(logslog.Config{
					Level:  cmd.GetString("log-level"),
					Format: format,
					Writer: logWriter,
				})
			}
			server.Log = globalLogger
			return ctx, nil
		},
		Run: runScriptling,
	}

	if err := cmd.Execute(context.Background()); err != nil {
		if code, ok := getExitCode(err); ok {
			if err.Error() != "" {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			os.Exit(code)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// pendingApp and pendingLibs hold the --package sources opened in PreRun,
// reused by Run. At most one app bundle (a bundle whose manifest declares
// serve) is allowed; the rest are library bundles.
var (
	pendingApp  *pack.Bundle
	pendingLibs []*pack.Bundle
)

// pluginManager holds the plugins loaded in PreRun (before --package sources
// open, so fetcher plugins can register their source schemes). Run reuses it.
var pluginManager *scriptlingplugin.Manager

// openBundles opens every --package source as a bundle (local dir, local zip,
// or remote zip URL), splitting the single allowed app bundle from library
// bundles.
func openBundles(sources []string, insecure bool, cacheDir string) (app *pack.Bundle, libs []*pack.Bundle, err error) {
	for _, src := range sources {
		b, err := pack.FetchBundle(src, insecure, cacheDir)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load package %s: %w", src, err)
		}
		if len(b.Manifest.Serve) > 0 {
			if app != nil {
				return nil, nil, fmt.Errorf("only one app bundle is allowed: %s and %s both declare serve", app.Source(), b.Source())
			}
			app = b
		} else {
			libs = append(libs, b)
		}
	}
	return app, libs, nil
}

// bundleFlagConflicts captures the path/registration CLI flags that an app
// bundle's manifest owns. Used by rejectBundleFlags.
type bundleFlagConflicts struct {
	File         string   // script file argument
	LibPath      []string // -L
	MCPTools     string
	MCPResources string
	MCPPrompts   string
	WebRoot      string
	Code         string // -c
	Interactive  bool
}

// rejectBundleFlags refuses path/registration flags that manifest.toml owns in
// app-bundle mode.
func rejectBundleFlags(c bundleFlagConflicts) error {
	checks := []struct {
		name string
		set  bool
	}{
		{"file", c.File != "" && !strings.HasPrefix(c.File, "--")},
		{"libpath", len(c.LibPath) > 0},
		{"mcp-tools", c.MCPTools != ""},
		{"mcp-resources", c.MCPResources != ""},
		{"mcp-prompts", c.MCPPrompts != ""},
		{"web-root", c.WebRoot != ""},
		{"code", c.Code != ""},
		{"interactive", c.Interactive},
	}
	for _, c := range checks {
		if c.set {
			return fmt.Errorf("--%s cannot be used with an app bundle; it is configured by the bundle's manifest.toml", c.name)
		}
	}
	return nil
}

func runScriptling(ctx context.Context, cmd *cli.Command) error {
	// App bundle mode: the manifest drives what gets served; deployment flags
	// (--server, --json-rpc, TLS, tokens) pick the transport.
	if pendingApp != nil {
		if err := rejectBundleFlags(bundleFlagConflicts{
			File:         cmd.GetStringArg("file"),
			LibPath:      cmd.GetStringSlice("libpath"),
			MCPTools:     cmd.GetString("mcp-tools"),
			MCPResources: cmd.GetString("mcp-resources"),
			MCPPrompts:   cmd.GetString("mcp-prompts"),
			WebRoot:      cmd.GetString("web-root"),
			Code:         cmd.GetString("code"),
			Interactive:  cmd.GetBool("interactive"),
		}); err != nil {
			return err
		}
		serve := map[string]bool{}
		for _, s := range pendingApp.Manifest.Serve {
			serve[s] = true
		}
		if serverAddr := cmd.GetString("server"); serverAddr != "" {
			return runServer(ctx, cmd, serverAddr)
		}
		if cmd.GetBool("json-rpc") {
			return runJSONRPCServer(ctx, cmd)
		}
		if serve["mcp"] {
			return runMCPStdioServer(ctx, cmd)
		}
		if serve["json-rpc"] {
			return runJSONRPCServer(ctx, cmd)
		}
		return fmt.Errorf("app bundle serves HTTP only; pass --server <addr> to start it")
	}

	if serverAddr := cmd.GetString("server"); serverAddr != "" {
		return runServer(ctx, cmd, serverAddr)
	}

	if cmd.GetBool("json-rpc") {
		return runJSONRPCServer(ctx, cmd)
	}

	// MCP over stdio: enabled by the MCP flags when no HTTP --server is set.
	if cmd.GetString("mcp-tools") != "" || cmd.GetString("mcp-resources") != "" || cmd.GetString("mcp-prompts") != "" || cmd.GetBool("mcp-exec-script") {
		return runMCPStdioServer(ctx, cmd)
	}

	if cmd.GetBool("lint") {
		return runLint(cmd)
	}

	disabledLibs := cmd.GetStringSlice("disable-lib")
	if cmd.GetBool("no-subprocess") && !slices.Contains(disabledLibs, extlibs.SubprocessLibraryName) {
		disabledLibs = append(disabledLibs, extlibs.SubprocessLibraryName)
	}

	if cmd.GetBool("list-libs") {
		disabled := make(map[string]bool, len(disabledLibs))
		for _, name := range disabledLibs {
			disabled[name] = true
		}
		for _, name := range setup.AllLibraryNames() {
			if !disabled[name] {
				fmt.Println(name)
			}
		}
		return nil
	}

	allowedPaths := bootstrap.ParseAllowedPaths(cmd.GetString("allowed-paths"))
	p := scriptling.New()
	secretRegistry, err := loadSecretRegistry(cmd.GetString("secret-config"))
	if err != nil {
		return err
	}

	file := cmd.GetStringArg("file")
	interactive := cmd.GetBool("interactive")

	// A scheme source (knot://scripts/hello) has no meaningful directory;
	// library resolution starts from the working directory instead.
	var baseDir string
	if _, isScheme := pack.SchemeFor(file); isScheme {
		baseDir, err = bootstrap.BaseDir("")
	} else {
		baseDir, err = bootstrap.BaseDir(file)
	}
	if err != nil {
		return err
	}

	kvStoragePath := cmd.GetString("kv-storage")
	if err := extlibs.InitKVStore(kvStoragePath, globalLogger); err != nil {
		return fmt.Errorf("failed to initialize KV store: %w", err)
	}
	defer extlibs.CloseKVStore()

	libDirs := bootstrap.BuildLibDirs(baseDir, cmd.GetStringSlice("libpath"))
	netPolicy, err := bootstrap.LoadNetworkPolicy(cmd.GetString("network-policy"))
	if err != nil {
		return err
	}
	setup.Factories(libDirs, allowedPaths, disabledLibs, secretRegistry, globalLogger, cmd.GetString("docker-host"), cmd.GetString("podman-host"), netPolicy)
	setup.Scriptling(p, libDirs, true, allowedPaths, disabledLibs, secretRegistry, globalLogger, cmd.GetString("docker-host"), cmd.GetString("podman-host"), netPolicy)
	// Plugins were loaded in PreRun (before --package sources opened, so
	// fetcher schemes could register); reuse that manager here.
	defer pluginManager.Close()
	scriptlingplugin.RegisterLibraries(p, pluginManager)

	// Build the library loader from three tiers, added in priority order
	// (the loader searches last-added first, so the app bundle's modules
	// win and explicit --package sources shadow declared ones):
	//
	//  1. packages fetcher plugins declared for automatic attachment —
	//     their modules import without any --package flag;
	//  2. --package sources opened in PreRun (library packs and app
	//     bundles alike);
	//  3. the app bundle, if any, last so its modules win.
	autoBundles, err := declaredLibBundles(cmd)
	if err != nil {
		return err
	}
	var packLoader *pack.Loader
	if len(autoBundles) > 0 || pendingApp != nil || len(pendingLibs) > 0 {
		packLoader = pack.NewLoader()
		packLoader.SetCacheDir(cmd.GetString("cache-dir"))
		for _, b := range autoBundles {
			if err := packLoader.AddBundle(b); err != nil {
				return err
			}
		}
		for _, b := range pendingLibs {
			if err := packLoader.AddBundle(b); err != nil {
				return err
			}
		}
		if pendingApp != nil {
			if err := packLoader.AddBundle(pendingApp); err != nil {
				return err
			}
		}
		go pack.PruneCache(cmd.GetString("cache-dir"), 0) // async, best-effort
		bootstrap.ApplyPackLoader(p, packLoader)
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

	// Wait for outstanding background tasks before exiting so fire-and-forget
	// runtime.background() tasks are not killed mid-flight (their output and
	// logging would be silently lost). Long-running modes — server, JSON-RPC,
	// MCP — return above and never reach this point.
	if code := cmd.GetString("code"); code != "" {
		err := evalAndCheckExit(p, code)
		extlibs.WaitBackgroundTasks()
		return err
	}
	if interactive {
		err := runInteractive(p)
		extlibs.WaitBackgroundTasks()
		return err
	}
	if file != "" {
		var err error
		if _, isScheme := pack.SchemeFor(file); isScheme {
			err = runFetchedScript(p, file)
		} else {
			err = runFile(p, file)
		}
		extlibs.WaitBackgroundTasks()
		return err
	}
	if !isStdinEmpty() {
		err := runStdin(p)
		extlibs.WaitBackgroundTasks()
		return err
	}
	if packLoader != nil {
		entry, found, err := packLoader.ResolveMain()
		if err != nil {
			return err
		}
		if found {
			if entry.Script != nil {
				err := evalAndCheckExit(p, string(entry.Script))
				extlibs.WaitBackgroundTasks()
				return err
			}
			err := evalAndCheckExit(p, fmt.Sprintf("import %s\n%s.%s()", entry.Module, entry.Module, entry.Function))
			extlibs.WaitBackgroundTasks()
			return err
		}
	}
	cmd.ShowHelp()
	return nil
}

func runServer(ctx context.Context, cmd *cli.Command, address string) error {
	// An app bundle started with --server serves every declared protocol
	// over HTTP: MCP at /mcp, JSON-RPC at /json-rpc, HTTP routes at their
	// registered paths. Any non-empty serve list is sufficient.
	if pendingApp != nil && len(pendingApp.Manifest.Serve) == 0 {
		return fmt.Errorf("bundle has no serve protocols declared; add serve = [\"http\"] and/or \"mcp\", \"json-rpc\" to manifest.toml")
	}
	file := cmd.GetStringArg("file")
	argv := cmd.GetArgs()
	if file != "" {
		argv = append([]string{file}, argv...)
	}
	if pendingApp != nil && strings.HasPrefix(file, "--") {
		file = ""
	}
	file, err := resolveScriptFile(file)
	if err != nil {
		return err
	}
	baseDir, err := bootstrap.BaseDir(file)
	if err != nil {
		return err
	}
	secretRegistry, err := loadSecretRegistry(cmd.GetString("secret-config"))
	if err != nil {
		return err
	}
	// Plugins were loaded in PreRun (before --package sources opened, so
	// fetcher schemes could register); reuse that manager.
	defer pluginManager.Close()
	autoBundles, err := declaredLibBundles(cmd)
	if err != nil {
		return err
	}
	libBundles := append(autoBundles, pendingLibs...)
	return server.RunServer(ctx, server.ServerConfig{
		Address:         address,
		ScriptFile:      file,
		LibDirs:         bootstrap.BuildLibDirs(baseDir, cmd.GetStringSlice("libpath")),
		Packages:        cmd.GetStringSlice("package"),
		Bundle:          pendingApp,
		LibBundles:      libBundles,
		Insecure:        cmd.GetBool("insecure"),
		CacheDir:        cmd.GetString("cache-dir"),
		BearerToken:     cmd.GetString("bearer-token"),
		AllowedPaths:    bootstrap.ParseAllowedPaths(cmd.GetString("allowed-paths")),
		NetworkPolicy:   mustLoadPolicy(cmd),
		DisabledLibs:    cmd.GetStringSlice("disable-lib"),
		PluginDirs:      cmd.GetStringSlice("plugin-dir"),
		PluginManager:   pluginManager,
		MCPToolsDir:     cmd.GetString("mcp-tools"),
		MCPResourcesDir: cmd.GetString("mcp-resources"),
		MCPPromptsDir:   cmd.GetString("mcp-prompts"),
		MCPExecTool:     cmd.GetBool("mcp-exec-script"),
		JSONRPC:         cmd.GetBool("json-rpc"),
		KVStoragePath:   cmd.GetString("kv-storage"),
		WebRoot:         cmd.GetString("web-root"),
		SecretRegistry:  secretRegistry,
		DockerSock:      cmd.GetString("docker-host"),
		PodmanSock:      cmd.GetString("podman-host"),
		TLSCert:         cmd.GetString("tls-cert"),
		TLSKey:          cmd.GetString("tls-key"),
		TLSGenerate:     cmd.GetBool("tls-generate"),
		Argv:            argv,
	})
}

func runJSONRPCServer(ctx context.Context, cmd *cli.Command) error {
	file := cmd.GetStringArg("file")
	argv := cmd.GetArgs()
	if file != "" {
		argv = append([]string{file}, argv...)
	}
	if pendingApp != nil && strings.HasPrefix(file, "--") {
		file = ""
	}
	file, err := resolveScriptFile(file)
	if err != nil {
		return err
	}
	baseDir, err := bootstrap.BaseDir(file)
	if err != nil {
		return err
	}
	secretRegistry, err := loadSecretRegistry(cmd.GetString("secret-config"))
	if err != nil {
		return err
	}
	// Plugins were loaded in PreRun (before --package sources opened, so
	// fetcher schemes could register); reuse that manager.
	defer pluginManager.Close()
	autoBundles, err := declaredLibBundles(cmd)
	if err != nil {
		return err
	}
	libBundles := append(autoBundles, pendingLibs...)
	return server.RunJSONRPCServer(ctx, server.ServerConfig{
		ScriptFile:     file,
		LibDirs:        bootstrap.BuildLibDirs(baseDir, cmd.GetStringSlice("libpath")),
		Packages:       cmd.GetStringSlice("package"),
		Bundle:         pendingApp,
		LibBundles:     libBundles,
		Insecure:       cmd.GetBool("insecure"),
		CacheDir:       cmd.GetString("cache-dir"),
		AllowedPaths:   bootstrap.ParseAllowedPaths(cmd.GetString("allowed-paths")),
		NetworkPolicy:  mustLoadPolicy(cmd),
		DisabledLibs:   cmd.GetStringSlice("disable-lib"),
		PluginDirs:     cmd.GetStringSlice("plugin-dir"),
		PluginManager:  pluginManager,
		KVStoragePath:  cmd.GetString("kv-storage"),
		SecretRegistry: secretRegistry,
		DockerSock:     cmd.GetString("docker-host"),
		PodmanSock:     cmd.GetString("podman-host"),
		Argv:           argv,
	})
}

func runMCPStdioServer(ctx context.Context, cmd *cli.Command) error {
	file := cmd.GetStringArg("file")
	argv := cmd.GetArgs()
	if file != "" {
		argv = append([]string{file}, argv...)
	}
	if pendingApp != nil && strings.HasPrefix(file, "--") {
		file = ""
	}
	file, err := resolveScriptFile(file)
	if err != nil {
		return err
	}
	baseDir, err := bootstrap.BaseDir(file)
	if err != nil {
		return err
	}
	secretRegistry, err := loadSecretRegistry(cmd.GetString("secret-config"))
	if err != nil {
		return err
	}
	// Plugins were loaded in PreRun (before --package sources opened, so
	// fetcher schemes could register); reuse that manager.
	defer pluginManager.Close()
	autoBundles, err := declaredLibBundles(cmd)
	if err != nil {
		return err
	}
	libBundles := append(autoBundles, pendingLibs...)
	return server.RunMCPStdioServer(ctx, server.ServerConfig{
		ScriptFile:      file,
		LibDirs:         bootstrap.BuildLibDirs(baseDir, cmd.GetStringSlice("libpath")),
		Packages:        cmd.GetStringSlice("package"),
		Bundle:          pendingApp,
		LibBundles:      libBundles,
		Insecure:        cmd.GetBool("insecure"),
		CacheDir:        cmd.GetString("cache-dir"),
		AllowedPaths:    bootstrap.ParseAllowedPaths(cmd.GetString("allowed-paths")),
		NetworkPolicy:   mustLoadPolicy(cmd),
		DisabledLibs:    cmd.GetStringSlice("disable-lib"),
		PluginDirs:      cmd.GetStringSlice("plugin-dir"),
		PluginManager:   pluginManager,
		MCPToolsDir:     cmd.GetString("mcp-tools"),
		MCPResourcesDir: cmd.GetString("mcp-resources"),
		MCPPromptsDir:   cmd.GetString("mcp-prompts"),
		MCPExecTool:     cmd.GetBool("mcp-exec-script"),
		KVStoragePath:   cmd.GetString("kv-storage"),
		SecretRegistry:  secretRegistry,
		DockerSock:      cmd.GetString("docker-host"),
		PodmanSock:      cmd.GetString("podman-host"),
		Argv:            argv,
	})
}

func loadPluginManager(ctx context.Context, dirs []string, plugins []string) (*scriptlingplugin.Manager, error) {
	manager := scriptlingplugin.NewManager(globalLogger, func(name string, err error) {
		if globalLogger != nil {
			globalLogger.Error("Plugin process exited", "plugin", name, "error", err)
		} else {
			fmt.Fprintf(os.Stderr, "Plugin crashed: %s: %v\n", name, err)
		}
	})
	// Explicit --plugin entries load first. Plugin identity is the resolved
	// executable path, so the same binary found later via --plugin-dir is a
	// no-op — explicit entries (with their arguments) win. Plugins register
	// under the name they declare in their handshake, like --plugin-dir.
	for _, spec := range plugins {
		path, args, err := splitPluginSpec(spec)
		if err != nil {
			return nil, err
		}
		if _, err := manager.LoadPlugin(ctx, path, args); err != nil {
			return nil, err
		}
	}
	for _, dir := range dirs {
		manager.AddDir(dir)
	}
	if err := manager.Load(ctx); err != nil {
		return nil, err
	}
	for _, warning := range manager.Warnings() {
		if globalLogger != nil {
			globalLogger.Warn("Plugin load warning", "warning", warning)
		} else {
			fmt.Fprintf(os.Stderr, "Plugin warning: %s\n", warning)
		}
	}
	return manager, nil
}

// splitPluginSpec splits a --plugin value into an executable path and optional
// arguments. Arguments are separated by spaces; single quotes, double quotes
// and backslash escapes protect paths containing spaces.
func splitPluginSpec(spec string) (string, []string, error) {
	var (
		parts   []string
		cur     strings.Builder
		quote   rune
		escaped bool
	)
	flush := func() {
		if cur.Len() > 0 {
			parts = append(parts, cur.String())
			cur.Reset()
		}
	}
	for _, r := range spec {
		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}
		switch {
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	if quote != 0 {
		return "", nil, fmt.Errorf("unterminated quote in --plugin value: %s", spec)
	}
	if escaped {
		return "", nil, fmt.Errorf("dangling escape in --plugin value: %s", spec)
	}
	flush()
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("empty --plugin value")
	}
	return parts[0], parts[1:], nil
}

func loadSecretRegistry(path string) (*secretprovider.Registry, error) {
	if path == "" {
		return secretprovider.NewRegistry(), nil
	}
	return secretconfig.LoadRegistryFile(path)
}

func runFile(p *scriptling.Scriptling, filename string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filename, err)
	}
	p.SetSourceFile(filename)
	return evalAndCheckExit(p, string(content))
}

// runFetchedScript executes a scheme source (knot://scripts/hello) fetched
// from its plugin. Scripts are always refetched — they run immediately, so
// serving a stale edit would be surprising.
func runFetchedScript(p *scriptling.Scriptling, source string) error {
	content, err := pluginpack.FetchScript(source)
	if err != nil {
		return fmt.Errorf("failed to fetch script %s: %w", source, err)
	}
	p.SetSourceFile(source)
	return evalAndCheckExit(p, string(content))
}

// declaredLibBundles opens the packages fetcher plugins declared for
// automatic attachment, skipping sources passed explicitly via --package
// (their PreRun-opened bundles are used instead).
func declaredLibBundles(cmd *cli.Command) ([]*pack.Bundle, error) {
	skip := map[string]bool{}
	for _, src := range cmd.GetStringSlice("package") {
		skip[src] = true
	}
	return pluginpack.DeclaredBundles(pluginManager, cmd.GetBool("insecure"), cmd.GetString("cache-dir"), skip)
}

// resolveScriptFile maps a scheme source to a local file for the server
// modes, which take a setup-script path. The fetched copy is ephemeral; the
// source refetches on every start.
func resolveScriptFile(file string) (string, error) {
	if _, isScheme := pack.SchemeFor(file); !isScheme {
		return file, nil
	}
	content, err := pluginpack.FetchScript(file)
	if err != nil {
		return "", fmt.Errorf("failed to fetch script %s: %w", file, err)
	}
	tmp, err := os.CreateTemp("", "scriptling-script-*.py")
	if err != nil {
		return "", fmt.Errorf("failed to stage fetched script %s: %w", file, err)
	}
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return "", fmt.Errorf("failed to stage fetched script %s: %w", file, err)
	}
	tmp.Close()
	return tmp.Name(), nil
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
		return exitCodeError{code: ex.GetExitCode()}
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

// readFile reads a local file, used by packCmd --hash.
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
