// This example demonstrates using the AI library with LM Studio.
// It uses the shared client pattern where the OpenAI client is configured in Go.
//
// Prerequisites:
// - LM Studio running on 127.0.0.1:1234
// - A model loaded (e.g., mistralai/ministral-3-3b)
//
// Run with: go run main.go

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/paularlott/mcp/ai"
	"github.com/paularlott/mcp/ai/openai"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	scriptlingai "github.com/paularlott/scriptling/extlibs/ai"
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

	// Create the AI client for LM Studio
	client, err := ai.NewClient(ai.Config{
		Config: openai.Config{
			BaseURL: "http://127.0.0.1:1234/v1",
			APIKey:  "lm-studio", // LM Studio doesn't require a real API key
		},
		Provider: ai.ProviderOpenAI,
	})
	if err != nil {
		log.Fatalf("Failed to create AI client: %v", err)
	}

	// Wrap the client as a scriptling object and set it as a global variable
	aiClient := scriptlingai.WrapClient(client)
	if err := p.SetObjectVar("ai_client", aiClient); err != nil {
		log.Fatalf("Failed to set ai_client variable: %v", err)
	}

	// Register the AI library
	scriptlingai.Register(p)

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
