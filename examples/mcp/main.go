package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/paularlott/mcp"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
)

func main() {
	server := mcp.NewServer("scriptling-server", "1.0.0")

	// Tool 1: Execute Scriptling code
	server.RegisterTool(
		mcp.NewTool(
			"execute_script",
			"Execute Scriptling code and return the output. IMPORTANT: If you are unsure about available libraries or functions, FIRST run help('modules') or help('library') to discover what exists. Do not invent modules.",
			mcp.String("code", "The Scriptling code to execute, scriptling is a Python style scripting language and should run most Python, use scriptling_info to get detailed information on it", mcp.Required()),
		),
		func(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
			code, _ := req.String("code")

			// Create interpreter
			p := scriptling.New()

			// Register HTTP library for scripts that need it
			p.RegisterLibrary("requests", extlibs.RequestsLibrary())

			// Enable output capture
			p.EnableOutputCapture()

			// Execute code
			result, err := p.Eval(code)

			// Get captured output
			output := p.GetOutput()

			var response strings.Builder
			if output != "" {
				response.WriteString(fmt.Sprintf("Output:\n%s\n", output))
			}

			if err != nil {
				response.WriteString(fmt.Sprintf("Error: %s\n", err.Error()))
			} else if result != nil && result.Type() != object.NULL_OBJ {
				response.WriteString(fmt.Sprintf("Result: %s\n", result.Inspect()))
			}

			return mcp.NewToolResponseText(response.String()), nil
		},
	)

	// Start HTTP server
	http.HandleFunc("/mcp", server.HandleRequest)
	fmt.Println("Scriptling MCP Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
