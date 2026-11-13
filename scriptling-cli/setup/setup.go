package setup

import (
	"fmt"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/agent"
	"github.com/paularlott/scriptling/extlibs/ai"
	aimemory "github.com/paularlott/scriptling/extlibs/ai/memory"
	scriptlingconsole "github.com/paularlott/scriptling/extlibs/console"
	scriptlingcontainer "github.com/paularlott/scriptling/extlibs/container"
	"github.com/paularlott/scriptling/extlibs/filesystem"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/logging"
	"github.com/paularlott/scriptling/extlibs/markdown"
	scriptlingmcp "github.com/paularlott/scriptling/extlibs/mcp"
	scriptlingmercure "github.com/paularlott/scriptling/extlibs/mercure"
	messagingconsole "github.com/paularlott/scriptling/extlibs/messaging/console"
	"github.com/paularlott/scriptling/extlibs/messaging/discord"
	"github.com/paularlott/scriptling/extlibs/messaging/slack"
	"github.com/paularlott/scriptling/extlibs/messaging/telegram"
	scriptlinggossip "github.com/paularlott/scriptling/extlibs/net/gossip"
	scriptlingmulticast "github.com/paularlott/scriptling/extlibs/net/multicast"
	scriptlingresolve "github.com/paularlott/scriptling/extlibs/net/resolve"
	scriptlingunicast "github.com/paularlott/scriptling/extlibs/net/unicast"
	"github.com/paularlott/scriptling/extlibs/net/websocket"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	scriptlingnomad "github.com/paularlott/scriptling/extlibs/nomad"
	scriptlingoauth "github.com/paularlott/scriptling/extlibs/oauth"
	"github.com/paularlott/scriptling/extlibs/platform"
	"github.com/paularlott/scriptling/extlibs/provision/become"
	provisioncommand "github.com/paularlott/scriptling/extlibs/provision/command"
	provisionfetch "github.com/paularlott/scriptling/extlibs/provision/fetch"
	provisionfile "github.com/paularlott/scriptling/extlibs/provision/file"
	provisionpackage "github.com/paularlott/scriptling/extlibs/provision/packages"
	"github.com/paularlott/scriptling/extlibs/requests"
	"github.com/paularlott/scriptling/extlibs/runtime"
	rthttp "github.com/paularlott/scriptling/extlibs/runtime/http"
	"github.com/paularlott/scriptling/extlibs/runtime/kv"
	"github.com/paularlott/scriptling/extlibs/secret"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	scriptlingsimilarity "github.com/paularlott/scriptling/extlibs/similarity"
	"github.com/paularlott/scriptling/extlibs/subprocess"
	"github.com/paularlott/scriptling/extlibs/sys"
	"github.com/paularlott/scriptling/extlibs/template"
	"github.com/paularlott/scriptling/extlibs/toml"
	"github.com/paularlott/scriptling/extlibs/waitfor"
	"github.com/paularlott/scriptling/extlibs/yaml"
	"github.com/paularlott/scriptling/libloader"
	"github.com/paularlott/scriptling/object"
	scriptlingplugin "github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/stdlib"
)

// AllLibraryNames returns the names of all built-in CLI libraries: every
// standard library (from stdlib.LibraryNames, so none is missed) and the
// extension libraries setup registers.
func AllLibraryNames() []string {
	return append(stdlib.LibraryNames(),
		platform.LibraryName,
		yaml.LibraryName,
		toml.LibraryName,
		requests.LibraryName,
		filesystem.OSLibraryName,
		filesystem.OSPathLibraryName,
		logging.LibraryName,
		runtime.RuntimeLibraryName,
		secret.LibraryName,
		subprocess.LibraryName,
		filesystem.PathlibLibraryName,
		filesystem.GlobLibraryName,
		filesystem.FSLibraryName,
		filesystem.GrepLibraryName,
		filesystem.SedLibraryName,
		scriptlingcontainer.LibraryName,
		scriptlingnomad.LibraryName,
		scriptlingoauth.LibraryName,
		scriptlingmercure.LibraryName,
		waitfor.LibraryName,
		websocket.LibraryName,
		template.HTMLLibraryName,
		template.TextLibraryName,
		markdown.LibraryName,
		scriptlingmulticast.LibraryName,
		scriptlingunicast.LibraryName,
		scriptlinggossip.LibraryName,
		scriptlingresolve.LibraryName,
		provisionfile.LibraryName,
		provisionfetch.LibraryName,
		provisioncommand.LibraryName,
		provisionpackage.LibraryName,
		ai.AILibraryName,
		agent.AgentLibraryName,
		scriptlingsimilarity.LibraryName,
		scriptlingconsole.LibraryName,
		telegram.LibraryName,
		discord.LibraryName,
		slack.LibraryName,
		messagingconsole.LibraryName,
		scriptlingmcp.MCPLibraryName,
		scriptlingmcp.ToonLibraryName,
	)
}

