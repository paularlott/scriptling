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
	"github.com/paularlott/scriptling/extlibs/filesystem"
	scriptlingmcp "github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/extlibs/requests"
	"github.com/paularlott/scriptling/extlibs/subprocess"
	"github.com/paularlott/scriptling/extlibs/sys"
	"github.com/paularlott/scriptling/extlibs/waitfor"
	"github.com/paularlott/scriptling/stdlib"
)

func main() {
	// Create scriptling environment
	p := scriptling.New()
	stdlib.RegisterAll(p)

	// Register extension libraries
	requests.Register(p)
	sys.Register(p, []string{}, nil)
	subprocess.Register(p)
	// Threads library removed - use scriptling.runtime instead
	filesystem.RegisterOS(p, []string{})
	filesystem.RegisterPathlib(p, []string{})
	waitfor.Register(p)

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
