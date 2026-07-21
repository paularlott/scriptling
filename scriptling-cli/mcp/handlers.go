package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/paularlott/logger"
	mcplib "github.com/paularlott/mcp"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/conversion"
	extlibsmcp "github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	"github.com/paularlott/scriptling/object"
	scriptlingplugin "github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/scriptling-cli/bootstrap"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
)

// HandlerConfig bundles the runtime context needed to build script-backed MCP
// tool, resource, and prompt handlers. With the exception of LibDirs, every
// field is optional: a nil SecretRegistry uses an empty one, a nil Logger uses
// a null logger, and a nil PackLoader / PluginManager simply skips pack / plugin
// wiring. Built-in libraries can be selectively disabled via DisabledLibs and
// the filesystem access of os/pathlib/glob/fs/grep/sed can be constrained with
// AllowedPaths (nil means unrestricted, matching scriptling-cli/server's default).
type HandlerConfig struct {
	LibDirs        []string
	AllowedPaths   []string                     // nil = unrestricted
	DisabledLibs   []string                     // nil = all built-ins enabled
	SecretRegistry *secretprovider.Registry     // nil = empty registry
	Logger         logger.Logger                // nil = null logger
	PackLoader     *pack.Loader                 // nil = no pack loader
	PluginManager  *scriptlingplugin.Manager    // nil = no plugins
	DockerSock     string                       // empty = default socket
	PodmanSock     string                       // empty = default socket
	SetupHook      func(*scriptling.Scriptling) // optional extra setup applied after standard libs
}

// HandlerOption configures a HandlerConfig.
type HandlerOption func(*HandlerConfig)

// WithPlugins returns an option that attaches a plugin manager to every handler
// built from the resulting HandlerConfig.
func WithPlugins(pm *scriptlingplugin.Manager) HandlerOption {
	return func(c *HandlerConfig) { c.PluginManager = pm }
}

// WithPackLoader returns an option that chains a pack loader behind the
// filesystem library loader of every handler's interpreter.
func WithPackLoader(pl *pack.Loader) HandlerOption {
	return func(c *HandlerConfig) { c.PackLoader = pl }
}

// WithSecrets returns an option that supplies the secret registry exposed to
// scripts via the secret library.
func WithSecrets(r *secretprovider.Registry) HandlerOption {
	return func(c *HandlerConfig) { c.SecretRegistry = r }
}

// WithLogger returns an option that supplies the logger used by the logging
// library inside the script interpreter.
func WithLogger(l logger.Logger) HandlerOption {
	return func(c *HandlerConfig) { c.Logger = l }
}

// WithAllowedPaths returns an option that constrains os/pathlib/glob/fs/grep/sed
// to the given paths. nil means unrestricted, an empty slice denies all.
func WithAllowedPaths(paths []string) HandlerOption {
	return func(c *HandlerConfig) { c.AllowedPaths = paths }
}

// WithDisabledLibs returns an option that disables the named built-in libraries.
func WithDisabledLibs(names []string) HandlerOption {
	return func(c *HandlerConfig) { c.DisabledLibs = names }
}

// WithDockerSock returns an option that sets the Docker socket path used by
// the container library inside the script interpreter.
func WithDockerSock(sock string) HandlerOption {
	return func(c *HandlerConfig) { c.DockerSock = sock }
}

// WithPodmanSock returns an option that sets the Podman socket path used by
// the container library inside the script interpreter.
func WithPodmanSock(sock string) HandlerOption {
	return func(c *HandlerConfig) { c.PodmanSock = sock }
}

// WithSetupHook returns an option that registers a callback invoked after the
// standard library set is registered on each fresh interpreter. Hosts use this
// to expose their own libraries to served scripts.
func WithSetupHook(fn func(*scriptling.Scriptling)) HandlerOption {
	return func(c *HandlerConfig) { c.SetupHook = fn }
}