// DisabledSet returns the set of library names disabledLibs turns off. csv
// and xml are also importable under their short alias names, so disabling
// scriptling.csv or scriptling.xml disables the alias too.
func DisabledSet(disabledLibs []string) map[string]bool {
	disabled := make(map[string]bool, len(disabledLibs)+2)
	for _, name := range disabledLibs {
		disabled[name] = true
	}
	for name, alias := range map[string]string{
		stdlib.CSVLibraryName: stdlib.CSVAliasLibraryName,
		stdlib.XMLLibraryName: stdlib.XMLAliasLibraryName,
	} {
		if disabled[name] {
			disabled[alias] = true
		}
	}
	return disabled
}

// rootHelper runs the provision libraries' become=True operations; nil
// refuses them unless scriptling itself runs as root.
var rootHelper *become.Become

// SetBecome lets scripts run provisioning steps as root through b (the CLI
// sets one unless --deny-become). Pass nil to turn it off again. Where the
// subprocess library is disabled, become stays off: running as root through
// sudo is a subprocess.
func SetBecome(b *become.Become) { rootHelper = b }

// Become returns what SetBecome set.
func Become() *become.Become { return rootHelper }

// Scriptling configures a Scriptling instance with the built-in CLI libraries.
// libdirs: Directories for on-demand library loading (first entry is typically the script dir or cwd)
// registerInteract: Whether to register the agent interact library
// allowedPaths: Filesystem path restrictions for os, pathlib, glob, sandbox (nil = no restrictions)
// disabledLibs: Library names to skip registration (nil = all enabled)
// log: Logger instance for the logging library
// containerDenyPrivileged: Refuse scriptling.container run(privileged=True) (false = allowed)
func Scriptling(p *scriptling.Scriptling, libdirs []string, registerInteract bool, allowedPaths []string, disabledLibs []string, secretRegistry *secretprovider.Registry, log logger.Logger, dockerSock, podmanSock string, containerDenyPrivileged bool, netPolicy ...*netsecurity.Config) {
	// A recovered panic reaches the caller as a one-line error (which may be
	// sent to a client); its stack trace is for the operator, at debug level.
	if log != nil {
		p.SetPanicHandler(func(e *scriptling.PanicError) {
			log.Debug("Script panic stack trace", "panic", fmt.Sprint(e.Value), "stack", string(e.Stack))
		})
	}

	disabled := DisabledSet(disabledLibs)
	reg := func(name string, fn func()) {
		if !disabled[name] {
			fn()
		}
	}

	// Register the standard libraries, skipping any that are disabled. They
	// come from stdlib.RegisterAll through a filtering registrar rather than
	// a second list here, which fell behind it (msgpack, copy, enum,
	// dataclasses and typing were missing whenever any library was disabled).
	stdlib.RegisterAll(disabledFilter{p, disabled})
	reg(platform.LibraryName, func() { platform.Register(p) })

	reg(yaml.LibraryName, func() { p.RegisterLibrary(yaml.Library) })
	reg(toml.LibraryName, func() { p.RegisterLibrary(toml.Library) })

	reg(requests.LibraryName, func() {
		// A CA bundle a script names with verify="path" is held to --allowed-paths, as os and pathlib are.
		var policy *netsecurity.Config
		if len(netPolicy) > 0 {
			policy = netPolicy[0]
		}
		requests.RegisterConfigured(p, fssecurity.Config{AllowedPaths: allowedPaths}, policy)
	})
	reg(filesystem.OSLibraryName, func() { filesystem.RegisterOS(p, allowedPaths) })
	reg(logging.LibraryName, func() { logging.Register(p, log) })
	reg(runtime.RuntimeLibraryName, func() { registerRuntime(p, allowedPaths, disabled) })
	reg(secret.LibraryName, func() { secret.Register(p, secretRegistry) })
	reg(subprocess.LibraryName, func() { subprocess.Register(p) })
	reg(filesystem.PathlibLibraryName, func() { filesystem.RegisterPathlib(p, allowedPaths) })
	reg(filesystem.GlobLibraryName, func() { filesystem.RegisterGlob(p, allowedPaths) })
	reg(filesystem.TempfileLibraryName, func() { filesystem.RegisterTempfile(p, allowedPaths) })
	reg(filesystem.ShutilLibraryName, func() { filesystem.RegisterShutil(p, allowedPaths) })
	reg(filesystem.ZipfileLibraryName, func() { filesystem.RegisterZipfile(p, allowedPaths) })
	reg(filesystem.TarfileLibraryName, func() { filesystem.RegisterTarfile(p, allowedPaths) })
	reg(filesystem.FSLibraryName, func() { filesystem.RegisterFS(p, allowedPaths) })
	reg(filesystem.GrepLibraryName, func() { filesystem.RegisterGrep(p, allowedPaths) })
	reg(filesystem.FindLibraryName, func() { filesystem.RegisterFind(p, allowedPaths) })
	reg(filesystem.SedLibraryName, func() { filesystem.RegisterSed(p, allowedPaths) })
	reg(scriptlingcontainer.LibraryName, func() {
		scriptlingcontainer.RegisterWithOptions(p, scriptlingcontainer.Options{
			DockerSock:     dockerSock,
			PodmanSock:     podmanSock,
			Paths:          fssecurity.Config{AllowedPaths: allowedPaths},
			Policy:         firstPolicy(netPolicy),
			DenyPrivileged: containerDenyPrivileged,
		})
	})
	reg(scriptlingnomad.LibraryName, func() { scriptlingnomad.Register(p, netPolicy...) })
	reg(scriptlingoauth.LibraryName, func() {
		var policy *netsecurity.Config
		if len(netPolicy) > 0 {
			policy = netPolicy[0]
		}
		scriptlingoauth.RegisterConfigured(p, fssecurity.Config{AllowedPaths: allowedPaths}, policy)
	})
	reg(scriptlingmercure.LibraryName, func() {
		scriptlingmercure.RegisterConfigured(p, fssecurity.Config{AllowedPaths: allowedPaths}, firstPolicy(netPolicy))
	})
	reg(waitfor.LibraryName, func() { waitfor.RegisterConfigured(p, allowedPaths, netPolicy...) })
	reg(websocket.LibraryName, func() { websocket.Register(p, netPolicy...) })
	reg(template.HTMLLibraryName, func() { template.RegisterHTML(p) })
	reg(template.TextLibraryName, func() { template.RegisterText(p) })
	reg(markdown.LibraryName, func() { markdown.Register(p) })

	reg(scriptlingmulticast.LibraryName, func() { scriptlingmulticast.Register(p, netPolicy...) })
	reg(scriptlingunicast.LibraryName, func() { scriptlingunicast.Register(p, netPolicy...) })
	reg(scriptlinggossip.LibraryName, func() { scriptlinggossip.Register(p, log, netPolicy...) })
	reg(scriptlingresolve.LibraryName, func() { scriptlingresolve.Register(p, scriptResolver(netPolicy)) })
	// become=True runs sudo: only where a script could run it through
	// subprocess anyway.
	root := rootHelper
	if disabled[subprocess.LibraryName] {
		root = nil
	}
	reg(provisionfile.LibraryName, func() {
		provisionfile.RegisterConfigured(p, fssecurity.Config{AllowedPaths: allowedPaths}, provisionfile.WithBecome(root))
	})
	reg(provisionfetch.LibraryName, func() {
		var guard *netsecurity.Guard
		if policy := firstPolicy(netPolicy); policy != nil {
			g, err := netsecurity.NewGuard(policy)
			if err != nil {
				g = netsecurity.FailClosed(err)
			}
			guard = g
		}
		provisionfetch.RegisterConfigured(p, fssecurity.Config{AllowedPaths: allowedPaths}, guard, provisionfetch.WithBecome(root))
	})
	// Running commands is a subprocess, so these are there only where the
	// subprocess library is.
	if !disabled[subprocess.LibraryName] {
		reg(provisioncommand.LibraryName, func() {
			provisioncommand.RegisterConfigured(p, provisioncommand.WithBecome(root))
		})
		reg(provisionpackage.LibraryName, func() {
			provisionpackage.RegisterConfigured(p, provisionpackage.WithBecome(root))
		})
	}

	reg(ai.AILibraryName, func() { ai.Register(p, netPolicy...) })
	reg(aimemory.MemoryLibraryName, func() { aimemory.Register(p, log) })
	reg(agent.AgentLibraryName, func() { agent.Register(p) })
	reg(runtime.RuntimePluginLibraryName, func() { runtime.RegisterPlugin(p) })
	reg(scriptlingsimilarity.LibraryName, func() { scriptlingsimilarity.Register(p) })
	reg(scriptlingconsole.LibraryName, func() { scriptlingconsole.Register(p) })
	if registerInteract && !disabled[agent.InteractLibraryName] {
		agent.RegisterInteract(p)
	}

	reg(telegram.LibraryName, func() { telegram.RegisterConfigured(p, log, allowedPaths, netPolicy...) })
	reg(discord.LibraryName, func() { discord.RegisterConfigured(p, log, allowedPaths, netPolicy...) })
	reg(slack.LibraryName, func() { slack.RegisterConfigured(p, log, allowedPaths, netPolicy...) })
	reg(messagingconsole.LibraryName, func() { messagingconsole.Register(p) })

	reg(scriptlingmcp.MCPLibraryName, func() {
		// A stdio MCP server is a subprocess: allow it only where the
		// subprocess library is allowed.
		scriptlingmcp.RegisterConfigured(p, firstPolicy(netPolicy), !disabled[subprocess.LibraryName])
	})
	reg(scriptlingmcp.ToonLibraryName, func() { scriptlingmcp.RegisterToon(p) })
	if !disabled[scriptlingmcp.MCPLibraryName] {
		scriptlingmcp.RegisterToolHelpers(p)
	}

	if len(libdirs) > 0 {
		loader := libloader.NewMultiFilesystem(libdirs...)
		// With allowed paths set, a library reached through a symlink must
		// land in a library directory or an allowed path. Without them
		// (nil) the loader is unrestricted and pays nothing for the check.
		loader.RestrictTo(allowedPaths)
		p.SetLibraryLoader(loader)
	}
}

