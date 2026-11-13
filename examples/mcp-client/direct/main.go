// This example demonstrates using the MCP library by creating a client
// directly in the script to connect to an MCP server.
//
// Prerequisites:
// - The scriptling MCP server running (from examples/mcp/)
//   Run: cd examples/mcp && go run main.go
//
// The MCP server provides tools like:
// - execute_code: Execute scriptling code
// - tool_search: Search for tools
// - execute_tool: Execute discovered tools
// - generate_calendar: Generate ASCII calendar
// - generate_password: Generate secure passwords
//
// Run with: go run main.go

package main

import (
	"context"
	"fmt"
	"log"
	"os"

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
