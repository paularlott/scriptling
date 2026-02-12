package mcp

import (
	"os"
	"path/filepath"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/ai"
	"github.com/paularlott/scriptling/extlibs/agent"
	scriptlingmcp "github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/stdlib"
)

// SetupScriptling configures a Scriptling instance with all standard libraries.
// libdir: Optional directory for on-demand library loading (empty = current directory)
// registerInteract: Whether to register the agent interact library
func SetupScriptling(p *scriptling.Scriptling, libdir string, registerInteract bool) {
	// Register all standard libraries
	stdlib.RegisterAll(p)

	// Register extended libraries
	extlibs.RegisterRequestsLibrary(p)
	extlibs.RegisterSecretsLibrary(p)
	extlibs.RegisterSubprocessLibrary(p)
	extlibs.RegisterHTMLParserLibrary(p)
	extlibs.RegisterThreadsLibrary(p)
	extlibs.RegisterOSLibrary(p, []string{})
	extlibs.RegisterPathlibLibrary(p, []string{})
	extlibs.RegisterGlobLibrary(p, []string{})
	extlibs.RegisterWaitForLibrary(p)
	extlibs.RegisterConsoleLibrary(p)
	p.RegisterLibrary(extlibs.YAMLLibrary)

	// Register AI and MCP libraries
	ai.Register(p)
	agent.Register(p)
	if registerInteract {
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
