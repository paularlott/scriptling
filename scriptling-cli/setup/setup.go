package setup

import (
	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/agent"
	"github.com/paularlott/scriptling/extlibs/ai"
	aimemory "github.com/paularlott/scriptling/extlibs/ai/memory"
	scriptlingconsole "github.com/paularlott/scriptling/extlibs/console"
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

// Scriptling configures a Scriptling instance with the built-in CLI libraries.
// libdirs: Directories for on-demand library loading (first entry is typically the script dir or cwd)
// registerInteract: Whether to register the agent interact library
// allowedPaths: Filesystem path restrictions for os, pathlib, glob, sandbox (nil = no restrictions)
// log: Logger instance for the logging library
func Scriptling(p *scriptling.Scriptling, libdirs []string, registerInteract bool, allowedPaths []string, secretRegistry *secretprovider.Registry, log logger.Logger) {
	// Register all standard libraries.
	stdlib.RegisterAll(p)

	p.RegisterLibrary(extlibs.YAMLLibrary)
	p.RegisterLibrary(extlibs.TOMLLibrary)

	extlibs.RegisterHTMLParserLibrary(p)
	extlibs.RegisterRequestsLibrary(p)
	extlibs.RegisterSecretsLibrary(p)
	extlibs.RegisterOSLibrary(p, allowedPaths)
	extlibs.RegisterLoggingLibrary(p, log)
	extlibs.RegisterRuntimeLibraryAll(p, allowedPaths)
	extlibs.RegisterSecretLibrary(p, secretRegistry)
	extlibs.RegisterSubprocessLibrary(p)
	extlibs.RegisterPathlibLibrary(p, allowedPaths)
	extlibs.RegisterGlobLibrary(p, allowedPaths)
	extlibs.RegisterWaitForLibrary(p)
	extlibs.RegisterWebSocketLibrary(p)

	scriptlingmulticast.Register(p)
	scriptlingunicast.Register(p)
	scriptlinggossip.Register(p, log)

	ai.Register(p)
	aimemory.Register(p, log)
	agent.Register(p)
	scriptlingsimilarity.Register(p)
	scriptlingconsole.Register(p)
	if registerInteract {
		agent.RegisterInteract(p)
	}

	telegram.Register(p, log)
	discord.Register(p, log)
	slack.Register(p, log)
	messagingconsole.Register(p)

	scriptlingmcp.Register(p)
	scriptlingmcp.RegisterToon(p)
	scriptlingmcp.RegisterToolHelpers(p)

	if len(libdirs) > 0 {
		p.SetLibraryLoader(libloader.NewMultiFilesystem(libdirs...))
	}
}

// Factories configures the global sandbox and background factories.
// Call this once at startup, before any scripts execute.
func Factories(libdirs []string, allowedPaths []string, secretRegistry *secretprovider.Registry, log logger.Logger) {
	factory := func() extlibs.SandboxInstance {
		p := scriptling.New()
		Scriptling(p, libdirs, false, allowedPaths, secretRegistry, log)
		return p
	}
	extlibs.SetSandboxFactory(factory)
	extlibs.SetBackgroundFactory(factory)
}
