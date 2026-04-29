package setup

import (
	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/agent"
	"github.com/paularlott/scriptling/extlibs/ai"
	aimemory "github.com/paularlott/scriptling/extlibs/ai/memory"
	scriptlingconsole "github.com/paularlott/scriptling/extlibs/console"
	scriptlingcontainer "github.com/paularlott/scriptling/extlibs/container"
	scriptlingmcp "github.com/paularlott/scriptling/extlibs/mcp"
	messagingconsole "github.com/paularlott/scriptling/extlibs/messaging/console"
	"github.com/paularlott/scriptling/extlibs/messaging/discord"
	"github.com/paularlott/scriptling/extlibs/messaging/slack"
	"github.com/paularlott/scriptling/extlibs/messaging/telegram"
	scriptlinggossip "github.com/paularlott/scriptling/extlibs/net/gossip"
	scriptlingmulticast "github.com/paularlott/scriptling/extlibs/net/multicast"
	scriptlingunicast "github.com/paularlott/scriptling/extlibs/net/unicast"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	scriptlingsimilarity "github.com/paularlott/scriptling/extlibs/similarity"
	"github.com/paularlott/scriptling/libloader"
	"github.com/paularlott/scriptling/stdlib"
)

// AllLibraryNames returns the names of all built-in CLI libraries.
func AllLibraryNames() []string {
	return []string{
		stdlib.JSONLibraryName,
		stdlib.ReLibraryName,
		stdlib.TimeLibraryName,
		stdlib.DatetimeLibraryName,
		stdlib.MathLibraryName,
		stdlib.Base64LibraryName,
		stdlib.HashlibLibraryName,
		stdlib.RandomLibraryName,
		stdlib.URLLibLibraryName,
		stdlib.URLParseLibraryName,
		stdlib.StringLibraryName,
		stdlib.UUIDLibraryName,
		stdlib.HTMLLibraryName,
		stdlib.StatisticsLibraryName,
		stdlib.FunctoolsLibraryName,
		stdlib.TextwrapLibraryName,
		stdlib.PlatformLibraryName,
		stdlib.ItertoolsLibraryName,
		stdlib.CollectionsLibraryName,
		stdlib.ContextlibLibraryName,
		stdlib.DifflibLibraryName,
		stdlib.IOLibraryName,
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
		extlibs.WaitForLibraryName,
		extlibs.WebSocketLibraryName,
		extlibs.TemplateHTMLLibraryName,
		extlibs.TemplateTextLibraryName,
		extlibs.MulticastLibraryName,
		extlibs.UnicastLibraryName,
		extlibs.GossipLibraryName,
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
	}
}

