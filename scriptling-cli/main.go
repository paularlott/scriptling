package main

import (
	"context"
	"errors"
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

	cmd := buildRootCommand()

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

// buildRootCommand assembles the CLI command tree. It is separate from main so
// tests can inspect and execute the same tree the binary uses.
func buildRootCommand() *cli.Command {
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
				Usage:      "Plugin executable to load (can be repeated); use --plugin-arg for its arguments",
				Global:     true,
				EnvVars:    []string{"SCRIPTLING_PLUGIN"},
				ConfigPath: []string{"plugins.paths"},
			},
			&cli.StringSliceFlag{
				Name:       "plugin-arg",
				Usage:      "Argument for the preceding --plugin (can be repeated; no quoting needed)",
				Global:     true,
				EnvVars:    []string{"SCRIPTLING_PLUGIN_ARG"},
				ConfigPath: []string{"plugins.args"},
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
			// below may have to resolve.
			if err := startPlugins(ctx, cmd); err != nil {
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
			// The logger can only be built once the app bundle is known (it
			// decides whether logs go to stderr), and bundles can only be opened
			// once fetcher plugins have registered their schemes. Plugins
			// therefore start before the logger exists, so wire it in now —
			// otherwise everything a plugin logs is dropped.
			if pluginManager != nil {
				pluginManager.SetLogger(globalLogger)
			}
			return ctx, nil
		},
		// Every command path releases plugin processes and their schemes, so a
		// subcommand that started plugins (help resolving a knot:// package,
		// say) does not leave them running.
		PostRun: func(ctx context.Context, cmd *cli.Command) error {
			closePlugins()
			return nil
		},
		Run: runScriptling,
	}

	return cmd
}

// pendingApp and pendingLibs hold the --package sources opened in PreRun,
// reused by Run. At most one app bundle (a bundle whose manifest declares
// serve) is allowed; the rest are library bundles.
var (
	pendingApp  *pack.Bundle
	pendingLibs []*pack.Bundle
)

// pluginManager and pluginBridge hold the plugins started in PreRun (before
// --package sources open, so fetcher plugins can register their source
// schemes). Run reuses them. Both are nil when this invocation cannot use
// plugins, so every access goes through the helpers below.
var (
	pluginManager *scriptlingplugin.Manager
	pluginBridge  *pluginpack.Bridge
)

// pluginFreeCommands name top-level commands whose whole subtree never
// evaluates a script and never resolves a package source, so discovering
// plugins for them is pure cost.
var pluginFreeCommands = map[string]bool{
	"pack":   true,
	"unpack": true,
	"cache":  true,
}

// pluginDiscoveryWanted reports whether this invocation should actually start
// the configured plugins. The manager itself is always created — it spawns
// nothing, and scripts reach it through scriptling.plugin at runtime — but
// scanning --plugin-dir and launching --plugin executables costs real
// processes. Linting parses without evaluating, and the package command trees
// never resolve a scheme source, so neither needs them.
func pluginDiscoveryWanted(cmd *cli.Command) bool {
	if pluginFreeCommands[topLevelCommandName(cmd)] {
		return false
	}
	if cmd.GetBool("lint") || cmd.GetBool("list-libs") {
		return false
	}
	return true
}

// topLevelCommandName returns the name of the top-level command the matched
// command belongs to, or "" when the root itself matched.
//
// PreRun receives the leaf command, so `cache clear` arrives as "clear" and
// `pack manifest` as "manifest". Testing those names against
// pluginFreeCommands would miss them, which is why the top-level ancestor is
// resolved here. The cli library does not export its command chain, so the
// ancestor is found by searching the command tree the root owns — the same
// pointers the library matched against.
func topLevelCommandName(cmd *cli.Command) string {
	return topLevelCommandNameIn(cmd.GetRootCmd(), cmd)
}

// topLevelCommandNameIn is topLevelCommandName with the root supplied, so it can
// be exercised without executing the command.
func topLevelCommandNameIn(root, cmd *cli.Command) string {
	if cmd == root {
		return ""
	}
	for _, top := range root.Commands {
		if commandSubtreeContains(top, cmd) {
			return top.Name
		}
	}
	// Not reachable from the root's tree: fall back to the command's own name.
	return cmd.Name
}

// commandSubtreeContains reports whether target is parent or one of its
// descendants, at any depth.
func commandSubtreeContains(parent, target *cli.Command) bool {
	if parent == target {
		return true
	}
	for _, child := range parent.Commands {
		if commandSubtreeContains(child, target) {
			return true
		}
	}
	return false
}

