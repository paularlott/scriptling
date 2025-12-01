package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/paularlott/mcp"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
	"gopkg.in/yaml.v2"
)

type FrontMatter struct {
	Description string `yaml:"description"`
}

func parseFrontMatter(content string) (FrontMatter, string, error) {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || lines[0] != "---" {
		return FrontMatter{}, content, nil // no front matter
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return FrontMatter{}, content, nil
	}
	yamlStr := strings.Join(lines[1:end], "\n")
	var fm FrontMatter
	err := yaml.Unmarshal([]byte(yamlStr), &fm)
	if err != nil {
		return FrontMatter{}, content, err
	}
	body := strings.Join(lines[end+1:], "\n")
	return fm, body, nil
}

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

			// Register all standard libraries
			stdlib.RegisterAll(p)

			// Register HTTP library for scripts that need it
			p.RegisterLibrary(extlibs.RequestsLibraryName, extlibs.RequestsLibrary)
			p.RegisterLibrary(extlibs.SysLibraryName, extlibs.SysLibrary)
			p.RegisterLibrary(extlibs.SecretsLibraryName, extlibs.SecretsLibrary)
			p.RegisterLibrary(extlibs.SubprocessLibraryName, extlibs.SubprocessLibrary)
			p.RegisterLibrary(extlibs.HTMLParserLibraryName, extlibs.HTMLParserLibrary)
			extlibs.RegisterOSLibrary(p, []string{})
			extlibs.RegisterPathlibLibrary(p, []string{})

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

	// Tool 2: Skills - Renamed and improved
	server.RegisterTool(
		mcp.NewTool(
			"list_and_get_skills",
			"ALWAYS START HERE: List all available pre-built skills with descriptions, then retrieve the full content of a specific skill. Skills are tested, working solutions for common tasks. Using existing skills is faster and more reliable than writing code from scratch. Call without parameters first to see what's available.",
			mcp.String("name", "Optional: The exact name of a skill to retrieve its full content. Omit this parameter to list all available skills with their descriptions first."),
		),
		func(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
			name, err := req.String("name")
			skillsDir := "skills"

			if err != nil || name == "" {
				// List all skills with enhanced formatting
				files, err := filepath.Glob(filepath.Join(skillsDir, "*.md"))
				if err != nil {
					return mcp.NewToolResponseText(fmt.Sprintf("Error listing skills: %s", err.Error())), nil
				}

				var response strings.Builder
				response.WriteString("Available Skills (call this tool again with 'name' parameter to get full skill content):\n\n")

				var skills []map[string]string
				for _, file := range files {
					contentBytes, err := os.ReadFile(file)
					if err != nil {
						continue
					}
					content := string(contentBytes)
					fm, _, err := parseFrontMatter(content)
					if err != nil {
						continue
					}
					skillName := strings.TrimSuffix(filepath.Base(file), ".md")
					skills = append(skills, map[string]string{
						"name":        skillName,
						"description": fm.Description,
					})
					response.WriteString(fmt.Sprintf("- %s: %s\n", skillName, fm.Description))
				}

				response.WriteString("\nTo use a skill, call this tool again with the skill name to get its full implementation.")

				return mcp.NewToolResponseText(response.String()), nil
			} else {
				// Get specific skill
				file := filepath.Join(skillsDir, name+".md")
				contentBytes, err := os.ReadFile(file)
				if err != nil {
					return mcp.NewToolResponseText(fmt.Sprintf("Skill '%s' not found. Call without 'name' parameter to see available skills.", name)), nil
				}
				content := string(contentBytes)
				_, body, err := parseFrontMatter(content)
				if err != nil {
					return mcp.NewToolResponseText(fmt.Sprintf("Error parsing skill: %s", err.Error())), nil
				}
				return mcp.NewToolResponseText(fmt.Sprintf("Skill: %s\n\n%s", name, body)), nil
			}
		},
	)

	// Start HTTP server
	http.HandleFunc("/mcp", server.HandleRequest)
	fmt.Println("Scriptling MCP Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
