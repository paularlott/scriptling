package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	mcplib "github.com/paularlott/mcp"
	"github.com/paularlott/mcp/toolmetadata"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/conversion"
	extlibsmcp "github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/object"
)

// SplitHandlerRef splits a "module.function" handler reference at the last
// dot: module names may themselves be dotted (a module in a subdirectory,
// such as "routes.me"), while the function or class name is always a single
// identifier. Cutting at the first dot instead would try to import "routes"
// for a "routes.me.me" reference, which is not a module.
func SplitHandlerRef(ref string) (module, fn string, ok bool) {
	idx := strings.LastIndex(ref, ".")
	if idx <= 0 || idx == len(ref)-1 {
		return "", "", false
	}
	return ref[:idx], ref[idx+1:], true
}

// refHandlerResult is the outcome of running a request-scoped handler: the
// function's return value, an explicit response set via tool.return_* (which
// stops execution with SystemExit, leaving callErr set), or a failure.
type refHandlerResult struct {
	value    object.Object
	explicit string
	callErr  error
}

// runHandlerRef imports the handler's module into a fresh evaluator and calls
// the named function with kwargs. It is the shared backbone of the
// request-scoped tool, resource and prompt handlers: like every other handler
// reference, resolution happens per call, so the middleware interpreter that
// registered the ref does not need to outlive the request.
func runHandlerRef(ctx context.Context, cfg HandlerConfig, handlerRef string, kwargs map[string]any) (refHandlerResult, error) {
	libName, _, ok := SplitHandlerRef(handlerRef)
	if !ok {
		return refHandlerResult{}, fmt.Errorf("invalid handler reference %q", handlerRef)
	}

	p := prepareScriptling(cfg, nil)
	if err := p.ImportWithContext(ctx, libName); err != nil {
		return refHandlerResult{}, fmt.Errorf("failed to import library %s: %w", libName, err)
	}

	// Seed __mcp_params so tool.get_string() also works inside ref-based
	// handlers, mirroring what RunToolScript does for script-backed tools.
	if len(kwargs) > 0 {
		paramsDict := &object.Dict{Pairs: make(map[string]object.DictPair, len(kwargs))}
		for k, v := range kwargs {
			paramsDict.SetByString(k, object.NewString(fmt.Sprintf("%v", v)))
		}
		p.SetObjectVar(extlibsmcp.MCPParamsVarName, paramsDict)
	}

	// Call through the full dotted ref: importing the module binds it as a
	// dict in the global env, and the evaluator resolves "mod.fn" paths.
	value, callErr := p.CallFunctionWithContext(ctx, handlerRef, scriptling.Kwargs(kwargs))

	// tool.return_* set __mcp_response and stopped execution; the stored
	// value outlives the call on the interpreter's environment.
	if respObj, err := p.GetVarAsObject(extlibsmcp.MCPResponseVarName); err == nil {
		if s, ok := respObj.(*object.String); ok && s.StringValue() != "" {
			return refHandlerResult{explicit: s.StringValue(), callErr: callErr}, nil
		}
	}

	return refHandlerResult{value: value, callErr: callErr}, nil
}

// ── tools ────────────────────────────────────────────────────────────────────

type requestTool struct {
	tool    mcplib.MCPTool
	handler string
}

type requestToolProvider struct {
	tools []requestTool
	cfg   HandlerConfig
}

// BuildRequestToolProvider builds an mcp ToolProvider from the
// register_request_tool registrations a middleware made. Registration dicts
// carry name, handler and optional description / params / keywords /
// discoverable; anything malformed is a build error and fails the request.
func BuildRequestToolProvider(entries []*object.Dict, cfg HandlerConfig) (mcplib.ToolProvider, error) {
	provider := &requestToolProvider{cfg: cfg}
	for _, entry := range entries {
		name := dictGetString(entry, "name")
		handler := dictGetString(entry, "handler")
		if name == "" || handler == "" {
			return nil, fmt.Errorf("register_request_tool: name and handler are required")
		}

		meta := &toolmetadata.ToolMetadata{
			Description: dictGetString(entry, "description"),
		}
		if pair, ok := entry.GetByString("params"); ok {
			params, err := requestToolParams(pair.Value)
			if err != nil {
				return nil, fmt.Errorf("register_request_tool %q: %w", name, err)
			}
			meta.Parameters = params
		}
		if pair, ok := entry.GetByString("keywords"); ok {
			if list, ok := pair.Value.(*object.List); ok {
				for _, el := range list.Elements {
					if s, err := el.AsString(); err == nil {
						meta.Keywords = append(meta.Keywords, s)
					}
				}
			}
		}
		if pair, ok := entry.GetByString("discoverable"); ok {
			if b, err := pair.Value.AsBool(); err == nil {
				meta.Discoverable = b
			}
		}

		tool, err := toolmetadata.BuildMCPTool(name, meta)
		if err != nil {
			return nil, err
		}
		provider.tools = append(provider.tools, requestTool{tool: tool.ToMCPTool(), handler: handler})
	}
	return provider, nil
}

