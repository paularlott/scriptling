package mcp

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/paularlott/cli"
	cli_toml "github.com/paularlott/cli/toml"
)

// loadTOML loads a .toml file via the cli/toml config loader.
func loadTOML(tomlPath, folder string) (*cli.ConfigFileTypedWrapper, error) {
	baseConfig := cli_toml.NewConfigFile(&tomlPath, func() []string { return []string{folder} })
	cfg := cli.NewTypedConfigFile(baseConfig)
	if err := cfg.LoadData(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// =========================================================================
// Resources: file-tree, no metadata files.
//
// The first path segment under the resources dir is the URI scheme; the rest
// of the path mirrors the URI. A file whose path contains a {var} segment and
// ends in .py is a resource TEMPLATE (the script is run with the extracted
// vars). Every other file is a STATIC resource served verbatim (a .py with no
// {var} is served as source text, not executed).
// =========================================================================

// scannedResource is one resource discovered in the resources tree.
type scannedResource struct {
	URI      string // full URI (static) or URI template
	Name     string
	MimeType string
	Template bool
	FilePath string // .py for templates; the content file for static
	Vars     []string
}

// ScanResourcesTree walks a resources directory and returns every resource
// (static or template) it describes.
func ScanResourcesTree(dir string) ([]scannedResource, error) {
	var out []scannedResource
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path == dir {
				return nil
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		segments := strings.Split(rel, "/")
		if len(segments) < 2 {
			// Root-level file has no scheme directory; skip it.
			return nil
		}
		scheme := segments[0]
		rest := segments[1:]

		hasVar := false
		for _, s := range rest {
			if strings.Contains(s, "{") && strings.Contains(s, "}") {
				hasVar = true
				break
			}
		}
		ext := filepath.Ext(path)
		base := rest[len(rest)-1]
		stem := strings.TrimSuffix(base, ext)
		isPy := ext == ".py"

		switch {
		case hasVar && isPy:
			// Template: strip the .py from the last segment to form the URI.
			uriPath := append([]string{}, rest[:len(rest)-1]...)
			uriPath = append(uriPath, stem)
			out = append(out, scannedResource{
				URI:      scheme + "://" + strings.Join(uriPath, "/"),
				Name:     stem,
				Template: true,
				FilePath: path,
				Vars:     extractTemplateVars(rest),
			})
		case hasVar && !isPy:
			// A template pattern with no .py handler — nothing can serve it.
			return nil
		default:
			// Static: keep the extension in the URI (e.g. readme.md).
			out = append(out, scannedResource{
				URI:      scheme + "://" + strings.Join(rest, "/"),
				Name:     base,
				MimeType: mimeTypeForExt(ext),
				FilePath: path,
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan resources folder %s: %w", dir, err)
	}
	return out, nil
}

// mimeTypeForExt returns the MIME type for an extension (e.g. ".md" -> "text/markdown"),
// or "" if unknown (the resource then carries no mimeType).
func mimeTypeForExt(ext string) string {
	if ext == "" {
		return ""
	}
	return mime.TypeByExtension(ext)
}

// extractTemplateVars returns the {var} placeholder names found across the path
// segments, in order of appearance.
func extractTemplateVars(segments []string) []string {
	var vars []string
	for _, seg := range segments {
		for _, v := range varsInSegment(seg) {
			vars = append(vars, v)
		}
	}
	return vars
}

// varsInSegment extracts any {var} names from a single path segment.
func varsInSegment(seg string) []string {
	var vars []string
	for i := 0; i < len(seg); {
		start := strings.IndexByte(seg[i:], '{')
		if start == -1 {
			break
		}
		start += i
		end := strings.IndexByte(seg[start:], '}')
		if end == -1 {
			break
		}
		vars = append(vars, strings.TrimSpace(seg[start+1:start+end]))
		i = start + end + 1
	}
	return vars
}

// =========================================================================
// Prompts: dynamic (name.toml + name.py) and static (name.md / name.txt).
// =========================================================================

// PromptArgument describes one argument a dynamic prompt accepts.
type PromptArgument struct {
	Name        string
	Description string
	Required    bool
}

// scannedPrompt is one prompt discovered in the prompts folder.
type scannedPrompt struct {
	Name        string
	Static      bool
	Description string
	Arguments   []PromptArgument // dynamic only
	FilePath    string           // .py (dynamic) or .md/.txt (static)
}

// ScanPromptsFolder scans a prompts folder. A prompt is dynamic when a name.toml
// (with sibling name.py) is present, or static when only a name.md/name.txt is
// present. If both exist for a name, the dynamic one wins.
func ScanPromptsFolder(dir string) ([]scannedPrompt, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read prompts folder %s: %w", dir, err)
	}

	// First pass: collect the stems that have a .toml (dynamic wins).
	dynamicStems := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".toml") {
			dynamicStems[strings.TrimSuffix(e.Name(), ".toml")] = true
		}
	}

	var out []scannedPrompt
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := filepath.Ext(name)
		stem := strings.TrimSuffix(name, ext)

		switch ext {
		case ".toml":
			scriptPath := filepath.Join(dir, stem+".py")
			if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
				// Declared prompt with no handler script — skip with a warning
				// rather than failing the whole folder.
				continue
			}
			cfg, err := loadTOML(filepath.Join(dir, name), dir)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %s: %w", name, err)
			}
			p := scannedPrompt{
				Name:        stem,
				Description: cfg.GetString("description"),
				FilePath:    scriptPath,
			}
			for _, arg := range cfg.GetObjectSlice("arguments") {
				p.Arguments = append(p.Arguments, PromptArgument{
					Name:        arg.GetString("name"),
					Description: arg.GetString("description"),
					Required:    arg.GetBool("required"),
				})
			}
			out = append(out, p)

		case ".md", ".txt":
			if dynamicStems[stem] {
				continue // dynamic version wins
			}
			desc, _ := firstLine(filepath.Join(dir, name))
			out = append(out, scannedPrompt{
				Name:        stem,
				Static:      true,
				Description: desc,
				FilePath:    filepath.Join(dir, name),
			})
		}
	}
	return out, nil
}

// firstLine returns the first non-empty line of a file (used as a static
// prompt's description), or ("", error).
func firstLine(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		l := strings.TrimSpace(line)
		if l != "" {
			return l, nil
		}
	}
	return "", nil
}
