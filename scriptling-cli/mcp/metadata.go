package mcp

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/paularlott/mcp/toolmetadata"
)

// toolMetaTOML mirrors the layout of a tool's .toml metadata file.
type toolMetaTOML struct {
	Description  string   `toml:"description"`
	Keywords     []string `toml:"keywords"`
	Discoverable bool     `toml:"discoverable"`
	Parameters   []struct {
		Name        string `toml:"name"`
		Type        string `toml:"type"`
		Description string `toml:"description"`
		Required    bool   `toml:"required"`
	} `toml:"parameters"`
}

// parseToolMetadata decodes tool metadata from TOML bytes. Unknown keys are
// ignored so metadata files stay forward compatible.
func parseToolMetadata(data []byte) (*toolmetadata.ToolMetadata, error) {
	var m toolMetaTOML
	if _, err := toml.NewDecoder(bytes.NewReader(data)).Decode(&m); err != nil {
		return nil, err
	}
	meta := &toolmetadata.ToolMetadata{
		Description:  m.Description,
		Keywords:     m.Keywords,
		Discoverable: m.Discoverable,
	}
	for _, p := range m.Parameters {
		meta.Parameters = append(meta.Parameters, toolmetadata.ToolParameter{
			Name:        p.Name,
			Type:        p.Type,
			Description: p.Description,
			Required:    p.Required,
		})
	}
	return meta, nil
}

// ScanToolsFS scans fsys (flat, root only) for .toml files and returns tool
// metadata keyed by tool name.
func ScanToolsFS(fsys fs.FS) (map[string]*toolmetadata.ToolMetadata, error) {
	tools := make(map[string]*toolmetadata.ToolMetadata)

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to read tools folder: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}

		toolName := strings.TrimSuffix(entry.Name(), ".toml")

		data, err := fs.ReadFile(fsys, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", entry.Name(), err)
		}
		meta, err := parseToolMetadata(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", entry.Name(), err)
		}

		tools[toolName] = meta
	}

	return tools, nil
}

// ScanToolsFolder scans a tools folder on disk for .toml files and returns
// metadata.
func ScanToolsFolder(toolsFolder string) (map[string]*toolmetadata.ToolMetadata, error) {
	return ScanToolsFS(os.DirFS(toolsFolder))
}
