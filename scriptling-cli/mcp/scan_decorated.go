package mcp

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"time"

	mcplib "github.com/paularlott/mcp"
	"github.com/paularlott/mcp/toolmetadata"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
)

// DecoratedTool represents a tool discovered via the @mcp.tool() decorator in
// a .py file (no .toml sidecar).
type DecoratedTool struct {
	Name     string
	Meta     *toolmetadata.ToolMetadata
	FuncName string // function to call within the source
	Source   []byte // the full .py file source
}

// ScannedToolEntry is a unified entry produced by the dual-format scanner.
// It covers both legacy (.toml+.py) and decorated (.py-only) tools.
type ScannedToolEntry struct {
	Name     string
	Meta     *toolmetadata.ToolMetadata
	Source   []byte // script source (.py content)
	FuncName string // non-empty for decorated tools; empty for legacy
	Legacy   bool   // true = legacy .toml+.py format
}

// ScanToolsFSDual scans fsys for tools in both formats:
//   - Legacy: .toml file with a sibling .py (existing behavior).
//   - Decorated: .py file with no sibling .toml; evaluated to discover
//     @mcp.tool() registrations.
//
// cfg is used to configure the interpreter for decorated tool discovery.
// Files prefixed with _ are skipped.
func ScanToolsFSDual(fsys fs.FS, cfg HandlerConfig) ([]ScannedToolEntry, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to read tools folder: %w", err)
	}

	// Build a set of stems that have a .toml (these are legacy tools).
	tomlStems := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		if strings.HasPrefix(e.Name(), "_") {
			continue
		}
		tomlStems[strings.TrimSuffix(e.Name(), ".toml")] = true
	}

	var result []ScannedToolEntry

	// Pass 1: Legacy tools (.toml + .py pairs).
	for stem := range tomlStems {
		tomlData, err := fs.ReadFile(fsys, stem+".toml")
		if err != nil {
			return nil, fmt.Errorf("failed to read %s.toml: %w", stem, err)
		}
		meta, err := parseToolMetadata(tomlData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s.toml: %w", stem, err)
		}

		// Read sibling .py if it exists (may be absent — caller handles that).
		var src []byte
		if pyData, readErr := fs.ReadFile(fsys, stem+".py"); readErr == nil {
			src = pyData
		}

		result = append(result, ScannedToolEntry{
			Name:   stem,
			Meta:   meta,
			Source: src,
			Legacy: true,
		})
	}

	// Pass 2: Decorated tools (.py without sibling .toml).
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".py") {
			continue
		}
		if strings.HasPrefix(e.Name(), "_") {
			continue
		}
		stem := strings.TrimSuffix(e.Name(), ".py")
		if tomlStems[stem] {
			continue // legacy tool — handled above
		}

		src, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", e.Name(), err)
		}

		tools, scanErr := ScanDecoratedTools(src, cfg)
		if scanErr != nil {
			return nil, fmt.Errorf("failed to scan decorated tools in %s: %w", e.Name(), scanErr)
		}

		for _, tool := range tools {
			result = append(result, ScannedToolEntry{
				Name:     tool.Name,
				Meta:     tool.Meta,
				Source:   tool.Source,
				FuncName: tool.FuncName,
				Legacy:   false,
			})
		}
	}

	return result, nil
}

