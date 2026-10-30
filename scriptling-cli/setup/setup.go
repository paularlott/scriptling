package setup

import (
	"fmt"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/agent"
	"github.com/paularlott/scriptling/extlibs/ai"
	aimemory "github.com/paularlott/scriptling/extlibs/ai/memory"
	scriptlingconsole "github.com/paularlott/scriptling/extlibs/console"
	scriptlingcontainer "github.com/paularlott/scriptling/extlibs/container"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	scriptlingmcp "github.com/paularlott/scriptling/extlibs/mcp"
	messagingconsole "github.com/paularlott/scriptling/extlibs/messaging/console"
	"github.com/paularlott/scriptling/extlibs/messaging/discord"
	"github.com/paularlott/scriptling/extlibs/messaging/slack"
	"github.com/paularlott/scriptling/extlibs/messaging/telegram"
	scriptlinggossip "github.com/paularlott/scriptling/extlibs/net/gossip"
	scriptlingmulticast "github.com/paularlott/scriptling/extlibs/net/multicast"
	scriptlingresolve "github.com/paularlott/scriptling/extlibs/net/resolve"
	scriptlingunicast "github.com/paularlott/scriptling/extlibs/net/unicast"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	scriptlingnomad "github.com/paularlott/scriptling/extlibs/nomad"
	scriptlingoauth "github.com/paularlott/scriptling/extlibs/oauth"
	provisionfetch "github.com/paularlott/scriptling/extlibs/provision/fetch"
	provisionfile "github.com/paularlott/scriptling/extlibs/provision/file"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	scriptlingsimilarity "github.com/paularlott/scriptling/extlibs/similarity"
	"github.com/paularlott/scriptling/libloader"
	"github.com/paularlott/scriptling/object"
	scriptlingplugin "github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/stdlib"
)