// disabledFilter registers libraries on an interpreter except the disabled
// ones.
type disabledFilter struct {
	p        *scriptling.Scriptling
	disabled map[string]bool
}

func (f disabledFilter) RegisterLibrary(lib *object.Library) {
	if !f.disabled[lib.Name()] {
		f.p.RegisterLibrary(lib)
	}
}

// RegisteredLibrary lets sub-libraries registered through the filter attach
// to their parent library (scriptling.runtime's rt.kv, rt.http, ...).
func (f disabledFilter) RegisteredLibrary(name string) (*object.Library, bool) {
	return f.p.RegisteredLibrary(name)
}

// registerRuntime registers scriptling.runtime and its sub-libraries (http,
// kv, sync, sandbox, jsonrpc, mcp; plugin is registered separately). Each
// sub-library honours --allowed-paths where it touches the filesystem
// (static files, kv stores, sandbox exec_file), and can also be turned off on
// its own with --disable-lib.
func registerRuntime(p *scriptling.Scriptling, allowedPaths []string, disabled map[string]bool) {
	r := disabledFilter{p, disabled}
	rthttp.Register(r, allowedPaths)
	kv.Register(r, allowedPaths)
	runtime.RegisterSync(r)
	runtime.RegisterSandbox(r, allowedPaths)
	runtime.RegisterJSONRPC(r)
	runtime.RegisterMCP(r)
	runtime.Register(r)
}