// requestToolParams converts a self-describing params dict — values are a
// string (description) or a dict with type/description/required — into tool
// metadata. Unlike the decorator form there is no function signature to
// cross-reference, so required defaults to False like the TOML format.
func requestToolParams(paramsObj object.Object) ([]toolmetadata.ToolParameter, error) {
	dict, ok := paramsObj.(*object.Dict)
	if !ok {
		return nil, fmt.Errorf("params must be a dict, got %s", paramsObj.Type())
	}
	var out []toolmetadata.ToolParameter
	for _, pair := range dict.Pairs {
		key, err := pair.Key.AsString()
		if err != nil || key == "" {
			return nil, fmt.Errorf("params keys must be non-empty strings")
		}
		tp := toolmetadata.ToolParameter{Name: key, Type: "string"}
		switch v := pair.Value.(type) {
		case *object.String:
			tp.Description = v.StringValue()
		case *object.Dict:
			if p, ok := v.GetByString("description"); ok {
				if s, err := p.Value.AsString(); err == nil {
					tp.Description = s
				}
			}
			if p, ok := v.GetByString("type"); ok {
				if s, err := p.Value.AsString(); err == nil {
					tp.Type = normalizeParamType(s)
				}
			}
			if p, ok := v.GetByString("required"); ok {
				if b, err := p.Value.AsBool(); err == nil {
					tp.Required = b
				}
			}
		default:
			return nil, fmt.Errorf("parameter %q: metadata must be a string or dict, got %s", key, pair.Value.Type())
		}
		out = append(out, tp)
	}
	return out, nil
}

func (p *requestToolProvider) GetTools(ctx context.Context) ([]mcplib.MCPTool, error) {
	tools := make([]mcplib.MCPTool, 0, len(p.tools))
	for _, t := range p.tools {
		tools = append(tools, t.tool)
	}
	return tools, nil
}

func (p *requestToolProvider) ExecuteTool(ctx context.Context, name string, params map[string]any) (*mcplib.ToolResponse, error) {
	for _, t := range p.tools {
		if t.tool.Name != name {
			continue
		}
		result, err := runHandlerRef(ctx, p.cfg, t.handler, params)
		if err != nil {
			return nil, mcplib.NewToolErrorInternal(err.Error())
		}
		// tool.return_error sets a response and exits non-zero.
		if result.explicit != "" && result.callErr != nil {
			return nil, mcplib.NewToolErrorInternal(result.explicit)
		}
		if result.explicit != "" {
			return mcplib.NewToolResponseText(result.explicit), nil
		}
		if result.callErr != nil {
			return nil, mcplib.NewToolErrorInternal(result.callErr.Error())
		}
		return toolResultToResponse(result.value)
	}
	return nil, nil // not handled
}

// ── resources ────────────────────────────────────────────────────────────────

type requestResource struct {
	uri         string // static URI, or the template when template is set
	name        string
	description string
	template    bool
	varNames    []string
	pattern     *regexp.Regexp
	handler     string
	mimeType    string
}

type requestResourceProvider struct {
	resources []requestResource
	cfg       HandlerConfig
}