// Scriptling configures a Scriptling instance with the built-in CLI libraries.
// libdirs: Directories for on-demand library loading (first entry is typically the script dir or cwd)
// registerInteract: Whether to register the agent interact library
// allowedPaths: Filesystem path restrictions for os, pathlib, glob, sandbox (nil = no restrictions)
// disabledLibs: Library names to skip registration (nil = all enabled)
// log: Logger instance for the logging library
func Scriptling(p *scriptling.Scriptling, libdirs []string, registerInteract bool, allowedPaths []string, disabledLibs []string, secretRegistry *secretprovider.Registry, log logger.Logger, dockerSock, podmanSock string) {
	disabled := make(map[string]bool, len(disabledLibs))
	for _, name := range disabledLibs {
		disabled[name] = true
	}
	reg := func(name string, fn func()) {
		if !disabled[name] {
			fn()
		}
	}

	// Register all standard libraries.
	if len(disabled) == 0 {
		stdlib.RegisterAll(p)
	} else {
		reg(stdlib.JSONLibraryName, func() { p.RegisterLibrary(stdlib.JSONLibrary) })
		reg(stdlib.ReLibraryName, func() { p.RegisterLibrary(stdlib.ReLibrary) })
		reg(stdlib.TimeLibraryName, func() { p.RegisterLibrary(stdlib.TimeLibrary) })
		reg(stdlib.DatetimeLibraryName, func() { p.RegisterLibrary(stdlib.DatetimeLibrary) })
		reg(stdlib.MathLibraryName, func() { p.RegisterLibrary(stdlib.MathLibrary) })
		reg(stdlib.Base64LibraryName, func() { p.RegisterLibrary(stdlib.Base64Library) })
		reg(stdlib.HashlibLibraryName, func() { p.RegisterLibrary(stdlib.HashlibLibrary) })
		reg(stdlib.RandomLibraryName, func() { p.RegisterLibrary(stdlib.RandomLibrary) })
		reg(stdlib.URLLibLibraryName, func() { p.RegisterLibrary(stdlib.URLLibLibrary) })
		reg(stdlib.URLParseLibraryName, func() { p.RegisterLibrary(stdlib.URLParseLibrary) })
		reg(stdlib.StringLibraryName, func() { p.RegisterLibrary(stdlib.StringLibrary) })
		reg(stdlib.UUIDLibraryName, func() { p.RegisterLibrary(stdlib.UUIDLibrary) })
		reg(stdlib.HTMLLibraryName, func() { p.RegisterLibrary(stdlib.HTMLLibrary) })
		reg(stdlib.StatisticsLibraryName, func() { p.RegisterLibrary(stdlib.StatisticsLibrary) })
		reg(stdlib.FunctoolsLibraryName, func() { p.RegisterLibrary(stdlib.FunctoolsLibrary) })
		reg(stdlib.TextwrapLibraryName, func() { p.RegisterLibrary(stdlib.TextwrapLibrary) })
		reg(stdlib.PlatformLibraryName, func() { p.RegisterLibrary(stdlib.PlatformLibrary) })
		reg(stdlib.ItertoolsLibraryName, func() { p.RegisterLibrary(stdlib.ItertoolsLibrary) })
		reg(stdlib.CollectionsLibraryName, func() { p.RegisterLibrary(stdlib.CollectionsLibrary) })
		reg(stdlib.ContextlibLibraryName, func() { p.RegisterLibrary(stdlib.ContextlibLibrary) })
		reg(stdlib.DifflibLibraryName, func() { p.RegisterLibrary(stdlib.DifflibLibrary) })
		reg(stdlib.IOLibraryName, func() { p.RegisterLibrary(stdlib.IOLibrary) })
	}

	reg(extlibs.YAMLLibraryName, func() { p.RegisterLibrary(extlibs.YAMLLibrary) })
	reg(extlibs.TOMLLibraryName, func() { p.RegisterLibrary(extlibs.TOMLLibrary) })

	reg(extlibs.HTMLParserLibraryName, func() { extlibs.RegisterHTMLParserLibrary(p) })
	reg(extlibs.RequestsLibraryName, func() { extlibs.RegisterRequestsLibrary(p) })
	reg(extlibs.SecretsLibraryName, func() { extlibs.RegisterSecretsLibrary(p) })
	reg(extlibs.OSLibraryName, func() { extlibs.RegisterOSLibrary(p, allowedPaths) })
	reg(extlibs.LoggingLibraryName, func() { extlibs.RegisterLoggingLibrary(p, log) })
	reg(extlibs.RuntimeLibraryName, func() { extlibs.RegisterRuntimeLibraryAll(p, allowedPaths) })
	reg(extlibs.SecretLibraryName, func() { extlibs.RegisterSecretLibrary(p, secretRegistry) })
	reg(extlibs.SubprocessLibraryName, func() { extlibs.RegisterSubprocessLibrary(p) })
	reg(extlibs.PathlibLibraryName, func() { extlibs.RegisterPathlibLibrary(p, allowedPaths) })
	reg(extlibs.GlobLibraryName, func() { extlibs.RegisterGlobLibrary(p, allowedPaths) })
	reg(extlibs.FSLibraryName, func() { extlibs.RegisterFSLibrary(p, allowedPaths) })
	reg(extlibs.GrepLibraryName, func() { extlibs.RegisterGrepLibrary(p, allowedPaths) })
	reg(extlibs.SedLibraryName, func() { extlibs.RegisterSedLibrary(p, allowedPaths) })
	reg(extlibs.ContainerLibraryName, func() { scriptlingcontainer.Register(p, dockerSock, podmanSock) })
	reg(extlibs.WaitForLibraryName, func() { extlibs.RegisterWaitForLibrary(p) })
	reg(extlibs.WebSocketLibraryName, func() { extlibs.RegisterWebSocketLibrary(p) })
	reg(extlibs.TemplateHTMLLibraryName, func() { extlibs.RegisterTemplateHTMLLibrary(p) })
	reg(extlibs.TemplateTextLibraryName, func() { extlibs.RegisterTemplateTextLibrary(p) })

	reg(extlibs.MulticastLibraryName, func() { scriptlingmulticast.Register(p) })
	reg(extlibs.UnicastLibraryName, func() { scriptlingunicast.Register(p) })
	reg(extlibs.GossipLibraryName, func() { scriptlinggossip.Register(p, log) })

	reg(extlibs.AILibraryName, func() { ai.Register(p) })
	reg(aimemory.MemoryLibraryName, func() { aimemory.Register(p, log) })
	reg(extlibs.AgentLibraryName, func() { agent.Register(p) })
	reg(extlibs.SimilarityLibraryName, func() { scriptlingsimilarity.Register(p) })
	reg(scriptlingconsole.LibraryName, func() { scriptlingconsole.Register(p) })
	if registerInteract && !disabled[extlibs.InteractLibraryName] {
		agent.RegisterInteract(p)
	}

	reg(telegram.LibraryName, func() { telegram.Register(p, log) })
	reg(discord.LibraryName, func() { discord.Register(p, log) })
	reg(slack.LibraryName, func() { slack.Register(p, log) })
	reg(messagingconsole.LibraryName, func() { messagingconsole.Register(p) })

	reg(extlibs.MCPLibraryName, func() { scriptlingmcp.Register(p) })
	reg(extlibs.ToonLibraryName, func() { scriptlingmcp.RegisterToon(p) })
	if !disabled[extlibs.MCPLibraryName] {
		scriptlingmcp.RegisterToolHelpers(p)
	}

	if len(libdirs) > 0 {
		p.SetLibraryLoader(libloader.NewMultiFilesystem(libdirs...))
	}
}

// Factories configures the global sandbox and background factories.
// Call this once at startup, before any scripts execute.
func Factories(libdirs []string, allowedPaths []string, disabledLibs []string, secretRegistry *secretprovider.Registry, log logger.Logger, dockerSock, podmanSock string) {
	factory := func() extlibs.SandboxInstance {
		p := scriptling.New()
		Scriptling(p, libdirs, false, allowedPaths, disabledLibs, secretRegistry, log, dockerSock, podmanSock)
		return p
	}
	extlibs.SetSandboxFactory(factory)
	extlibs.SetBackgroundFactory(factory)
}