// NewHandlerConfig builds a HandlerConfig from the given lib dirs plus any
// number of options. It is the recommended entry point for hosts constructing
// tool/resource/prompt handlers.
func NewHandlerConfig(libDirs []string, opts ...HandlerOption) HandlerConfig {
	c := HandlerConfig{LibDirs: libDirs}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// prepareScriptling configures a fresh Scriptling instance using cfg. It is the
// shared backbone of every script-backed handler: it applies setup.Scriptling,
// registers plugin libraries when a plugin manager is present, chains the pack
// loader behind the filesystem loader when one is present, and finally invokes
// the optional SetupHook for host-specific extras.
func prepareScriptling(cfg HandlerConfig, extraLibDirs []string) *scriptling.Scriptling {
	log := cfg.Logger
	if log == nil {
		log = logger.NewNullLogger()
	}
	registry := cfg.SecretRegistry
	if registry == nil {
		registry = secretprovider.NewRegistry()
	}

	libDirs := append(append([]string{}, cfg.LibDirs...), extraLibDirs...)

	p := scriptling.New()
	setup.Scriptling(p, libDirs, false, cfg.AllowedPaths, cfg.DisabledLibs, registry, log, cfg.DockerSock, cfg.PodmanSock)
	if cfg.PluginManager != nil {
		scriptlingplugin.RegisterLibraries(p, cfg.PluginManager)
	}
	bootstrap.ApplyPackLoader(p, cfg.PackLoader)
	if cfg.SetupHook != nil {
		cfg.SetupHook(p)
	}
	return p
}

// FileReader returns a read function over os.ReadFile, for use with the
// static handler builders.
func FileReader(path string) func() ([]byte, error) {
	return func() ([]byte, error) { return os.ReadFile(path) }
}

// BuildToolHandler reads the script once at registration time and returns a
// ToolHandler that runs a fresh interpreter per invocation. The script receives
// its parameters via mcp.tool.get_* helpers and returns its result via
// mcp.tool.return_string / return_object / return_error.
//
// Tool scripts resolve imports only via the configured library dirs (and pack
// loader); pass the tools dir in cfg.LibDirs if sibling imports are needed.
func BuildToolHandler(scriptPath string, cfg HandlerConfig) (mcplib.ToolHandler, error) {
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read script %s: %w", scriptPath, err)
	}
	return BuildToolHandlerSource(script, cfg), nil
}

// BuildToolHandlerSource is BuildToolHandler for in-memory script source (e.g.
// from a pack bundle).
func BuildToolHandlerSource(src []byte, cfg HandlerConfig) mcplib.ToolHandler {
	return func(ctx context.Context, req *mcplib.ToolRequest) (*mcplib.ToolResponse, error) {
		p := prepareScriptling(cfg, nil)
		params := req.Args()

		response, exitCode, err := extlibsmcp.RunToolScript(ctx, p, string(src), params)

		// If the script produced an explicit response (via return_error,
		// return_string, etc.), return it to the client. return_error sets a
		// response AND exits non-zero, so check for a response before treating
		// non-zero exit as a failure.
		if response != "" {
			if exitCode != 0 {
				return nil, mcplib.NewToolErrorInternal(response)
			}
			return mcplib.NewToolResponseText(response), nil
		}
		if err != nil {
			return nil, fmt.Errorf("script execution failed: %w", err)
		}
		if exitCode != 0 {
			return nil, fmt.Errorf("script exited with code %d", exitCode)
		}
		return mcplib.NewToolResponseText(""), nil
	}
}

// BuildToolHandlerFunc builds a ToolHandler for a decorated tool. Unlike
// BuildToolHandlerSource (which runs the entire script as a top-level program),
// this handler evaluates the source to define the function, then calls the named
// function with the MCP request parameters mapped to keyword arguments. The
// function's return value becomes the tool response:
//   - string → text response
//   - dict/list → JSON response
//   - None/null → empty text response
//   - exception → error response
func BuildToolHandlerFunc(src []byte, funcName string, cfg HandlerConfig) mcplib.ToolHandler {
	return func(ctx context.Context, req *mcplib.ToolRequest) (*mcplib.ToolResponse, error) {
		p := prepareScriptling(cfg, nil)

		// Evaluate the source to define the decorated functions.
		_, evalErr := p.EvalWithContext(ctx, string(src))
		if evalErr != nil {
			return nil, fmt.Errorf("failed to load tool source: %w", evalErr)
		}

		// Build kwargs from the MCP request params.
		params := req.Args()
		kwargs := scriptling.Kwargs(params)

		// Call the tool function.
		result, callErr := p.CallFunctionWithContext(ctx, funcName, kwargs)
		if callErr != nil {
			return nil, mcplib.NewToolErrorInternal(callErr.Error())
		}

		// Map the return value to an MCP tool response.
		return toolResultToResponse(result)
	}
}

