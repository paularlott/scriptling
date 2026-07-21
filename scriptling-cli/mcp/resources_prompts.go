package mcp

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// resourceMetaTOML mirrors the optional _{stem}.toml metadata sibling of a
// resource file.
type resourceMetaTOML struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
	MimeType    string `toml:"mimeType"`
}

// parseResourceMetadata decodes resource metadata from TOML bytes.
func parseResourceMetadata(data []byte) (name, description, mimeType string, err error) {
	var m resourceMetaTOML
	if _, err = toml.NewDecoder(bytes.NewReader(data)).Decode(&m); err != nil {
		return "", "", "", err
	}
	return m.Name, m.Description, m.MimeType, nil
}

// promptMetaTOML mirrors a prompt's .toml metadata file.
type promptMetaTOML struct {
	Description string `toml:"description"`
	Arguments   []struct {
		Name        string `toml:"name"`
		Description string `toml:"description"`
		Required    bool   `toml:"required"`
	} `toml:"arguments"`
}

// parsePromptMetadata decodes prompt metadata from TOML bytes.
func parsePromptMetadata(data []byte) (string, []PromptArgument, error) {
	var m promptMetaTOML
	if _, err := toml.NewDecoder(bytes.NewReader(data)).Decode(&m); err != nil {
		return "", nil, err
	}
	var args []PromptArgument
	for _, a := range m.Arguments {
		args = append(args, PromptArgument{
			Name:        a.Name,
			Description: a.Description,
			Required:    a.Required,
		})
	}
	return m.Description, args, nil
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
	URI         string // full URI (static) or URI template
	Name        string
	Description string
	MimeType    string
	Template    bool
	FilePath    string // path within the scanned FS (slash-separated)
	Vars        []string
}

// readMetadataSibling reads and parses the optional _{stem}.toml metadata
// sibling of rel. found is false when no sibling exists.
func readMetadataSibling(fsys fs.FS, rel string) (meta resourceMetaTOML, found bool, err error) {
	ext := path.Ext(rel)
	stem := strings.TrimSuffix(path.Base(rel), ext)
	sib := path.Join(path.Dir(rel), "_"+stem+".toml")
	data, err := fs.ReadFile(fsys, sib)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return resourceMetaTOML{}, false, nil
		}
		return resourceMetaTOML{}, false, err
	}
	name, desc, mimeType, err := parseResourceMetadata(data)
	if err != nil {
		return resourceMetaTOML{}, false, fmt.Errorf("failed to parse resource metadata %s: %w", sib, err)
	}
	return resourceMetaTOML{Name: name, Description: desc, MimeType: mimeType}, true, nil
}

// ScanResourcesFS walks fsys and returns every resource (static or template)
// it describes. FilePath in the results is the slash path within fsys.
func ScanResourcesFS(fsys fs.FS) ([]scannedResource, error) {
	var out []scannedResource
	err := fs.WalkDir(fsys, ".", func(rel string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		// Files starting with _ are metadata, never served as resources.
		if strings.HasPrefix(d.Name(), "_") {
			return nil
		}
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
		ext := path.Ext(rel)
		base := rest[len(rest)-1]
		stem := strings.TrimSuffix(base, ext)
		isPy := ext == ".py"

		switch {
		case hasVar && isPy:
			// Template: strip the .py from the last segment to form the URI.
			uriPath := append([]string{}, rest[:len(rest)-1]...)
			uriPath = append(uriPath, stem)
			uri := scheme + "://" + strings.Join(uriPath, "/")

			// Load optional _{stem}.toml metadata sibling. If absent, fall back
			// to the full URI as the name with no description.
			name, desc, mimeType := uri, "", ""
			if meta, found, err := readMetadataSibling(fsys, rel); err != nil {
				return err
			} else if found {
				if meta.Name != "" {
					name = meta.Name
				}
				desc = meta.Description
				mimeType = meta.MimeType
			}

			out = append(out, scannedResource{
				URI:         uri,
				Name:        name,
				Description: desc,
				MimeType:    mimeType,
				Template:    true,
				FilePath:    rel,
				Vars:        extractTemplateVars(rest),
			})
		case hasVar && !isPy:
			// A template pattern with no .py handler — nothing can serve it.
			return nil
		default:
			// Static: keep the extension in the URI (e.g. readme.md).
			uri := scheme + "://" + strings.Join(rest, "/")
			name, desc := uri, ""
			mimeType := mimeTypeForExt(ext)
			// Load optional _{stem}.toml metadata sibling.
			if meta, found, err := readMetadataSibling(fsys, rel); err != nil {
				return err
			} else if found {
				if meta.Name != "" {
					name = meta.Name
				}
				desc = meta.Description
				if meta.MimeType != "" {
					mimeType = meta.MimeType
				}
			}
			out = append(out, scannedResource{
				URI:         uri,
				Name:        name,
				Description: desc,
				MimeType:    mimeType,
				FilePath:    rel,
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan resources: %w", err)
	}
	return out, nil
}

// ScanResourcesTree walks a resources directory on disk and returns every
// resource it describes. FilePath in the results is a disk path.
func ScanResourcesTree(dir string) ([]scannedResource, error) {
	res, err := ScanResourcesFS(os.DirFS(dir))
	if err != nil {
		return nil, err
	}
	for i := range res {
		res[i].FilePath = filepath.Join(dir, filepath.FromSlash(res[i].FilePath))
	}
	return res, nil
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
	FilePath    string           // .py (dynamic) or .md/.txt (static); path within the scanned FS
}

// ScanPromptsFS scans fsys for prompts. A prompt is dynamic when a name.toml
// (with sibling name.py) is present, or static when only a name.md/name.txt is
// present. If both exist for a name, the dynamic one wins. FilePath in the
// results is the slash path within fsys.
func ScanPromptsFS(fsys fs.FS) ([]scannedPrompt, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to read prompts folder: %w", err)
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
		ext := path.Ext(name)
		stem := strings.TrimSuffix(name, ext)

		switch ext {
		case ".toml":
			scriptPath := stem + ".py"
			if _, err := fs.Stat(fsys, scriptPath); errors.Is(err, fs.ErrNotExist) {
				// Declared prompt with no handler script — skip with a warning
				// rather than failing the whole folder.
				continue
			}
			data, err := fs.ReadFile(fsys, name)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", name, err)
			}
			desc, args, err := parsePromptMetadata(data)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %s: %w", name, err)
			}
			out = append(out, scannedPrompt{
				Name:        stem,
				Description: desc,
				Arguments:   args,
				FilePath:    scriptPath,
			})

		case ".md", ".txt":
			if dynamicStems[stem] {
				continue // dynamic version wins
			}
			data, err := fs.ReadFile(fsys, name)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", name, err)
			}
			out = append(out, scannedPrompt{
				Name:        stem,
				Static:      true,
				Description: firstLineBytes(data),
				FilePath:    name,
			})
		}
	}
	return out, nil
}

// ScanPromptsFolder scans a prompts folder on disk. FilePath in the results is
// a disk path.
func ScanPromptsFolder(dir string) ([]scannedPrompt, error) {
	prompts, err := ScanPromptsFS(os.DirFS(dir))
	if err != nil {
		return nil, err
	}
	for i := range prompts {
		prompts[i].FilePath = filepath.Join(dir, filepath.FromSlash(prompts[i].FilePath))
	}
	return prompts, nil
}

// firstLineBytes returns the first non-empty line of data (used as a static
// prompt's description).
func firstLineBytes(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		if l := strings.TrimSpace(line); l != "" {
			return l
		}
	}
	return ""
}