// AllLibraryNames returns the names of all built-in CLI libraries: every
// standard library (from stdlib.LibraryNames, so none is missed) and the
// extended libraries setup registers.
func AllLibraryNames() []string {
	return append(stdlib.LibraryNames(),
		extlibs.YAMLLibraryName,
		extlibs.TOMLLibraryName,
		extlibs.HTMLParserLibraryName,
		extlibs.RequestsLibraryName,
		extlibs.SecretsLibraryName,
		extlibs.OSLibraryName,
		extlibs.OSPathLibraryName,
		extlibs.LoggingLibraryName,
		extlibs.RuntimeLibraryName,
		extlibs.SecretLibraryName,
		extlibs.SubprocessLibraryName,
		extlibs.PathlibLibraryName,
		extlibs.GlobLibraryName,
		extlibs.FSLibraryName,
		extlibs.GrepLibraryName,
		extlibs.SedLibraryName,
		extlibs.ContainerLibraryName,
		extlibs.NomadLibraryName,
		extlibs.OAuthLibraryName,
		extlibs.WaitForLibraryName,
		extlibs.WebSocketLibraryName,
		extlibs.TemplateHTMLLibraryName,
		extlibs.TemplateTextLibraryName,
		extlibs.MarkdownLibraryName,
		extlibs.MulticastLibraryName,
		extlibs.UnicastLibraryName,
		extlibs.GossipLibraryName,
		extlibs.ResolveLibraryName,
		extlibs.FileProvisionLibraryName,
		extlibs.FetchProvisionLibraryName,
		extlibs.AILibraryName,
		extlibs.AgentLibraryName,
		extlibs.SimilarityLibraryName,
		scriptlingconsole.LibraryName,
		telegram.LibraryName,
		discord.LibraryName,
		slack.LibraryName,
		messagingconsole.LibraryName,
		extlibs.MCPLibraryName,
		extlibs.ToonLibraryName,
	)
}

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

	disabled := make(map[string]bool, len(disabledLibs))
	for _, name := range disabledLibs {
		disabled[name] = true
	}
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

	reg(extlibs.YAMLLibraryName, func() { p.RegisterLibrary(extlibs.YAMLLibrary) })
	reg(extlibs.TOMLLibraryName, func() { p.RegisterLibrary(extlibs.TOMLLibrary) })

	reg(extlibs.HTMLParserLibraryName, func() { extlibs.RegisterHTMLParserLibrary(p) })
	reg(extlibs.RequestsLibraryName, func() {
		// A CA bundle a script names with verify="path" is held to --allowed-paths, as os and pathlib are.
		var policy *netsecurity.Config
		if len(netPolicy) > 0 {
			policy = netPolicy[0]
		}
		extlibs.RegisterRequestsLibraryConfigured(p, fssecurity.Config{AllowedPaths: allowedPaths}, policy)
	})
	reg(extlibs.SecretsLibraryName, func() { extlibs.RegisterSecretsLibrary(p) })
	reg(extlibs.OSLibraryName, func() { extlibs.RegisterOSLibrary(p, allowedPaths) })
	reg(extlibs.LoggingLibraryName, func() { extlibs.RegisterLoggingLibrary(p, log) })
	reg(extlibs.RuntimeLibraryName, func() { extlibs.RegisterRuntimeLibraryAll(p, allowedPaths) })
	reg(extlibs.SecretLibraryName, func() { extlibs.RegisterSecretLibrary(p, secretRegistry) })
	reg(extlibs.SubprocessLibraryName, func() { extlibs.RegisterSubprocessLibrary(p) })
	reg(extlibs.PathlibLibraryName, func() { extlibs.RegisterPathlibLibrary(p, allowedPaths) })
	reg(extlibs.GlobLibraryName, func() { extlibs.RegisterGlobLibrary(p, allowedPaths) })
	reg(extlibs.TempfileLibraryName, func() { extlibs.RegisterTempfileLibrary(p, allowedPaths) })
	reg(extlibs.ShutilLibraryName, func() { extlibs.RegisterShutilLibrary(p, allowedPaths) })
	reg(extlibs.ShlexLibraryName, func() { extlibs.RegisterShlexLibrary(p) })
	reg(extlibs.ZipfileLibraryName, func() { extlibs.RegisterZipfileLibrary(p, allowedPaths) })
	reg(extlibs.TarfileLibraryName, func() { extlibs.RegisterTarfileLibrary(p, allowedPaths) })
	reg(extlibs.CsvLibraryName, func() { extlibs.RegisterCsvLibrary(p) })
	reg(extlibs.XmlLibraryName, func() { extlibs.RegisterXmlLibrary(p) })
	reg(extlibs.FSLibraryName, func() { extlibs.RegisterFSLibrary(p, allowedPaths) })
	reg(extlibs.GrepLibraryName, func() { extlibs.RegisterGrepLibrary(p, allowedPaths) })
	reg(extlibs.FindLibraryName, func() { extlibs.RegisterFindLibrary(p, allowedPaths) })
	reg(extlibs.SedLibraryName, func() { extlibs.RegisterSedLibrary(p, allowedPaths) })
	reg(extlibs.ContainerLibraryName, func() {
		scriptlingcontainer.RegisterWithOptions(p, scriptlingcontainer.Options{
			DockerSock:     dockerSock,
			PodmanSock:     podmanSock,
			Paths:          fssecurity.Config{AllowedPaths: allowedPaths},
			Policy:         firstPolicy(netPolicy),
			DenyPrivileged: containerDenyPrivileged,
		})
	})
	reg(extlibs.NomadLibraryName, func() { scriptlingnomad.Register(p, netPolicy...) })
	reg(extlibs.OAuthLibraryName, func() {
		var policy *netsecurity.Config
		if len(netPolicy) > 0 {
			policy = netPolicy[0]
		}
		scriptlingoauth.RegisterConfigured(p, fssecurity.Config{AllowedPaths: allowedPaths}, policy)
	})
	reg(extlibs.WaitForLibraryName, func() { extlibs.RegisterWaitForLibraryConfigured(p, allowedPaths, netPolicy...) })
	reg(extlibs.WebSocketLibraryName, func() { extlibs.RegisterWebSocketLibrary(p, netPolicy...) })
	reg(extlibs.TemplateHTMLLibraryName, func() { extlibs.RegisterTemplateHTMLLibrary(p) })
	reg(extlibs.TemplateTextLibraryName, func() { extlibs.RegisterTemplateTextLibrary(p) })
	reg(extlibs.MarkdownLibraryName, func() { extlibs.RegisterMarkdownLibrary(p) })

	reg(extlibs.MulticastLibraryName, func() { scriptlingmulticast.Register(p, netPolicy...) })
	reg(extlibs.UnicastLibraryName, func() { scriptlingunicast.Register(p, netPolicy...) })
	reg(extlibs.GossipLibraryName, func() { scriptlinggossip.Register(p, log, netPolicy...) })
	reg(extlibs.ResolveLibraryName, func() { scriptlingresolve.Register(p, scriptResolver(netPolicy)) })
	reg(extlibs.FileProvisionLibraryName, func() {
		provisionfile.RegisterConfigured(p, fssecurity.Config{AllowedPaths: allowedPaths})
	})
	reg(extlibs.FetchProvisionLibraryName, func() {
		var guard *netsecurity.Guard
		if policy := firstPolicy(netPolicy); policy != nil {
			g, err := netsecurity.NewGuard(policy)
			if err != nil {
				g = netsecurity.FailClosed(err)
			}
			guard = g
		}
		provisionfetch.RegisterConfigured(p, fssecurity.Config{AllowedPaths: allowedPaths}, guard)
	})

	reg(extlibs.AILibraryName, func() { ai.Register(p, netPolicy...) })
	reg(aimemory.MemoryLibraryName, func() { aimemory.Register(p, log) })
	reg(extlibs.AgentLibraryName, func() { agent.Register(p) })
	reg(extlibs.RuntimePluginLibraryName, func() { extlibs.RegisterRuntimePluginLibrary(p) })
	reg(extlibs.SimilarityLibraryName, func() { scriptlingsimilarity.Register(p) })
	reg(scriptlingconsole.LibraryName, func() { scriptlingconsole.Register(p) })
	if registerInteract && !disabled[extlibs.InteractLibraryName] {
		agent.RegisterInteract(p)
	}

	reg(telegram.LibraryName, func() { telegram.RegisterConfigured(p, log, allowedPaths, netPolicy...) })
	reg(discord.LibraryName, func() { discord.RegisterConfigured(p, log, allowedPaths, netPolicy...) })
	reg(slack.LibraryName, func() { slack.RegisterConfigured(p, log, allowedPaths, netPolicy...) })
	reg(messagingconsole.LibraryName, func() { messagingconsole.Register(p) })

	reg(extlibs.MCPLibraryName, func() {
		// A stdio MCP server is a subprocess: allow it only where the
		// subprocess library is allowed.
		scriptlingmcp.RegisterConfigured(p, firstPolicy(netPolicy), !disabled[extlibs.SubprocessLibraryName])
	})
	reg(extlibs.ToonLibraryName, func() { scriptlingmcp.RegisterToon(p) })
	if !disabled[extlibs.MCPLibraryName] {
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
	factory := func() extlibs.SandboxInstance {
		p := scriptling.New()
		Scriptling(p, libdirs, false, allowedPaths, disabledLibs, secretRegistry, log, dockerSock, podmanSock, containerDenyPrivileged, netPolicy...)
		// Compiled-in plugins (default scriptling builds) must exist in every
		// spun-up environment, not just the main one; a nil manager means
		// no external plugins, which RegisterLibraries handles.
		scriptlingplugin.RegisterLibrariesConfigured(p, nil, scriptlingplugin.PolicyFromSecurity(firstPolicy(netPolicy), allowedPaths), scriptlingplugin.ProcessesAllowed(disabledLibs))
		return p
	}
	extlibs.SetSandboxFactory(factory)
	extlibs.SetBackgroundFactory(factory)
}

// RegisterSys registers the sys library (sys.argv, sys.platform, etc.) on the
// given Scriptling instance with the provided argv. Pass nil argv to register
// with an empty list. This is a thin wrapper so callers in other packages
// don't need to import extlibs directly.
func RegisterSys(p *scriptling.Scriptling, argv []string) {
	if argv == nil {
		argv = []string{}
	}
	extlibs.RegisterSysLibrary(p, argv, nil)
}
