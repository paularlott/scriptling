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
	extlibsmcp "github.com/paularlott/scriptling/extlibs/mcp"
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

		// A file may register resources, prompts and skills too; this scan
		// only reports tools (ScanDecoratedRegistrations reports everything).
		if kind := dictGetString(entry, "type"); kind != "" && kind != "tool" {
			continue
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

// DecoratedResource is a resource registered via @mcp.resource() in a .py file.
type DecoratedResource struct {
	URI         string
	Name        string
	Description string
	MimeType    string
	Template    bool
	FuncName    string
	Source      []byte
}

// DecoratedPrompt is a prompt registered via @mcp.prompt() in a .py file.
type DecoratedPrompt struct {
	Name        string
	Description string
	Arguments   []PromptArgument
	FuncName    string
	Source      []byte
}

// DecoratedSkill is a skill registered via @mcp.skill() in a .py file. Files
// always includes SKILL.md, produced by calling the function at scan time:
// skill content is static thereafter (matching the skills folder, which also
// only registers at startup).
type DecoratedSkill struct {
	Name   string
	Files  map[string][]byte
	Source []byte
}

// DecoratedRegistrations is everything the decorators in one .py file (or a
// whole folder) registered.
type DecoratedRegistrations struct {
	Tools     []ScannedToolEntry
	Resources []DecoratedResource
	Prompts   []DecoratedPrompt
	Skills    []DecoratedSkill
}

// ScanRegistrationsFSDual scans fsys the same way ScanToolsFSDual does, but
// returns every decorated registration kind, not just tools. Legacy .toml+.py
// pairs land in Tools as before; resources, prompts and skills only come from
// decorators.
func ScanRegistrationsFSDual(fsys fs.FS, cfg HandlerConfig) (*DecoratedRegistrations, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to read tools folder: %w", err)
	}

	// Build a set of stems that have a .toml (these are legacy tools).
	tomlStems := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") || strings.HasPrefix(e.Name(), "_") {
			continue
		}
		tomlStems[strings.TrimSuffix(e.Name(), ".toml")] = true
	}

	out := &DecoratedRegistrations{}

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

		var src []byte
		if pyData, readErr := fs.ReadFile(fsys, stem+".py"); readErr == nil {
			src = pyData
		}

		out.Tools = append(out.Tools, ScannedToolEntry{
			Name:   stem,
			Meta:   meta,
			Source: src,
			Legacy: true,
		})
	}

	// Pass 2: Decorated registrations (.py without sibling .toml).
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".py") || strings.HasPrefix(e.Name(), "_") {
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

		regs, scanErr := ScanDecoratedRegistrations(src, cfg)
		if scanErr != nil {
			return nil, fmt.Errorf("failed to scan decorated registrations in %s: %w", e.Name(), scanErr)
		}

		out.Tools = append(out.Tools, regs.Tools...)
		out.Resources = append(out.Resources, regs.Resources...)
		out.Prompts = append(out.Prompts, regs.Prompts...)
		out.Skills = append(out.Skills, regs.Skills...)
	}

	return out, nil
}

