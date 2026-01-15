// This example demonstrates wrapping an MCP client in Go and passing it to the script.
// The MCP client is created in Go, wrapped as a scriptling object, and passed to the script as a global variable.
//
// Prerequisites:
// - The scriptling MCP server running on localhost:8080
//   Run: cd ../../mcp && go run main.go
//
// Run with: go run main.go

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	mcplib "github.com/paularlott/mcp"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	scriptlingmcp "github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/stdlib"
)

func main() {
	// Create scriptling environment
	p := scriptling.New()
	stdlib.RegisterAll(p)

	// Register extended libraries
	extlibs.RegisterRequestsLibrary(p)
	extlibs.RegisterSysLibrary(p, []string{})
	extlibs.RegisterSecretsLibrary(p)
	extlibs.RegisterSubprocessLibrary(p)
	extlibs.RegisterHTMLParserLibrary(p)
	extlibs.RegisterThreadsLibrary(p)
	extlibs.RegisterOSLibrary(p, []string{})
	extlibs.RegisterPathlibLibrary(p, []string{})
	extlibs.RegisterWaitForLibrary(p)

	// Create MCP client for the scriptling MCP server
	mcpClient := mcplib.NewClient("http://127.0.0.1:8080/mcp", nil, "scriptling")

	// Wrap the MCP client and set it as a global variable
	wrappedClient := scriptlingmcp.WrapClient(mcpClient)
	if err := p.SetObjectVar("mcp_client", wrappedClient); err != nil {
		log.Fatalf("Failed to set mcp_client variable: %v", err)
	}

	// Register the MCP library
	scriptlingmcp.Register(p)

	// Load script from file
	script, err := os.ReadFile("example.py")
	if err != nil {
		log.Fatalf("Failed to read script: %v", err)
	}

	ctx := context.Background()
	result, err := p.EvalWithContext(ctx, string(script))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else if result != nil {
		fmt.Printf("Result: %s\n", result.Inspect())
	}
}