// startPlugins creates the plugin manager and bridges its fetchers into the
// pack scheme registry, leaving both in the package globals for Run to reuse.
// The manager always exists so scriptling.plugin.load works even when no
// plugins were configured; only discovery is conditional. The host security
// policy (network + allowed paths) rides the handshake to every plugin.

func startPlugins(ctx context.Context, cmd *cli.Command) error {
	var dirs, plugins, pluginArgs []string
	if pluginDiscoveryWanted(cmd) {
		dirs = cmd.GetStringSlice("plugin-dir")
		plugins = cmd.GetStringSlice("plugin")
		pluginArgs = cmd.GetStringSlice("plugin-arg")
	}
	netPolicy, err := bootstrap.LoadNetworkPolicy(cmd.GetString("network-policy"))
	if err != nil {
		return err
	}
	policy := scriptlingplugin.PolicyFromSecurity(netPolicy, bootstrap.ParseAllowedPaths(cmd.GetString("allowed-paths")))
	manager, err := loadPluginManager(ctx, dirs, plugins, pluginArgs, policy)
	if err != nil {
		return err
	}
	bridge := pluginpack.New(pluginpack.Options{
		Manager: manager,
		Context: ctx,
	})
	if err := bridge.Register(); err != nil {
		_ = manager.Close()
		return err
	}
	pluginManager = manager
	pluginBridge = bridge
	return nil
}

// closePlugins releases the bridge's schemes and shuts down the plugin
// processes. Safe to call when no plugins were started, and safe to call twice.
func closePlugins() {
	if pluginBridge != nil {
		_ = pluginBridge.Close()
		pluginBridge = nil
	}
	if pluginManager != nil {
		_ = pluginManager.Close()
		pluginManager = nil
	}
}

// fetchScriptSource reads a scheme source that is itself a script. It reports a
// clear error when no plugin serves the scheme, rather than letting the source
// fall through to the local-file path.
func fetchScriptSource(ctx context.Context, source string) ([]byte, error) {
	if pluginBridge == nil {
		scheme, _ := pack.SchemeSyntax(source)
		return nil, withPluginFlagHint(fmt.Errorf("%w %q for %s: load the plugin that serves it", pack.ErrUnknownScheme, scheme, source))
	}
	content, err := pluginBridge.FetchScript(ctx, source)
	if err != nil {
		return nil, withPluginFlagHint(fmt.Errorf("failed to fetch script %s: %w", source, err))
	}
	return content, nil
}

// withPluginFlagHint names the flags that load a plugin, for errors caused by a
// scheme no loaded plugin serves. The pack and pluginpack packages deliberately
// stay audience-neutral — an embedding host loads plugins its own way — so the
// CLI-specific advice is added here, where the flags exist.
func withPluginFlagHint(err error) error {
	if err == nil || !errors.Is(err, pack.ErrUnknownScheme) {
		return err
	}
	// The library message ends with "load the plugin that serves it", so this
	// completes the sentence rather than adding a second parenthetical.
	return fmt.Errorf("%w with --plugin or --plugin-dir", err)
}

// isFetchedScript reports whether file is a <scheme>:// source rather than a
// path. Scheme syntax alone decides, so a missing plugin produces a "no plugin
// provides that scheme" error instead of "no such file or directory".
func isFetchedScript(file string) bool {
	_, ok := pack.SchemeSyntax(file)
	return ok
}