// BuildRequestResourceProvider builds an mcp ResourceProvider from the
// register_request_resource registrations a middleware made. Registration
// dicts carry uri, handler, name and optional description / mime_type /
// template. Providers receive only the raw URI on read, so templates are
// matched (and their variables extracted) here rather than by the server.
func BuildRequestResourceProvider(entries []*object.Dict, cfg HandlerConfig) (mcplib.ResourceProvider, error) {
	provider := &requestResourceProvider{cfg: cfg}
	for _, entry := range entries {
		res := requestResource{
			uri:         dictGetString(entry, "uri"),
			name:        dictGetString(entry, "name"),
			description: dictGetString(entry, "description"),
			handler:     dictGetString(entry, "handler"),
			mimeType:    dictGetString(entry, "mime_type"),
		}
		if res.uri == "" || res.name == "" || res.handler == "" {
			return nil, fmt.Errorf("register_request_resource: uri, name and handler are required")
		}
		if pair, ok := entry.GetByString("template"); ok {
			if b, err := pair.Value.AsBool(); err == nil {
				res.template = b
			}
		}
		if res.template {
			varNames, pattern := parseRequestResourceTemplate(res.uri)
			if pattern == nil {
				return nil, fmt.Errorf("register_request_resource %q: malformed URI template", res.uri)
			}
			res.varNames, res.pattern = varNames, pattern
		}
		provider.resources = append(provider.resources, res)
	}
	return provider, nil
}

// parseRequestResourceTemplate compiles a {var} URI template into an anchored
// regexp, mirroring the server's own template matching.
func parseRequestResourceTemplate(template string) ([]string, *regexp.Regexp) {
	var (
		b        strings.Builder
		varNames []string
	)
	b.WriteByte('^')
	i := 0
	for i < len(template) {
		if template[i] == '{' {
			end := strings.IndexByte(template[i:], '}')
			if end == -1 {
				b.WriteString(regexp.QuoteMeta(template[i:]))
				break
			}
			varNames = append(varNames, strings.TrimSpace(template[i+1:i+end]))
			b.WriteString("(.+)")
			i += end + 1
			continue
		}
		next := strings.IndexByte(template[i:], '{')
		if next == -1 {
			b.WriteString(regexp.QuoteMeta(template[i:]))
			break
		}
		b.WriteString(regexp.QuoteMeta(template[i : i+next]))
		i += next
	}
	b.WriteByte('$')
	re, err := regexp.Compile(b.String())
	if err != nil {
		return varNames, nil
	}
	return varNames, re
}

func (p *requestResourceProvider) GetResources(ctx context.Context) (*mcplib.ProvidedResources, error) {
	out := &mcplib.ProvidedResources{}
	for _, res := range p.resources {
		if res.template {
			out.Templates = append(out.Templates, mcplib.NewResourceTemplate(res.uri, res.name, res.description, res.mimeType).ToMCPResourceTemplate())
		} else {
			out.Resources = append(out.Resources, mcplib.NewResource(res.uri, res.name, res.description, res.mimeType).ToMCPResource())
		}
	}
	return out, nil
}

func (p *requestResourceProvider) ReadResource(ctx context.Context, uri string) (*mcplib.ResourceResponse, error) {
	for _, res := range p.resources {
		kwargs := map[string]any{"__uri": uri}
		if res.template {
			m := res.pattern.FindStringSubmatch(uri)
			if m == nil {
				continue
			}
			for i, name := range res.varNames {
				if i+1 < len(m) {
					kwargs[name] = m[i+1]
				}
			}
		} else if res.uri != uri {
			continue
		}

		result, err := runHandlerRef(ctx, p.cfg, res.handler, kwargs)
		if err != nil {
			return nil, mcplib.NewToolErrorInternal(err.Error())
		}
		if result.explicit != "" {
			if result.callErr != nil {
				return nil, mcplib.NewToolErrorInternal(result.explicit)
			}
			return mcplib.NewResourceResponseText(uri, result.explicit, orDefault(res.mimeType, "text/plain")), nil
		}
		if result.callErr != nil {
			return nil, mcplib.NewToolErrorInternal(result.callErr.Error())
		}
		content, mime := resourceContentOf(result.value, res.mimeType)
		return mcplib.NewResourceResponseText(uri, content, mime), nil
	}
	return nil, mcplib.ErrUnknownResource
}