// firstPolicy returns the first policy of a variadic list, or nil.
func firstPolicy(netPolicy []*netsecurity.Config) *netsecurity.Config {
	if len(netPolicy) > 0 {
		return netPolicy[0]
	}
	return nil
}

// Factories configures the global sandbox and background factories.
// Call this once at startup, before any scripts execute.
func Factories(libdirs []string, allowedPaths []string, disabledLibs []string, secretRegistry *secretprovider.Registry, log logger.Logger, dockerSock, podmanSock string, containerDenyPrivileged bool, netPolicy ...*netsecurity.Config) {
	factory := func() runtime.SandboxInstance {
		p := scriptling.New()
		Scriptling(p, libdirs, false, allowedPaths, disabledLibs, secretRegistry, log, dockerSock, podmanSock, containerDenyPrivileged, netPolicy...)
		// Compiled-in plugins (default scriptling builds) must exist in every
		// spun-up environment, not just the main one; a nil manager means
		// no external plugins, which RegisterLibraries handles.
		scriptlingplugin.RegisterLibrariesConfigured(p, nil, scriptlingplugin.PolicyFromSecurity(firstPolicy(netPolicy), allowedPaths), scriptlingplugin.ProcessesAllowed(disabledLibs))
		return p
	}
	runtime.SetSandboxFactory(factory)
	runtime.SetBackgroundFactory(factory)
}

// RegisterSys registers the sys library (sys.argv, sys.platform, etc.) on the
// given Scriptling instance with the provided argv. Pass nil argv to register
// with an empty list. This is a thin wrapper so callers in other packages
// don't need to import extlibs directly.
func RegisterSys(p *scriptling.Scriptling, argv []string) {
	if argv == nil {
		argv = []string{}
	}
	sys.Register(p, argv, nil)
}