// openBundles opens every --package source as a bundle (local dir, local zip,
// or remote zip URL), splitting the single allowed app bundle from library
// bundles.
//
// --package does not accept plugin scheme sources (knot://libs). A fetcher
// plugin exposes its packages by declaring them (DeclarePackage), and the host
// attaches those automatically — so routing a scheme through --package would
// only duplicate that, with a second way to spell the same thing. The flag
// keeps its single meaning: a .zip, a directory, or a URL.
func openBundles(sources []string, insecure bool, cacheDir string) (app *pack.Bundle, libs []*pack.Bundle, err error) {
	for _, src := range sources {
		if _, ok := pack.SchemeSyntax(src); ok {
			return nil, nil, fmt.Errorf("--package does not take plugin scheme sources like %s: a plugin's library attaches automatically with the plugin itself, so the flag is not needed", src)
		}
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
	if isFetchedScript(file) {
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
	// Plugins were started in PreRun (before --package sources opened, so
	// fetcher schemes could register); reuse that manager here. PostRun
	// releases them however this command exits. The manager always exists, so
	// scriptling.plugin.load works even with no plugins configured.
	scriptlingplugin.RegisterLibraries(p, pluginManager, scriptlingplugin.PolicyFromSecurity(netPolicy, allowedPaths))

	// Build the library loader from three tiers, added in priority order
	// (the loader searches last-added first, so the app bundle's modules
	// win and explicit --package sources shadow declared ones):
	//
	//  1. packages fetcher plugins declared for automatic attachment —
	//     their modules import without any --package flag;
	//  2. --package sources opened in PreRun (library packs and app
	//     bundles alike);
	//  3. the app bundle, if any, last so its modules win.
	autoBundles, err := declaredLibBundles()
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
		// One-shot runs get the same read-only package file access the server
		// modes have, so a script can read assets (images, fonts, docs) from
		// its --package bundles and from plugin libraries alike.
		pack.RegisterPackageLibrary(p, packLoader)
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
		if isFetchedScript(file) {
			err = runFetchedScript(ctx, p, file)
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
	scriptPath, scriptSource, scriptName, err := setupScript(ctx, file)
	if err != nil {
		return err
	}
	baseDir, err := bootstrap.BaseDir(scriptPath)
	if err != nil {
		return err
	}
	secretRegistry, err := loadSecretRegistry(cmd.GetString("secret-config"))
	if err != nil {
		return err
	}
	// Plugins were started in PreRun (before --package sources opened, so
	// fetcher schemes could register); reuse that manager. PostRun releases
	// them however this command exits.
	autoBundles, err := declaredLibBundles()
	if err != nil {
		return err
	}
	libBundles := append(autoBundles, pendingLibs...)
	return server.RunServer(ctx, server.ServerConfig{
		Address:         address,
		ScriptFile:      scriptPath,
		ScriptSource:    scriptSource,
		ScriptName:      scriptName,
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
	scriptPath, scriptSource, scriptName, err := setupScript(ctx, file)
	if err != nil {
		return err
	}
	baseDir, err := bootstrap.BaseDir(scriptPath)
	if err != nil {
		return err
	}
	secretRegistry, err := loadSecretRegistry(cmd.GetString("secret-config"))
	if err != nil {
		return err
	}
	// Plugins were started in PreRun (before --package sources opened, so
	// fetcher schemes could register); reuse that manager. PostRun releases
	// them however this command exits.
	autoBundles, err := declaredLibBundles()
	if err != nil {
		return err
	}
	libBundles := append(autoBundles, pendingLibs...)
	return server.RunJSONRPCServer(ctx, server.ServerConfig{
		ScriptFile:     scriptPath,
		ScriptSource:   scriptSource,
		ScriptName:     scriptName,
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
	scriptPath, scriptSource, scriptName, err := setupScript(ctx, file)
	if err != nil {
		return err
	}
	baseDir, err := bootstrap.BaseDir(scriptPath)
	if err != nil {
		return err
	}
	secretRegistry, err := loadSecretRegistry(cmd.GetString("secret-config"))
	if err != nil {
		return err
	}
	// Plugins were started in PreRun (before --package sources opened, so
	// fetcher schemes could register); reuse that manager. PostRun releases
	// them however this command exits.
	autoBundles, err := declaredLibBundles()
	if err != nil {
		return err
	}
	libBundles := append(autoBundles, pendingLibs...)
	return server.RunMCPStdioServer(ctx, server.ServerConfig{
		ScriptFile:      scriptPath,
		ScriptSource:    scriptSource,
		ScriptName:      scriptName,
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

func loadPluginManager(ctx context.Context, dirs []string, plugins []string, pluginArgs []string, policy ...*scriptlingplugin.Policy) (*scriptlingplugin.Manager, error) {
	specs, err := resolvePluginSpecs(plugins, pluginArgs)
	if err != nil {
		return nil, err
	}
	loadSpecs := make([]scriptlingplugin.PluginSpec, len(specs))
	for i, spec := range specs {
		loadSpecs[i] = scriptlingplugin.PluginSpec{Path: spec.Path, Args: spec.Args}
	}
	manager := scriptlingplugin.NewManager(globalLogger, func(name string, err error) {
		if globalLogger != nil {
			globalLogger.Error("Plugin process exited", "plugin", name, "error", err)
		} else {
			fmt.Fprintf(os.Stderr, "Plugin crashed: %s: %v\n", name, err)
		}
	})
	if len(policy) > 0 {
		manager.SetPolicy(policy[0])
	}
	// Explicit --plugin entries load first, in parallel (capped). Plugin
	// identity is the resolved executable path, so the same binary found
	// later via --plugin-dir is a no-op — explicit entries (with their
	// arguments) win. Plugins register under the name they declare in their
	// handshake, in --plugin order, like --plugin-dir.
	if err := manager.LoadPlugins(ctx, loadSpecs); err != nil {
		_ = manager.Close()
		return nil, err
	}
	for _, dir := range dirs {
		manager.AddDir(dir)
	}
	if err := manager.Load(ctx); err != nil {
		_ = manager.Close()
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

// pluginSpec is a resolved plugin: an executable path and its arguments.
type pluginSpec struct {
	Path string
	Args []string
}

// resolvePluginSpecs pairs --plugin executable paths with --plugin-arg values.
//
// A --plugin value is taken literally, so paths containing spaces need no
// quoting. Arguments come from --plugin-arg, in the order given:
//
//	--plugin /opt/my app/knot --plugin-arg serve --plugin-arg --alias=testing
//
// With exactly one --plugin, bare --plugin-arg values belong to it. With
// several, each argument must name its plugin as <plugin>=<arg>, where
// <plugin> is the full path or the executable's base name:
//
//	--plugin /usr/local/bin/knot --plugin /usr/local/bin/other \
//	  --plugin-arg knot=serve --plugin-arg other=--port=8080
//
// A value whose text before "=" matches no --plugin is treated as a bare
// argument, so ordinary flags like --alias=testing pass through unqualified.
func resolvePluginSpecs(plugins, args []string) ([]pluginSpec, error) {
	specs := make([]pluginSpec, 0, len(plugins))
	byKey := map[string][]int{} // path and base name → indexes into specs
	for _, path := range plugins {
		if strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("empty --plugin value")
		}
		i := len(specs)
		specs = append(specs, pluginSpec{Path: path})
		byKey[path] = append(byKey[path], i)
		if base := filepath.Base(path); base != path {
			byKey[base] = append(byKey[base], i)
		}
	}

	for _, arg := range args {
		key, value, qualified := strings.Cut(arg, "=")
		targets, known := byKey[key]
		if !qualified || !known {
			// Not a <plugin>=<arg> qualifier: a bare argument for the sole plugin.
			if len(specs) == 0 {
				return nil, fmt.Errorf("--plugin-arg %s given without any --plugin", arg)
			}
			if len(specs) > 1 {
				return nil, fmt.Errorf("--plugin-arg %s is ambiguous with %d --plugin entries: qualify it as <plugin>=<arg>, e.g. %s=%s",
					arg, len(specs), filepath.Base(specs[0].Path), arg)
			}
			specs[0].Args = append(specs[0].Args, arg)
			continue
		}
		if len(targets) > 1 {
			return nil, fmt.Errorf("--plugin-arg %s is ambiguous: %d --plugin entries match %q, qualify it with the full path", arg, len(targets), key)
		}
		specs[targets[0]].Args = append(specs[targets[0]].Args, value)
	}
	return specs, nil
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
func runFetchedScript(ctx context.Context, p *scriptling.Scriptling, source string) error {
	content, err := fetchScriptSource(ctx, source)
	if err != nil {
		return err
	}
	p.SetSourceFile(source)
	return evalAndCheckExit(p, string(content))
}

// declaredLibBundles opens the packages fetcher plugins declared for
// automatic attachment, skipping sources passed explicitly via --package
// (their PreRun-opened bundles are used instead).
func declaredLibBundles() ([]*pack.Bundle, error) {
	if pluginBridge == nil {
		return nil, nil
	}
	bundles, err := pluginBridge.Bundles()
	return bundles, withPluginFlagHint(err)
}

// setupScript resolves the server modes' setup script argument. A plain path is
// passed through as a path; a scheme source is fetched and returned as source
// text, so nothing is ever staged to a temporary file.
func setupScript(ctx context.Context, file string) (path string, source []byte, name string, err error) {
	if !isFetchedScript(file) {
		return file, nil, "", nil
	}
	content, err := fetchScriptSource(ctx, file)
	if err != nil {
		return "", nil, "", err
	}
	return "", content, file, nil
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
