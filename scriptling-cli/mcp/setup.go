package mcp

import (
	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/agent"
	"github.com/paularlott/scriptling/extlibs/ai"
	aimemory "github.com/paularlott/scriptling/extlibs/ai/memory"
	"github.com/paularlott/scriptling/extlibs/console"
	scriptlingfuzzy "github.com/paularlott/scriptling/extlibs/fuzzy"
	scriptlingmcp "github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/libloader"
	"github.com/paularlott/scriptling/stdlib"
)

// SetupScriptling configures a Scriptling instance with all libraries.
// libdirs: Directories for on-demand library loading (first entry is typically the script dir or cwd)
// registerInteract: Whether to register the agent interact library
// allowedPaths: Filesystem path restrictions for os, pathlib, glob, sandbox (nil = no restrictions)
// log: Logger instance for the logging library
func SetupScriptling(p *scriptling.Scriptling, libdirs []string, registerInteract bool, allowedPaths []string, log logger.Logger) {
	// Register all standard libraries
	stdlib.RegisterAll(p)

	// Register YAML
	p.RegisterLibrary(extlibs.YAMLLibrary)

	// Register TOML
	p.RegisterLibrary(extlibs.TOMLLibrary)

	// Register HTML parser
	extlibs.RegisterHTMLParserLibrary(p)
	extlibs.RegisterRequestsLibrary(p)
	extlibs.RegisterSecretsLibrary(p)
	extlibs.RegisterOSLibrary(p, allowedPaths)
	extlibs.RegisterLoggingLibrary(p, log)

	// Register runtime library with sandbox using the same allowed paths
	extlibs.RegisterRuntimeLibraryAll(p, allowedPaths)

	// Register all libraries (use --allowed-paths to restrict file access)
	extlibs.RegisterSubprocessLibrary(p)
	extlibs.RegisterPathlibLibrary(p, allowedPaths)
	extlibs.RegisterGlobLibrary(p, allowedPaths)
	extlibs.RegisterWaitForLibrary(p)

	// Register AI and MCP libraries
	ai.Register(p)
	aimemory.Register(p)
	agent.Register(p)
	scriptlingfuzzy.Register(p)
	if registerInteract {
		console.Register(p)
		agent.RegisterInteract(p)
	}

	// Register MCP library
	scriptlingmcp.Register(p)
	scriptlingmcp.RegisterToon(p)
	scriptlingmcp.RegisterToolHelpers(p)

	// Set up library loading from filesystem
	if len(libdirs) > 0 {
		p.SetLibraryLoader(libloader.NewMultiFilesystem(libdirs...))
	}
}

// SetupFactories configures the global sandbox and background factories.
// Call this once at startup, before any scripts execute.
// The factories create new Scriptling instances with the same library configuration.
func SetupFactories(libdirs []string, allowedPaths []string, log logger.Logger) {
	factory := func() extlibs.SandboxInstance {
		p := scriptling.New()
		SetupScriptling(p, libdirs, false, allowedPaths, log)
		return p
	}
	extlibs.SetSandboxFactory(factory)
	extlibs.SetBackgroundFactory(factory)
}
