package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paularlott/cli"
	cli_toml "github.com/paularlott/cli/toml"
)

type ToolParameter struct {
	Name        string
	Type        string
	Description string
	Required    bool
}

type ToolMetadata struct {
	Description  string
	Keywords     []string
	Parameters   []ToolParameter
	Discoverable bool
}

// ScanToolsFolder scans the tools folder for .toml files and returns metadata
func ScanToolsFolder(toolsFolder string) (map[string]*ToolMetadata, error) {
	tools := make(map[string]*ToolMetadata)

	entries, err := os.ReadDir(toolsFolder)
	if err != nil {
		return nil, fmt.Errorf("failed to read tools folder: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}

		toolName := strings.TrimSuffix(entry.Name(), ".toml")
		tomlPath := filepath.Join(toolsFolder, entry.Name())

		// Load TOML file using cli/toml
		baseConfig := cli_toml.NewConfigFile(&tomlPath, func() []string { return []string{toolsFolder} })
		cfg := cli.NewTypedConfigFile(baseConfig)
		if err := cfg.LoadData(); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", tomlPath, err)
		}

		// Parse metadata
		meta := ToolMetadata{
			Description:  cfg.GetString("description"),
			Keywords:     cfg.GetStringSlice("keywords"),
			Discoverable: cfg.GetBool("discoverable"),
		}

		// Parse parameters
		paramObjs := cfg.GetObjectSlice("parameters")
		for _, paramObj := range paramObjs {
			param := ToolParameter{
				Name:        paramObj.GetString("name"),
				Type:        paramObj.GetString("type"),
				Description: paramObj.GetString("description"),
				Required:    paramObj.GetBool("required"),
			}
			meta.Parameters = append(meta.Parameters, param)
		}

		tools[toolName] = &meta
	}

	return tools, nil
}

// ValidateTool checks if a tool's metadata matches its script
func ValidateTool(toolName string, meta *ToolMetadata, scriptPath string) []string {
	var warnings []string

	// Check if script file exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		warnings = append(warnings, fmt.Sprintf("script file not found: %s", scriptPath))
		return warnings
	}

	// Read script content
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to read script: %v", err))
		return warnings
	}

	script := string(content)

	// Check if parameters are accessed in the script
	for _, param := range meta.Parameters {
		// Look for mcp.tool.get_* calls with this parameter name
		accessPattern := fmt.Sprintf(`mcp.tool.get_%s("%s"`, param.Type, param.Name)
		if !strings.Contains(script, accessPattern) {
			// Also check for single quotes
			accessPattern = fmt.Sprintf(`mcp.tool.get_%s('%s'`, param.Type, param.Name)
			if !strings.Contains(script, accessPattern) {
				warnings = append(warnings, fmt.Sprintf("parameter '%s' not accessed in script", param.Name))
			}
		}
	}

	return warnings
}
