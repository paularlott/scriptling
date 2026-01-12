// This example demonstrates creating OpenAI and MCP clients in Go, attaching
// the MCP server to the OpenAI client, and passing both to the script.
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

	mcplib "github.com/paularlott/mcp"
	"github.com/paularlott/mcp/openai"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/ai"
	"github.com/paularlott/scriptling/extlibs"
	scriptlingmcp "github.com/paularlott/scriptling/mcp"
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

	// Create OpenAI client for LM Studio
	client, err := openai.New(openai.Config{
		BaseURL: "http://127.0.0.1:1234/v1",
		APIKey:  "lm-studio",
	})
	if err != nil {
		log.Fatalf("Failed to create OpenAI client: %v", err)
	}

	// Create MCP client for the scriptling MCP server
	mcpClient := mcplib.NewClient("http://127.0.0.1:8080/mcp", nil)

	// Attach MCP server to the OpenAI client
	client.AddRemoteServer("scriptling", mcpClient)

	// Wrap the clients and set them as global variables
	aiClient := ai.WrapClient(client)
	wrappedMCPClient := scriptlingmcp.WrapClient(mcpClient)
	if err := p.SetObjectVar("ai_client", aiClient); err != nil {
		log.Fatalf("Failed to set ai_client variable: %v", err)
	}
	if err := p.SetObjectVar("mcp_client", wrappedMCPClient); err != nil {
		log.Fatalf("Failed to set mcp_client variable: %v", err)
	}

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