// ScanDecoratedRegistrations evaluates a .py source in a fresh interpreter
// with runtime.mcp registered, then splits __mcp_registry by entry type:
// tool entries decode as before, resource/prompt/skill entries decode into
// their own shapes. Skill functions are called here to capture their static
// content.
func ScanDecoratedRegistrations(src []byte, cfg HandlerConfig) (*DecoratedRegistrations, error) {
	p := prepareScriptling(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := p.EvalWithContext(ctx, string(src))
	if err != nil {
		return nil, fmt.Errorf("eval failed: %w", err)
	}

	registryObj, getErr := p.GetVarAsObject(extlibs.MCPRegistryVar)
	if getErr != nil {
		// No registry means no decorated registrations — not an error.
		return &DecoratedRegistrations{}, nil
	}

	registryList, ok := registryObj.(*object.List)
	if !ok {
		return nil, fmt.Errorf("%s is not a list", extlibs.MCPRegistryVar)
	}

	out := &DecoratedRegistrations{}
	for i, elem := range registryList.Elements {
		entry, ok := elem.(*object.Dict)
		if !ok {
			return nil, fmt.Errorf("registry entry %d is not a dict", i)
		}

		kind := dictGetString(entry, "type")
		switch kind {
		case "", "tool":
			tool, derr := decodeRegistryEntry(entry, src, p)
			if derr != nil {
				return nil, fmt.Errorf("registry entry %d: %w", i, derr)
			}
			out.Tools = append(out.Tools, ScannedToolEntry{
				Name:     tool.Name,
				Meta:     tool.Meta,
				Source:   src,
				FuncName: tool.FuncName,
				Legacy:   false,
			})

		case "resource":
			res, derr := decodeResourceEntry(entry, src, p)
			if derr != nil {
				return nil, fmt.Errorf("registry entry %d: %w", i, derr)
			}
			out.Resources = append(out.Resources, *res)

		case "prompt":
			prm, derr := decodePromptEntry(entry, src, p)
			if derr != nil {
				return nil, fmt.Errorf("registry entry %d: %w", i, derr)
			}
			out.Prompts = append(out.Prompts, *prm)

		case "skill":
			skill, derr := decodeSkillEntry(entry, src, p, ctx)
			if derr != nil {
				return nil, fmt.Errorf("registry entry %d: %w", i, derr)
			}
			out.Skills = append(out.Skills, *skill)

		default:
			return nil, fmt.Errorf("registry entry %d: unknown type %q", i, kind)
		}
	}

	return out, nil
}

// decodeResourceEntry converts one resource registration dict, cross-checking
// the function signature against the URI the way the tool scanner checks
// params: a template variable that is not a parameter (or a required
// parameter that is not a template variable) would fail every read, so it
// fails the scan instead.
func decodeResourceEntry(entry *object.Dict, src []byte, p *scriptling.Scriptling) (*DecoratedResource, error) {
	uri := dictGetString(entry, "uri")
	if uri == "" {
		return nil, fmt.Errorf("missing or empty 'uri'")
	}
	funcName := dictGetString(entry, "func")
	if funcName == "" {
		return nil, fmt.Errorf("resource %q: missing 'func'", uri)
	}
	name := dictGetString(entry, "name")
	if name == "" {
		name = uri
	}
	template := dictGetBool(entry, "template")

	if err := crossCheckResourceSignature(uri, funcName, template, p); err != nil {
		return nil, fmt.Errorf("resource %q: %w", uri, err)
	}

	return &DecoratedResource{
		URI:         uri,
		Name:        name,
		Description: dictGetString(entry, "description"),
		MimeType:    dictGetString(entry, "mime_type"),
		Template:    template,
		FuncName:    funcName,
		Source:      src,
	}, nil
}

// crossCheckResourceSignature validates a decorated resource function's
// parameters against its URI. For a template, every {var} must be a function
// parameter and every parameter without a default must be a {var}. For a
// static resource every parameter must have a default (nothing is passed).
func crossCheckResourceSignature(uri, funcName string, template bool, p *scriptling.Scriptling) error {
	sigParams, err := functionSignature(p, funcName)
	if err != nil {
		return err
	}

	if !template {
		for _, sp := range sigParams {
			if !sp.hasDefault {
				return fmt.Errorf("%s takes required parameter %q but a static resource is read with no arguments; give it a default or use template=True", funcName, sp.name)
			}
		}
		return nil
	}

	path := uri
	if idx := strings.Index(uri, "://"); idx >= 0 {
		path = uri[idx+3:]
	}
	vars := extractTemplateVars(strings.Split(path, "/"))
	isVar := map[string]bool{}
	for _, v := range vars {
		isVar[v] = true
		if !sigParams.has(v) {
			return fmt.Errorf("template variable %q does not match any parameter of function %q", v, funcName)
		}
	}
	for _, sp := range sigParams {
		if !sp.hasDefault && !isVar[sp.name] {
			return fmt.Errorf("parameter %q of function %s is required but not a template variable of %q", sp.name, funcName, uri)
		}
	}
	return nil
}

// sigParam is one parameter of a scanned function signature.
type sigParam struct {
	name       string
	hasDefault bool
}

// sigParams is a name-keyed view over a signature.
type sigParams []sigParam

func (s sigParams) has(name string) bool {
	for _, sp := range s {
		if sp.name == name {
			return true
		}
	}
	return false
}

// functionSignature looks up a function by name and returns its parameters
// with whether each has a default.
func functionSignature(p *scriptling.Scriptling, funcName string) (sigParams, error) {
	fnObj, err := p.GetVarAsObject(funcName)
	if err != nil {
		return nil, fmt.Errorf("function %q not found in environment", funcName)
	}
	fn, ok := fnObj.(*object.Function)
	if !ok {
		return nil, fmt.Errorf("%q is not a function (got %s)", funcName, fnObj.Type())
	}
	var out sigParams
	for _, param := range fn.Parameters {
		name := param.Value()
		_, hasDefault := fn.DefaultValues[name]
		out = append(out, sigParam{name: name, hasDefault: hasDefault})
	}
	return out, nil
}

// decodePromptEntry converts one prompt registration dict. When no arguments
// metadata was given, the arguments are inferred from the function signature
// (a parameter without a default is required).
func decodePromptEntry(entry *object.Dict, src []byte, p *scriptling.Scriptling) (*DecoratedPrompt, error) {
	name := dictGetString(entry, "name")
	if name == "" {
		return nil, fmt.Errorf("missing or empty 'name'")
	}

	var args []PromptArgument
	if pair, ok := entry.GetByString("arguments"); ok {
		sig, err := functionSignature(p, name)
		if err != nil {
			return nil, err
		}
		list, ok := pair.Value.(*object.List)
		if !ok {
			return nil, fmt.Errorf("prompt %q: arguments must be a list", name)
		}
		for i, elem := range list.Elements {
			d, ok := elem.(*object.Dict)
			if !ok {
				return nil, fmt.Errorf("prompt %q: arguments[%d] must be a dict", name, i)
			}
			argName := dictGetString(d, "name")
			if argName == "" {
				return nil, fmt.Errorf("prompt %q: arguments[%d] missing 'name'", name, i)
			}
			required := false
			if pair, ok := d.GetByString("required"); ok {
				if b, e := pair.Value.AsBool(); e == nil {
					required = b
				}
			}
			if !sig.has(argName) {
				return nil, fmt.Errorf("argument %q does not match any parameter of function %q", argName, name)
			}
			args = append(args, PromptArgument{
				Name:        argName,
				Description: dictGetString(d, "description"),
				Required:    required,
			})
		}
	} else {
		// Infer from the function signature.
		sig, err := functionSignature(p, name)
		if err != nil {
			return nil, err
		}
		for _, sp := range sig {
			args = append(args, PromptArgument{Name: sp.name, Required: !sp.hasDefault})
		}
	}

	return &DecoratedPrompt{
		Name:        name,
		Description: dictGetString(entry, "description"),
		Arguments:   args,
		FuncName:    dictGetString(entry, "func"),
		Source:      src,
	}, nil
}

// decodeSkillEntry converts one skill registration dict by calling the
// decorated function: its string return is the SKILL.md content, and the
// decorator's files dict supplies the supporting files.
func decodeSkillEntry(entry *object.Dict, src []byte, p *scriptling.Scriptling, ctx context.Context) (*DecoratedSkill, error) {
	name := dictGetString(entry, "name")
	if name == "" {
		return nil, fmt.Errorf("missing or empty 'name'")
	}
	funcName := dictGetString(entry, "func")
	if funcName == "" {
		funcName = name
	}

	result, callErr := p.CallFunctionWithContext(ctx, funcName, scriptling.Kwargs(nil))

	// A plain string return is the SKILL.md; mcp.tool.return_string sets
	// __mcp_response instead (raising SystemExit), so honour it too.
	skillMD := ""
	if respObj, getErr := p.GetVarAsObject(extlibsmcp.MCPResponseVarName); getErr == nil {
		if s, ok := respObj.(*object.String); ok {
			skillMD = s.StringValue()
		}
	}
	if skillMD == "" && callErr == nil {
		if s, ok := result.(*object.String); ok {
			skillMD = s.StringValue()
		} else if result != nil {
			return nil, fmt.Errorf("skill %q: %s must return the SKILL.md content as a string, got %s", name, funcName, result.Type())
		}
	}
	if callErr != nil {
		return nil, fmt.Errorf("skill %q: running %q failed: %w", name, funcName, callErr)
	}
	if strings.TrimSpace(skillMD) == "" {
		return nil, fmt.Errorf("skill %q: %s returned empty SKILL.md content", name, funcName)
	}

	files := map[string][]byte{"SKILL.md": []byte(skillMD)}
	if pair, ok := entry.GetByString("files"); ok {
		filesDict, ok := pair.Value.(*object.Dict)
		if !ok {
			return nil, fmt.Errorf("skill %q: files must be a dict", name)
		}
		for _, fp := range filesDict.Pairs {
			fileName, _ := fp.Key.AsString()
			content, e := fp.Value.AsString()
			if e != nil || fileName == "" {
				return nil, fmt.Errorf("skill %q: files keys and values must be strings", name)
			}
			if fileName == "SKILL.md" {
				return nil, fmt.Errorf("skill %q: files must not contain SKILL.md (returned by %s)", name, funcName)
			}
			files[fileName] = []byte(content)
		}
	}

	return &DecoratedSkill{Name: name, Files: files, Source: src}, nil
}
