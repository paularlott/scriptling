package mcp

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/paularlott/mcp"
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
	// UI links the tool to a companion ui:// resource per the MCP Apps
	// extension (SEP-1865): the host renders that resource to display this
	// tool's results. resourceUri is optional — an "app"-only action tool
	// (one only ever called by a view that's already open, such as a form
	// submission) has no rendering purpose of its own and can declare just
	// visibility, which is otherwise optional too and defaults to both
	// "model" and "app" when omitted — but at least one of the two must be
	// present, or [ui] shouldn't be there at all.
	UI *struct {
		ResourceURI string   `toml:"resourceUri"`
		Visibility  []string `toml:"visibility"`
	} `toml:"ui"`
	// Icons attaches visual identifiers to the tool's tools/list descriptor,
	// e.g.:
	//
	//	[[icons]]
	//	src = "https://example.com/icon.png"
	//	mimeType = "image/png"
	//	sizes = ["48x48"]
	Icons []struct {
		Src      string   `toml:"src"`
		MimeType string   `toml:"mimeType"`
		Sizes    []string `toml:"sizes"`
		Theme    string   `toml:"theme"`
	} `toml:"icons"`
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
	if m.UI != nil {
		if m.UI.ResourceURI == "" && len(m.UI.Visibility) == 0 {
			return nil, fmt.Errorf("[ui] table requires at least one of resourceUri or visibility")
		}
		meta.UI = &mcp.UIToolMeta{
			ResourceURI: m.UI.ResourceURI,
			Visibility:  m.UI.Visibility,
		}
	}
	for i, ic := range m.Icons {
		if ic.Src == "" {
			return nil, fmt.Errorf("icons[%d] requires a non-empty src", i)
		}
		meta.Icons = append(meta.Icons, mcp.Icon{
			Src:      ic.Src,
			MimeType: ic.MimeType,
			Sizes:    ic.Sizes,
			Theme:    ic.Theme,
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
