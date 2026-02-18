package mcp

import (
	"os"
	"path/filepath"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/agent"
	"github.com/paularlott/scriptling/extlibs/ai"
	scriptlingfuzzy "github.com/paularlott/scriptling/extlibs/fuzzy"
	scriptlingmcp "github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/stdlib"
)

// SetupScriptling configures a Scriptling instance with all libraries.
// libdir: Optional directory for on-demand library loading (empty = current directory)
// registerInteract: Whether to register the agent interact library
// safeMode: If true, only register safe libraries (no file/network/subprocess access)
// allowedPaths: Filesystem path restrictions for os, pathlib, glob, sandbox (nil = no restrictions)
// log: Logger instance for the logging library
func SetupScriptling(p *scriptling.Scriptling, libdir string, registerInteract bool, safeMode bool, allowedPaths []string, log logger.Logger) {
	// Register all standard libraries (always safe)
	stdlib.RegisterAll(p)

	// Register YAML (safe - pure parsing)
	p.RegisterLibrary(extlibs.YAMLLibrary)

	// Register HTML parser (safe - no external access)
	extlibs.RegisterHTMLParserLibrary(p)
	extlibs.RegisterRequestsLibrary(p)
	extlibs.RegisterSecretsLibrary(p)
	extlibs.RegisterOSLibrary(p, allowedPaths)
	extlibs.RegisterLoggingLibrary(p, log)

	// Register runtime library core (background) and sub-libraries (excluding http)
	extlibs.RegisterRuntimeLibraryAll(p)

	// Set sandbox allowed paths for exec_file restrictions
	if allowedPaths != nil {
		extlibs.SetSandboxAllowedPaths(allowedPaths)
	}

	// Skip dangerous libraries in safe mode
	if !safeMode {
		extlibs.RegisterSubprocessLibrary(p)
		extlibs.RegisterPathlibLibrary(p, allowedPaths)
		extlibs.RegisterGlobLibrary(p, allowedPaths)
		extlibs.RegisterWaitForLibrary(p)
	}

	// Register AI and MCP libraries
	ai.Register(p)
	agent.Register(p)
	scriptlingfuzzy.Register(p)
	if registerInteract {
		extlibs.RegisterConsoleLibrary(p)
		agent.RegisterInteract(p)
	}

	// Register MCP library
	scriptlingmcp.Register(p)
	scriptlingmcp.RegisterToon(p)
	scriptlingmcp.RegisterToolHelpers(p)

	// Set up on-demand library loading for local .py files
	p.SetOnDemandLibraryCallback(func(p *scriptling.Scriptling, libName string) bool {
		var filename string
		if libdir != "" {
			filename = filepath.Join(libdir, libName+".py")
		} else {
			filename = libName + ".py"
		}
		content, err := os.ReadFile(filename)
		if err == nil {
			return p.RegisterScriptLibrary(libName, string(content)) == nil
		}
		return false
	})
}

// SetupFactories configures the global sandbox and background factories.
// Call this once at startup, before any scripts execute.
// The factories create new Scriptling instances with the same library configuration.
func SetupFactories(libdir string, safeMode bool, allowedPaths []string, log logger.Logger) {
	factory := func() extlibs.SandboxInstance {
		p := scriptling.New()
		SetupScriptling(p, libdir, false, safeMode, allowedPaths, log)
		return p
	}
	extlibs.SetSandboxFactory(factory)
	extlibs.SetBackgroundFactory(factory)
}
