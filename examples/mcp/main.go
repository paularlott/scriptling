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
		mcp.NewTool("execute_script", "Execute Scriptling code and return the output",
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

	// Tool 2: Get language info and differences
	server.RegisterTool(
		mcp.NewTool("scriptling_info", "Get information about Scriptling language differences from Python and available libraries",
			mcp.String("info_type", "Type of information: 'differences', 'libraries', or 'all'", mcp.Required()),
		),
		func(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
			infoType, _ := req.String("info_type")

			var response strings.Builder

			if infoType == "differences" || infoType == "all" {
				response.WriteString("## Scriptling vs Python Differences\n\n")
				response.WriteString("### Syntax Differences:\n")
				differences := []string{
					"Division (/) always returns float (like Python 3)",
					"No classes or inheritance - functions only",
					"No decorators or generators",
					"No list/dict/set comprehensions with nested loops",
					"No with statements or context managers",
					"No modules/packages - only library imports",
					"No async/await or yield",
					"String formatting: Use + concatenation or .replace() method",
				}
				for _, diff := range differences {
					response.WriteString(fmt.Sprintf("- %s\n", diff))
				}

				response.WriteString("\n### Type Differences:\n")
				typeDiffs := []string{
					"No complex numbers, sets, or bytes type",
					"Tuples are immutable lists with unpacking",
					"None instead of null",
				}
				for _, diff := range typeDiffs {
					response.WriteString(fmt.Sprintf("- %s\n", diff))
				}
			}

			if infoType == "libraries" || infoType == "all" {
				if infoType == "all" {
					response.WriteString("\n")
				}
				response.WriteString("## Available Libraries\n\n")
				response.WriteString("### Built-in Libraries (import to use):\n")
				libs := []string{
					"json - Parse and stringify JSON (loads/dumps)",
					"re - Regular expressions (Python re module)",
					"math - Mathematical functions (Python math module)",
					"time - Time operations (Python time module)",
					"datetime - Date/time operations (Python datetime module)",
					"base64 - Base64 encoding/decoding",
					"hashlib - Hashing functions (md5, sha1, sha256)",
					"random - Random number generation",
					"lib - URL parsing and manipulation (Python urllib.parse compatible)",
				}
				for _, lib := range libs {
					response.WriteString(fmt.Sprintf("- %s\n", lib))
				}

				response.WriteString("\n### Optional Libraries:\n")
				response.WriteString("- http - HTTP requests (pre-registered in this server)\n")

				response.WriteString("\n### Usage Example:\n")
				response.WriteString("```python\n")
				response.WriteString("# JSON parsing (Python equivalent)\n")
				response.WriteString("import json\n")
				response.WriteString("data = json.loads('{\"key\": \"value\"}')\n")
				response.WriteString("result = json.dumps(data)\n")
				response.WriteString("\n# URL manipulation (Python urllib.parse equivalent)\n")
				response.WriteString("import lib\n")
				response.WriteString("encoded = lib.quote('hello world')\n")
				response.WriteString("parts = lib.urlsplit('https://example.com/path')\n")
				response.WriteString("query = lib.urlencode({'key': 'value'})\n")
				response.WriteString("\n# Regular expressions (Python re equivalent)\n")
				response.WriteString("import re\n")
				response.WriteString("matches = re.findall(r'\\d+', 'abc123def456')  # findall()\n")
				response.WriteString("\n# Exception handling\n")
				response.WriteString("try:\n")
				response.WriteString("    x = 1 / 0\n")
				response.WriteString("except Exception as e:\n")
				response.WriteString("    print(f\"Error: {e}\")\n")
				response.WriteString("```\n")
			}

			return mcp.NewToolResponseText(response.String()), nil
		},
	)

	// Start HTTP server
	http.HandleFunc("/mcp", server.HandleRequest)
	fmt.Println("Scriptling MCP Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
