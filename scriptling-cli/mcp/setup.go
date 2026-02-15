package mcp

import (
	"os"
	"path/filepath"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/agent"
	"github.com/paularlott/scriptling/extlibs/ai"
	scriptlingmcp "github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/stdlib"
)

// SetupScriptling configures a Scriptling instance with libraries.
// libdir: Optional directory for on-demand library loading (empty = current directory)
// registerInteract: Whether to register the agent interact library
// safeMode: If true, only register safe libraries (no file/network/subprocess access)
// log: Logger instance for the logging library
func SetupScriptling(p *scriptling.Scriptling, libdir string, registerInteract bool, safeMode bool, log logger.Logger) {
	// Register all standard libraries (always safe)
	stdlib.RegisterAll(p)

	// Register YAML (safe - pure parsing)
	p.RegisterLibrary(extlibs.YAMLLibrary)

	// Register HTML parser (safe - no external access)
	extlibs.RegisterHTMLParserLibrary(p)
	extlibs.RegisterRequestsLibrary(p)
	extlibs.RegisterSecretsLibrary(p)
	extlibs.RegisterOSLibrary(p, []string{})
	extlibs.RegisterLoggingLibrary(p, log)

	// Register KV library (always safe - in-memory store)
	extlibs.RegisterKVLibrary(p)

	// Skip dangerous libraries in safe mode
	if !safeMode {
		extlibs.RegisterSubprocessLibrary(p)
		extlibs.RegisterThreadsLibrary(p)
		extlibs.RegisterPathlibLibrary(p, []string{})
		extlibs.RegisterGlobLibrary(p, []string{})
		extlibs.RegisterWaitForLibrary(p)
	}

	// Register AI and MCP libraries
	ai.Register(p)
	agent.Register(p)
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
