package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/paularlott/mcp"
	"github.com/paularlott/mcp/discovery"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/ai"
	"github.com/paularlott/scriptling/extlibs"
	scriptlingmcp "github.com/paularlott/scriptling/mcp"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

func main() {
	// Create MCP server
	server := mcp.NewServer("scriptling-server", "1.0.0")

	// Set instructions for the LLM
	server.SetInstructions(`This server executes Scriptling/Python code.
Use tool_search to discover pre-built tools for common tasks.
Use execute_tool to run a discovered tool, or execute_code for custom code.`)

	// Register execute_code as a regular visible tool
	registerExecuteCode(server)

	// Create discovery registry and register script tools
	registry := discovery.NewToolRegistry()
	registerScriptTools(registry)

	// Attach registry to server (registers tool_search, execute_tool)
	registry.Attach(server)

	// Start HTTP server
	http.HandleFunc("/mcp", server.HandleRequest)

	fmt.Println("Scriptling MCP Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// registerExecuteCode registers the execute_code tool with the MCP server
func registerExecuteCode(server *mcp.Server) {
	server.RegisterTool(
		mcp.NewTool(
			"execute_code",
			"Execute Scriptling/Python code. Use tool_search first to check for existing implementations.",
			mcp.String("code", "The Scriptling/Python code to execute", mcp.Required()),
		),
		func(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
			code, _ := req.String("code")
			return executeCode(code)
		},
	)
}

// registerScriptTools registers pre-built script tools with the discovery registry
func registerScriptTools(registry *discovery.ToolRegistry) {
	// Generate Calendar
	registry.RegisterTool(
		mcp.NewTool("generate_calendar",
			"Generate a formatted ASCII calendar for any month and year",
			mcp.Number("year", "Year (e.g., 2025)", mcp.Required()),
			mcp.Number("month", "Month 1-12", mcp.Required()),
		),
		func(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
			year := req.IntOr("year", 2025)
			month := req.IntOr("month", 12)
			code := fmt.Sprintf(`import datetime

def is_leap_year(year):
    return year %% 4 == 0 and (year %% 100 != 0 or year %% 400 == 0)

def days_in_month(year, month):
    days = [31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31]
    if month == 2 and is_leap_year(year):
        return 29
    return days[month - 1]

def generate_calendar(year, month):
    month_names = ["", "January", "February", "March", "April", "May", "June",
                   "July", "August", "September", "October", "November", "December"]
    header = f"{month_names[month]} {year}".center(20)
    days_header = "Mo Tu We Th Fr Sa Su"
    lines = [header, "", days_header, ""]

    first = datetime.datetime.strptime(f"{year}-{month:02d}-01", "%%Y-%%m-%%d")
    ref = datetime.datetime.strptime("2000-01-01", "%%Y-%%m-%%d").timestamp()
    diff = int((first.timestamp() - ref) / 86400)
    start = (6 + diff) %% 7

    day = 1
    num_days = days_in_month(year, month)
    week = ""
    for i in range(7):
        if i < start:
            week += "   "
        else:
            week += f"{day:2d} "
            day += 1
    lines.append(week.rstrip())

    while day <= num_days:
        week = ""
        for i in range(7):
            if day <= num_days:
                week += f"{day:2d} "
                day += 1
            else:
                week += "   "
        lines.append(week.rstrip())

    return "\n".join(lines)

print(generate_calendar(%d, %d))`, year, month)
			return executeCode(code)
		},
		"calendar", "date", "month", "year", "schedule", "datetime", "ascii",
	)

	// Generate Password
	registry.RegisterTool(
		mcp.NewTool("generate_password",
			"Generate a secure random password",
			mcp.Number("length", "Password length (default: 16)"),
			mcp.Boolean("uppercase", "Include uppercase (default: true)"),
			mcp.Boolean("lowercase", "Include lowercase (default: true)"),
			mcp.Boolean("digits", "Include digits (default: true)"),
			mcp.Boolean("symbols", "Include symbols (default: false)"),
		),
		func(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
			length := req.IntOr("length", 16)
			upper := req.BoolOr("uppercase", true)
			lower := req.BoolOr("lowercase", true)
			digits := req.BoolOr("digits", true)
			symbols := req.BoolOr("symbols", false)

			// Helper to convert Go bool to Python bool string
			pyBool := func(b bool) string {
				if b {
					return "True"
				}
				return "False"
			}

			code := fmt.Sprintf(`import random
import string

def generate_password(length=%d, upper=%s, lower=%s, digits=%s, symbols=%s):
    chars = ""
    if lower: chars += string.ascii_lowercase
    if upper: chars += string.ascii_uppercase
    if digits: chars += string.digits
    if symbols: chars += string.punctuation
    if not chars: return "Error: No character sets selected"

    password = []
    if lower: password.append(random.choice(string.ascii_lowercase))
    if upper: password.append(random.choice(string.ascii_uppercase))
    if digits: password.append(random.choice(string.digits))
    if symbols: password.append(random.choice(string.punctuation))

    while len(password) < length:
        password.append(random.choice(chars))

    random.shuffle(password)
    return "".join(password)

print(generate_password())`, length, pyBool(upper), pyBool(lower), pyBool(digits), pyBool(symbols))
			return executeCode(code)
		},
		"password", "random", "secure", "security", "secret", "credentials",
	)

	// HTTP POST JSON
	registry.RegisterTool(
		mcp.NewTool("http_post_json",
			"Send a JSON POST request to a URL",
			mcp.String("url", "The URL to POST to", mcp.Required()),
			mcp.String("data", "JSON data to send", mcp.Required()),
		),
		func(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
			url, _ := req.String("url")
			data, _ := req.String("data")

			code := fmt.Sprintf(`import json
import requests

url = %q
data = %q

response = requests.post(url, data, {"headers": {"Content-Type": "application/json"}})
print(f"Status: {response.status_code}")
print(f"Response: {response.text}")`, url, data)
			return executeCode(code)
		},
		"http", "post", "json", "api", "request", "rest", "web",
	)
}

// executeCode runs Scriptling code and returns the result
func executeCode(code string) (*mcp.ToolResponse, error) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	extlibs.RegisterRequestsLibrary(p)
	extlibs.RegisterSysLibrary(p, []string{})
	extlibs.RegisterSecretsLibrary(p)
	extlibs.RegisterSubprocessLibrary(p)
	extlibs.RegisterHTMLParserLibrary(p)
	extlibs.RegisterThreadsLibrary(p)
	extlibs.RegisterOSLibrary(p, []string{})
	extlibs.RegisterPathlibLibrary(p, []string{})

	ai.Register(p)
	scriptlingmcp.Register(p)
	scriptlingmcp.RegisterToon(p)

	p.EnableOutputCapture()

	result, err := p.Eval(code)
	output := p.GetOutput()

	var response strings.Builder
	if output != "" {
		response.WriteString(output)
	}
	if err != nil {
		response.WriteString(fmt.Sprintf("\nError: %s", err.Error()))
	} else if result != nil && result.Type() != object.NULL_OBJ {
		if response.Len() > 0 {
			response.WriteString("\n")
		}
		response.WriteString(fmt.Sprintf("Result: %s", result.Inspect()))
	}

	return mcp.NewToolResponseText(response.String()), nil
}
