package mcp

import (
	"context"
	"fmt"
	"strings"

	mcplib "github.com/paularlott/mcp"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// Completion (completion/complete) for prompt arguments and resource template
// variables. A script completer is a function called as
//
//	completer(argument, value, context)
//
// with the name of the argument (or template variable) being completed, the
// partial value typed so far, and a dict of the arguments the client has
// already filled in. It returns a list of suggestion strings, best match
// first; the server sends at most 100. An exception, or a return value that
// isn't a list of strings, becomes a JSON-RPC error for that request.
//
// Completers are named per argument by a "complete" mapping on the
// registration: {"language": complete_language}. In a decorated file the
// value is a function defined in that file (or its name); everywhere else —
// and for a dotted name in a decorated file — it is a "module.function"
// reference resolved through the library dirs, like a handler reference.

// BuildCompleterSource returns a completer that evaluates src in a fresh
// interpreter on every call and calls funcName from it — the completer of a
// decorated prompt or resource template, defined in the same file.
func BuildCompleterSource(src []byte, funcName string, cfg HandlerConfig) mcplib.Completer {
	return func(ctx context.Context, req *mcplib.CompletionRequest) ([]string, error) {
		p := prepareScriptling(cfg, nil)
		defer p.Close()
		if _, err := p.EvalWithContext(ctx, string(src)); err != nil {
			return nil, fmt.Errorf("failed to load completer source: %w", err)
		}
		result, callErr := p.CallFunctionWithContext(ctx, funcName, req.Argument(), req.Value(), contextArgs(req))
		return completionValues(funcName, result, callErr)
	}
}

// BuildCompleterRef returns a completer for a "module.function" reference,
// imported per call through the configured library dirs.
func BuildCompleterRef(ref string, cfg HandlerConfig) mcplib.Completer {
	return func(ctx context.Context, req *mcplib.CompletionRequest) ([]string, error) {
		libName, _, ok := SplitHandlerRef(ref)
		if !ok {
			return nil, fmt.Errorf("invalid completer reference %q", ref)
		}
		p := prepareScriptling(cfg, nil)
		defer p.Close()
		if err := p.ImportWithContext(ctx, libName); err != nil {
			return nil, fmt.Errorf("failed to import library %s: %w", libName, err)
		}
		result, callErr := p.CallFunctionWithContext(ctx, ref, req.Argument(), req.Value(), contextArgs(req))
		return completionValues(ref, result, callErr)
	}
}

// BuildCompleter picks BuildCompleterRef for a dotted reference and
// BuildCompleterSource (a function in src) for a plain name. With no source
// (TOML metadata, register_request_*), every reference must be dotted.
func BuildCompleter(src []byte, ref string, cfg HandlerConfig) (mcplib.Completer, error) {
	if strings.Contains(ref, ".") {
		if _, _, ok := SplitHandlerRef(ref); !ok {
			return nil, fmt.Errorf("invalid completer reference %q: use \"module.function\"", ref)
		}
		return BuildCompleterRef(ref, cfg), nil
	}
	if src == nil {
		return nil, fmt.Errorf("invalid completer reference %q: use \"module.function\"", ref)
	}
	if ref == "" {
		return nil, fmt.Errorf("empty completer reference")
	}
	return BuildCompleterSource(src, ref, cfg), nil
}

func contextArgs(req *mcplib.CompletionRequest) map[string]any {
	out := make(map[string]any, len(req.ContextArgs()))
	for k, v := range req.ContextArgs() {
		out[k] = v
	}
	return out
}

// completionValues maps a completer's return value to suggestions.
func completionValues(name string, result object.Object, callErr error) ([]string, error) {
	if callErr != nil {
		return nil, fmt.Errorf("completer %s failed: %w", name, callErr)
	}
	var elems []object.Object
	switch v := result.(type) {
	case *object.List:
		elems = v.Elements
	case *object.Tuple:
		elems = v.Elements
	case *object.Null:
		return nil, nil
	default:
		return nil, fmt.Errorf("completer %s must return a list of strings, got %s", name, result.Type())
	}
	values := make([]string, 0, len(elems))
	for i, el := range elems {
		s, ok := el.(*object.String)
		if !ok {
			return nil, fmt.Errorf("completer %s: element %d must be a string, got %s", name, i, el.Type())
		}
		values = append(values, s.StringValue())
	}
	return values, nil
}

// dictGetCompleters reads the optional "complete" mapping of a registration
// dict: argument (or template variable) name → completer reference. A value
// may be a string, or — in a decorated file — the completer function itself,
// recorded by name. allowed lists the names that may be completed; any other
// key is an error.
func dictGetCompleters(d *object.Dict, allowed []string) (map[string]string, error) {
	pair, ok := d.GetByString("complete")
	if !ok {
		return nil, nil
	}
	return completersFromObject(pair.Value, allowed)
}

func completersFromObject(v object.Object, allowed []string) (map[string]string, error) {
	switch v.(type) {
	case *object.Null:
		return nil, nil
	case *object.Dict:
	default:
		return nil, fmt.Errorf("\"complete\" must be a dict of argument name to completer, got %s", v.Type())
	}
	dict := v.(*object.Dict)
	out := make(map[string]string, len(dict.Pairs))
	for _, pair := range dict.Pairs {
		key, err := pair.Key.AsString()
		if err != nil || key == "" {
			return nil, fmt.Errorf("\"complete\" keys must be non-empty strings")
		}
		if !containsName(allowed, key) {
			return nil, fmt.Errorf("\"complete\" names %q, which is not one of its arguments (%s)", key, strings.Join(allowed, ", "))
		}
		switch c := pair.Value.(type) {
		case *object.String:
			if c.StringValue() == "" {
				return nil, fmt.Errorf("\"complete\"[%q] is empty", key)
			}
			out[key] = c.StringValue()
		case *object.Function:
			out[key] = c.Name
		default:
			return nil, fmt.Errorf("\"complete\"[%q] must be a function or a \"module.function\" string, got %s", key, pair.Value.Type())
		}
	}
	return out, nil
}

// checkCompleterNames verifies, at scan time, that every plain (undotted)
// completer of a decorated registration is a function in the scanned file.
func checkCompleterNames(completers map[string]string, p *scriptling.Scriptling) error {
	for arg, ref := range completers {
		if strings.Contains(ref, ".") {
			continue
		}
		obj, err := p.GetVarAsObject(ref)
		if err != nil {
			return fmt.Errorf("completer %q for %q is not defined in this file", ref, arg)
		}
		if _, ok := obj.(*object.Function); !ok {
			return fmt.Errorf("completer %q for %q is not a function (got %s)", ref, arg, obj.Type())
		}
	}
	return nil
}

func containsName(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