// toolResultToResponse converts a scriptling return value from a decorated tool
// function into an MCP ToolResponse.
func toolResultToResponse(result object.Object) (*mcplib.ToolResponse, error) {
	if result == nil {
		return mcplib.NewToolResponseText(""), nil
	}

	switch v := result.(type) {
	case *object.Null:
		return mcplib.NewToolResponseText(""), nil

	case *object.String:
		return mcplib.NewToolResponseText(v.StringValue()), nil

	case *object.Integer:
		s, _ := v.CoerceString()
		return mcplib.NewToolResponseText(s), nil

	case *object.Float:
		s, _ := v.CoerceString()
		return mcplib.NewToolResponseText(s), nil

	case *object.Boolean:
		s, _ := v.CoerceString()
		return mcplib.NewToolResponseText(s), nil

	case *object.Dict, *object.List:
		goVal := conversion.ToGo(result)
		jsonBytes, err := json.Marshal(goVal)
		if err != nil {
			return nil, mcplib.NewToolErrorInternal(fmt.Sprintf("failed to encode response as JSON: %v", err))
		}
		return mcplib.NewToolResponseText(string(jsonBytes)), nil

	case *object.Error:
		return nil, mcplib.NewToolErrorInternal(v.Message)

	case *object.Exception:
		msg := v.Message
		if msg == "" {
			msg = "tool raised an exception"
		}
		return nil, mcplib.NewToolErrorInternal(msg)

	default:
		// Fallback: coerce to string.
		if s, err := result.CoerceString(); err == nil {
			return mcplib.NewToolResponseText(s), nil
		}
		return mcplib.NewToolResponseText(""), nil
	}
}

// BuildStaticResourceHandler serves content verbatim: text for UTF-8 data,
// base64 blob otherwise. read is called on every request so content is always
// current (e.g. FileReader for disk, a bundle read for packs).
func BuildStaticResourceHandler(read func() ([]byte, error), uri, mimeType string) mcplib.ResourceHandler {
	return func(ctx context.Context, req *mcplib.ResourceRequest) (*mcplib.ResourceResponse, error) {
		data, err := read()
		if err != nil {
			return nil, mcplib.NewToolErrorInternal(fmt.Sprintf("failed to read resource: %v", err))
		}
		if utf8.Valid(data) {
			mime := mimeType
			if mime == "" {
				mime = "text/plain"
			}
			return mcplib.NewResourceResponseText(uri, string(data), mime), nil
		}
		mime := mimeType
		if mime == "" {
			mime = "application/octet-stream"
		}
		return mcplib.NewResourceResponseBlob(uri, data, mime), nil
	}
}

// BuildResourceScriptHandler builds a ResourceHandler that runs a Scriptling
// script. The script receives the request URI as "__uri" and any template
// variables as parameters (read via mcp.tool.get_string), and returns the
// resource content via mcp.tool.return_string / return_object.
func BuildResourceScriptHandler(scriptPath, mimeType string, cfg HandlerConfig) (mcplib.ResourceHandler, error) {
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read script %s: %w", scriptPath, err)
	}
	return BuildResourceScriptHandlerSource(script, mimeType, cfg), nil
}

// BuildResourceScriptHandlerSource is BuildResourceScriptHandler for in-memory
// script source (e.g. from a pack bundle).
func BuildResourceScriptHandlerSource(src []byte, mimeType string, cfg HandlerConfig) mcplib.ResourceHandler {
	return func(ctx context.Context, req *mcplib.ResourceRequest) (*mcplib.ResourceResponse, error) {
		p := prepareScriptling(cfg, nil)

		params := map[string]any{"__uri": req.URI()}
		for k, v := range req.Vars() {
			params[k] = v
		}
		response, exitCode, err := extlibsmcp.RunToolScript(ctx, p, string(src), params)
		if response != "" {
			if exitCode != 0 {
				return nil, mcplib.NewToolErrorInternal(response)
			}
			return mcplib.NewResourceResponseText(req.URI(), response, mimeType), nil
		}
		if err != nil {
			return nil, fmt.Errorf("resource script failed: %w", err)
		}
		return mcplib.NewResourceResponseText(req.URI(), "", mimeType), nil
	}
}