// resourceContentOf maps a handler's return value to resource text: strings
// pass through, anything else is JSON encoded with a JSON content type.
func resourceContentOf(result object.Object, mimeType string) (string, string) {
	if s, errObj := result.AsString(); errObj == nil {
		return s, orDefault(mimeType, "text/plain")
	}
	data, err := json.Marshal(conversion.ToGo(result))
	if err != nil {
		return result.Inspect(), mimeType
	}
	return string(data), orDefault(mimeType, "application/json")
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// ── prompts ──────────────────────────────────────────────────────────────────

type requestPrompt struct {
	prompt  mcplib.MCPPrompt
	handler string
}

type requestPromptProvider struct {
	prompts []requestPrompt
	cfg     HandlerConfig
}

// BuildRequestPromptProvider builds an mcp PromptProvider from the
// register_request_prompt registrations a middleware made. Registration dicts
// carry name, handler and optional description / arguments.
func BuildRequestPromptProvider(entries []*object.Dict, cfg HandlerConfig) (mcplib.PromptProvider, error) {
	provider := &requestPromptProvider{cfg: cfg}
	for _, entry := range entries {
		name := dictGetString(entry, "name")
		handler := dictGetString(entry, "handler")
		if name == "" || handler == "" {
			return nil, fmt.Errorf("register_request_prompt: name and handler are required")
		}

		builder := mcplib.NewPrompt(name, dictGetString(entry, "description"))
		if pair, ok := entry.GetByString("arguments"); ok {
			if list, ok := pair.Value.(*object.List); ok {
				for _, el := range list.Elements {
					argDict, ok := el.(*object.Dict)
					if !ok {
						return nil, fmt.Errorf("register_request_prompt %q: arguments entries must be dicts", name)
					}
					argName := dictGetString(argDict, "name")
					if argName == "" {
						return nil, fmt.Errorf("register_request_prompt %q: argument entries need a name", name)
					}
					required := false
					if p, ok := argDict.GetByString("required"); ok {
						if b, err := p.Value.AsBool(); err == nil {
							required = b
						}
					}
					builder.Argument(argName, dictGetString(argDict, "description"), required)
				}
			}
		}
		provider.prompts = append(provider.prompts, requestPrompt{prompt: builder.ToMCPPrompt(), handler: handler})
	}
	return provider, nil
}

func (p *requestPromptProvider) GetPrompts(ctx context.Context) ([]mcplib.MCPPrompt, error) {
	prompts := make([]mcplib.MCPPrompt, 0, len(p.prompts))
	for _, pr := range p.prompts {
		prompts = append(prompts, pr.prompt)
	}
	return prompts, nil
}

func (p *requestPromptProvider) GetPrompt(ctx context.Context, name string, args map[string]string) (*mcplib.PromptResponse, error) {
	for _, pr := range p.prompts {
		if pr.prompt.Name != name {
			continue
		}
		kwargs := make(map[string]any, len(args))
		for k, v := range args {
			kwargs[k] = v
		}
		result, err := runHandlerRef(ctx, p.cfg, pr.handler, kwargs)
		if err != nil {
			return nil, mcplib.NewToolErrorInternal(err.Error())
		}
		if result.explicit != "" {
			if result.callErr != nil {
				return nil, mcplib.NewToolErrorInternal(result.explicit)
			}
			return mcplib.NewPromptResponseText(result.explicit), nil
		}
		if result.callErr != nil {
			return nil, mcplib.NewToolErrorInternal(result.callErr.Error())
		}
		return promptResponseOf(result.value)
	}
	return nil, mcplib.ErrUnknownPrompt
}

// promptResponseOf maps a handler's return value to a prompt response: a
// string is a single user message; a dict with a "messages" list of
// {"role", "content"} builds a multi-message prompt.
func promptResponseOf(result object.Object) (*mcplib.PromptResponse, error) {
	if s, errObj := result.AsString(); errObj == nil {
		return mcplib.NewPromptResponseText(s), nil
	}
	if dict, ok := result.(*object.Dict); ok {
		if pair, ok := dict.GetByString("messages"); ok {
			if list, ok := pair.Value.(*object.List); ok {
				var messages []mcplib.PromptMessage
				for _, el := range list.Elements {
					msgDict, ok := el.(*object.Dict)
					if !ok {
						continue
					}
					role := mcplib.PromptRoleUser
					if p, ok := msgDict.GetByString("role"); ok {
						if s, errObj := p.Value.AsString(); errObj == nil && s == "assistant" {
							role = mcplib.PromptRoleAssistant
						}
					}
					content := ""
					if p, ok := msgDict.GetByString("content"); ok {
						if s, errObj := p.Value.AsString(); errObj == nil {
							content = s
						}
					}
					messages = append(messages, mcplib.NewPromptTextMessage(role, content))
				}
				if len(messages) > 0 {
					return mcplib.NewPromptResponseMessages(messages...), nil
				}
			}
		}
	}
	return mcplib.NewPromptResponseText(result.Inspect()), nil
}