// ScanDecoratedTools evaluates a .py source in a fresh interpreter with
// runtime.mcp registered, then reads __mcp_registry to discover decorated
// tools. For each entry it builds ToolMetadata by cross-referencing the
// decorator's params dict with the function's signature (name from __name__,
// required from presence of defaults).
func ScanDecoratedTools(src []byte, cfg HandlerConfig) ([]DecoratedTool, error) {
	p := prepareScriptling(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := p.EvalWithContext(ctx, string(src))
	if err != nil {
		return nil, fmt.Errorf("eval failed: %w", err)
	}

	registryObj, getErr := p.GetVarAsObject(extlibs.MCPRegistryVar)
	if getErr != nil {
		// No registry means no decorated tools — not an error.
		return nil, nil
	}

	registryList, ok := registryObj.(*object.List)
	if !ok {
		return nil, fmt.Errorf("%s is not a list", extlibs.MCPRegistryVar)
	}

	if len(registryList.Elements) == 0 {
		return nil, nil
	}

	var tools []DecoratedTool
	for i, elem := range registryList.Elements {
		entry, ok := elem.(*object.Dict)
		if !ok {
			return nil, fmt.Errorf("registry entry %d is not a dict", i)
		}

		tool, err := decodeRegistryEntry(entry, src, p)
		if err != nil {
			return nil, fmt.Errorf("registry entry %d: %w", i, err)
		}
		tools = append(tools, *tool)
	}

	return tools, nil
}

// decodeRegistryEntry converts one __mcp_registry dict entry into a DecoratedTool.
func decodeRegistryEntry(entry *object.Dict, src []byte, p *scriptling.Scriptling) (*DecoratedTool, error) {
	name := dictGetString(entry, "name")
	if name == "" {
		return nil, fmt.Errorf("missing or empty 'name'")
	}

	description := dictGetString(entry, "description")
	discoverable := dictGetBool(entry, "discoverable")
	keywords := dictGetStringList(entry, "keywords")

	// Build parameters from the decorator's params dict + function signature.
	params, err := buildParamsFromRegistry(entry, name, p)
	if err != nil {
		return nil, err
	}

	ui, err := dictGetUIToolMeta(entry, "ui")
	if err != nil {
		return nil, fmt.Errorf("tool %q: %w", name, err)
	}

	icons, err := dictGetIcons(entry, "icons")
	if err != nil {
		return nil, fmt.Errorf("tool %q: %w", name, err)
	}

	meta := &toolmetadata.ToolMetadata{
		Description:  description,
		Keywords:     keywords,
		Discoverable: discoverable,
		Parameters:   params,
		UI:           ui,
		Icons:        icons,
	}

	return &DecoratedTool{
		Name:     name,
		Meta:     meta,
		FuncName: name,
		Source:   src,
	}, nil
}

// buildParamsFromRegistry resolves the parameter list for a decorated tool by
// combining the decorator's params dict with the function signature. The
// function object is looked up in the post-eval environment by name.
func buildParamsFromRegistry(entry *object.Dict, funcName string, p *scriptling.Scriptling) ([]toolmetadata.ToolParameter, error) {
	// Look up the function to inspect its signature.
	fnObj, err := p.GetVarAsObject(funcName)
	if err != nil {
		return nil, fmt.Errorf("function %q not found in environment", funcName)
	}

	fn, ok := fnObj.(*object.Function)
	if !ok {
		return nil, fmt.Errorf("%q is not a function (got %s)", funcName, fnObj.Type())
	}

	// Extract param names and which have defaults.
	type sigParam struct {
		name       string
		hasDefault bool
	}
	var sigParams []sigParam
	for _, param := range fn.Parameters {
		paramName := param.Value()
		_, hasDefault := fn.DefaultValues[paramName]
		sigParams = append(sigParams, sigParam{name: paramName, hasDefault: hasDefault})
	}

	// Get the decorator's params dict (may be nil/absent).
	var paramsDict *object.Dict
	if pair, ok := entry.GetByString("params"); ok {
		if d, ok := pair.Value.(*object.Dict); ok {
			paramsDict = d
		}
	}

	// Cross-check: params dict keys must match signature params.
	if paramsDict != nil {
		for _, pair := range paramsDict.Pairs {
			keyStr, _ := pair.Key.AsString()
			if keyStr == "" {
				continue
			}
			found := false
			for _, sp := range sigParams {
				if sp.name == keyStr {
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("params key %q does not match any parameter of function %q", keyStr, funcName)
			}
		}
	}

	// Build the parameter list from the function signature.
	var result []toolmetadata.ToolParameter
	for _, sp := range sigParams {
		tp := toolmetadata.ToolParameter{
			Name:     sp.name,
			Type:     "string", // default
			Required: !sp.hasDefault,
		}

		// If the params dict has metadata for this param, apply it.
		if paramsDict != nil {
			if pair, ok := paramsDict.GetByString(sp.name); ok {
				applyParamMetadata(&tp, pair.Value, sp, fn)
			}
		}

		// Infer type from default value if not explicitly set via params dict.
		if tp.Type == "string" && sp.hasDefault {
			if inferred := inferTypeFromDefault(fn, sp.name); inferred != "" {
				tp.Type = inferred
			}
		}

		result = append(result, tp)
	}

	return result, nil
}

// applyParamMetadata applies decorator params dict metadata to a ToolParameter.
// The value can be a string (description only) or a dict (type, description,
// optional required override).
func applyParamMetadata(tp *toolmetadata.ToolParameter, value object.Object, sp struct {
	name       string
	hasDefault bool
}, fn *object.Function) {
	switch v := value.(type) {
	case *object.String:
		tp.Description = v.StringValue()
	case *object.Dict:
		if descPair, ok := v.GetByString("description"); ok {
			if s, e := descPair.Value.AsString(); e == nil {
				tp.Description = s
			}
		}
		if typePair, ok := v.GetByString("type"); ok {
			if s, e := typePair.Value.AsString(); e == nil {
				tp.Type = normalizeParamType(s)
			}
		}
		if reqPair, ok := v.GetByString("required"); ok {
			if b, e := reqPair.Value.AsBool(); e == nil {
				tp.Required = b
			}
		}
	}
}

// inferTypeFromDefault looks at a parameter's default value expression to
// infer the MCP type. This is a best-effort heuristic based on the AST
// literal type.
func inferTypeFromDefault(fn *object.Function, paramName string) string {
	expr, ok := fn.DefaultValues[paramName]
	if !ok || expr == nil {
		return ""
	}

	switch expr.(type) {
	case *ast.Boolean:
		return "boolean"
	case *ast.IntegerLiteral:
		return "integer"
	case *ast.FloatLiteral:
		return "number"
	default:
		return ""
	}
}

// normalizeParamType normalizes type aliases to the canonical form expected by
// toolmetadata.BuildMCPTool.
func normalizeParamType(t string) string {
	switch strings.ToLower(t) {
	case "int", "integer":
		return "integer"
	case "float", "number":
		return "number"
	case "bool", "boolean":
		return "boolean"
	case "string", "str":
		return "string"
	default:
		return t // pass through (e.g. "array:string")
	}
}

// --- dict helpers ---

func dictGetString(d *object.Dict, key string) string {
	pair, ok := d.GetByString(key)
	if !ok {
		return ""
	}
	s, _ := pair.Value.AsString()
	return s
}

func dictGetBool(d *object.Dict, key string) bool {
	pair, ok := d.GetByString(key)
	if !ok {
		return false
	}
	b, _ := pair.Value.AsBool()
	return b
}

func dictGetStringList(d *object.Dict, key string) []string {
	pair, ok := d.GetByString(key)
	if !ok {
		return nil
	}
	list, ok := pair.Value.(*object.List)
	if !ok {
		return nil
	}
	var result []string
	for _, elem := range list.Elements {
		if s, e := elem.AsString(); e == nil {
			result = append(result, s)
		}
	}
	return result
}

// dictGetUIToolMeta reads the optional "ui" entry of d — a dict shaped like
// {"resourceUri": "ui://...", "visibility": ["model", "app"]} — into an
// mcp.UIToolMeta, per the MCP Apps extension (SEP-1865). Returns (nil, nil)
// when d has no "ui" key. Used by both the decorated (@mcp.tool) and
// register_request_tool registration paths; the legacy .toml format has its
// own equivalent in metadata.go (parseToolMetadata), since it decodes TOML
// rather than a scriptling dict.
//
// resourceUri is optional per spec: an "app"-only action tool — one only
// ever called by a view that's already open, such as a form submission —
// has no rendering purpose of its own and should omit it, declaring only
// visibility. At least one of the two must be present, or "ui" shouldn't
// have been set at all.
func dictGetUIToolMeta(d *object.Dict, key string) (*mcplib.UIToolMeta, error) {
	pair, ok := d.GetByString(key)
	if !ok {
		return nil, nil
	}
	uiDict, ok := pair.Value.(*object.Dict)
	if !ok {
		return nil, fmt.Errorf("%q must be a dict, got %s", key, pair.Value.Type())
	}
	resourceURI := dictGetString(uiDict, "resourceUri")
	visibility := dictGetStringList(uiDict, "visibility")
	if resourceURI == "" && len(visibility) == 0 {
		return nil, fmt.Errorf("%q requires at least one of \"resourceUri\" or \"visibility\"", key)
	}
	return &mcplib.UIToolMeta{
		ResourceURI: resourceURI,
		Visibility:  visibility,
	}, nil
}

// dictGetIcons reads the optional "icons" entry of d — a list of dicts shaped
// like {"src": "https://...", "mimeType": "image/png", "sizes": ["48x48"],
// "theme": "light"} — into a slice of mcp.Icon, per the MCP icons
// convention. Returns (nil, nil) when d has no "icons" key. Used by both the
// decorated (@mcp.tool) and register_request_tool registration paths; the
// legacy .toml format has its own equivalent in metadata.go
// (parseToolMetadata), since it decodes TOML rather than a scriptling dict.
func dictGetIcons(d *object.Dict, key string) ([]mcplib.Icon, error) {
	pair, ok := d.GetByString(key)
	if !ok {
		return nil, nil
	}
	list, ok := pair.Value.(*object.List)
	if !ok {
		return nil, fmt.Errorf("%q must be a list, got %s", key, pair.Value.Type())
	}
	icons := make([]mcplib.Icon, 0, len(list.Elements))
	for i, elem := range list.Elements {
		iconDict, ok := elem.(*object.Dict)
		if !ok {
			return nil, fmt.Errorf("%q[%d] must be a dict, got %s", key, i, elem.Type())
		}
		src := dictGetString(iconDict, "src")
		if src == "" {
			return nil, fmt.Errorf("%q[%d] requires a non-empty \"src\"", key, i)
		}
		icons = append(icons, mcplib.Icon{
			Src:      src,
			MimeType: dictGetString(iconDict, "mimeType"),
			Sizes:    dictGetStringList(iconDict, "sizes"),
			Theme:    dictGetString(iconDict, "theme"),
		})
	}
	return icons, nil
}