// BuildStaticPromptHandler returns a PromptHandler whose single user message is
// the content returned by read (called fresh each time).
func BuildStaticPromptHandler(read func() ([]byte, error)) mcplib.PromptHandler {
	return func(ctx context.Context, req *mcplib.PromptRequest) (*mcplib.PromptResponse, error) {
		data, err := read()
		if err != nil {
			return nil, mcplib.NewToolErrorInternal(fmt.Sprintf("failed to read prompt: %v", err))
		}
		return mcplib.NewPromptResponseText(string(data)), nil
	}
}

// BuildPromptScriptHandler builds a PromptHandler that runs a Scriptling script.
// The script receives the prompt arguments as parameters and returns messages
// via mcp.tool.return_object({"messages": [{"role": ..., "content": ...}]})
// (a bare string is treated as a single user message).
func BuildPromptScriptHandler(scriptPath string, cfg HandlerConfig) (mcplib.PromptHandler, error) {
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read script %s: %w", scriptPath, err)
	}
	return BuildPromptScriptHandlerSource(script, cfg), nil
}

// BuildPromptScriptHandlerSource is BuildPromptScriptHandler for in-memory
// script source (e.g. from a pack bundle).
func BuildPromptScriptHandlerSource(src []byte, cfg HandlerConfig) mcplib.PromptHandler {
	return func(ctx context.Context, req *mcplib.PromptRequest) (*mcplib.PromptResponse, error) {
		p := prepareScriptling(cfg, nil)

		params := map[string]any{}
		for k, v := range req.Args() {
			params[k] = v
		}
		response, exitCode, err := extlibsmcp.RunToolScript(ctx, p, string(src), params)
		if exitCode != 0 && err != nil {
			return nil, fmt.Errorf("prompt script failed: %w", err)
		}
		return DecodePromptScriptResponse(response), nil
	}
}

// DecodePromptScriptResponse interprets a prompt script's return value into a
// PromptResponse. Accepted shapes: a JSON object {"description":..., "messages":
// [{"role","content"}]}, a JSON array of messages, or a plain string (single
// user message).
func DecodePromptScriptResponse(response string) *mcplib.PromptResponse {
	trimmed := strings.TrimSpace(response)
	if trimmed == "" {
		return mcplib.NewPromptResponseMessages()
	}
	var raw any
	if err := json.Unmarshal([]byte(trimmed), &raw); err == nil {
		switch v := raw.(type) {
		case map[string]any:
			desc, _ := v["description"].(string)
			msgs := promptMessagesFromAny(v["messages"])
			if msgs == nil {
				msgs = []mcplib.PromptMessage{}
			}
			return &mcplib.PromptResponse{Description: desc, Messages: msgs}
		case []any:
			msgs := promptMessagesFromAny(v)
			if msgs == nil {
				msgs = []mcplib.PromptMessage{}
			}
			return &mcplib.PromptResponse{Messages: msgs}
		}
	}
	return mcplib.NewPromptResponseText(response)
}

// promptMessagesFromAny converts a parsed JSON value (list of {role, content})
// into PromptMessages. Returns nil if the value is not a message list.
func promptMessagesFromAny(v any) []mcplib.PromptMessage {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	var msgs []mcplib.PromptMessage
	for _, elem := range list {
		m, ok := elem.(map[string]any)
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		if role == "" {
			role = "user"
		}
		content := promptContentFromAny(m["content"])
		msgs = append(msgs, mcplib.PromptMessage{Role: mcplib.PromptMessageRole(role), Content: content})
	}
	return msgs
}

// promptContentFromAny converts a message's "content" value into a
// PromptMessageContent. Strings become text blocks; dicts are used as-is.
func promptContentFromAny(v any) mcplib.PromptMessageContent {
	switch c := v.(type) {
	case string:
		return mcplib.PromptMessageContent{Type: "text", Text: c}
	case map[string]any:
		typ, _ := c["type"].(string)
		if typ == "" {
			typ = "text"
		}
		text, _ := c["text"].(string)
		data, _ := c["data"].(string)
		mime, _ := c["mimeType"].(string)
		return mcplib.PromptMessageContent{Type: typ, Text: text, Data: data, MimeType: mime}
	default:
		return mcplib.PromptMessageContent{Type: "text", Text: fmt.Sprintf("%v", v)}
	}
}
