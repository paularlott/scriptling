// This example demonstrates using the AI library by creating a client instance
// directly from the script (without pre-configuring a shared client in Go).
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

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/ai"
	"github.com/paularlott/scriptling/extlibs"
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

	// Register the AI library (no shared client configured)
	ai.Register(p)

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
