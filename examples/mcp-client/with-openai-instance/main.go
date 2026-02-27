// This example demonstrates creating OpenAI and MCP clients in the script,
// attaching the MCP server to the OpenAI client, and using them together.
//
// Prerequisites:
// - LM Studio running on 127.0.0.1:1234
// - A model loaded (e.g., mistralai/ministral-3-3b)
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

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/ai"
	scriptlingmcp "github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/stdlib"
)

func main() {
	// Create scriptling environment
	p := scriptling.New()
	stdlib.RegisterAll(p)

	// Register extended libraries
	extlibs.RegisterRequestsLibrary(p)
	extlibs.RegisterSysLibrary(p, []string{}, nil)
	extlibs.RegisterSecretsLibrary(p)
	extlibs.RegisterSubprocessLibrary(p)
	extlibs.RegisterHTMLParserLibrary(p)
	// Threads library removed - use scriptling.runtime instead
	extlibs.RegisterOSLibrary(p, []string{})
	extlibs.RegisterPathlibLibrary(p, []string{})
	extlibs.RegisterWaitForLibrary(p)

	// Register both libraries
	ai.Register(p)
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
