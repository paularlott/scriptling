package plugin

import (
	"context"

	"github.com/paularlott/scriptling/object"
)

const ControlLibraryName = "scriptling.plugin"

func NewControlLibrary(manager *Manager, registrar Registrar, scriptRegistrar ScriptLibraryRegistrar, unregistrar LibraryUnregistrar) *object.Library {
	functions := map[string]*object.Builtin{
		"list": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				plugins := manager.List()
				items := make([]object.Object, 0, len(plugins))
				for _, meta := range plugins {
					items = append(items, metadataToDict(meta))
				}
				return &object.List{Elements: items}
			},
			HelpText: "list() - Return loaded plugin metadata.",
		},
		"describe": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return pluginErr("describe() requires plugin library name")
				}
				name, errObj := args[0].AsString()
				if errObj != nil {
					return errObj
				}
				client, ok := manager.Get(name)
				if !ok {
					return pluginErr("plugin not found: " + name)
				}
				return metadataToDict(client.Metadata())
			},
			HelpText: "describe(name) - Return metadata for a loaded plugin.",
		},
		"call_function": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 2 {
					return pluginErr("call_function() requires library and function name")
				}
				library, errObj := args[0].AsString()
				if errObj != nil {
					return errObj
				}
				name, errObj := args[1].AsString()
				if errObj != nil {
					return errObj
				}
				client, ok := manager.Get(library)
				if !ok {
					return pluginErr("plugin not found: " + library)
				}
				return callPluginFunction(ctx, client, name, kwargs, args[2:]...)
			},
			HelpText: "call_function(library, name, *args, **kwargs) - Call a plugin function.",
		},
		"batch_call": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(kwargs.Kwargs) != 0 {
					return pluginErr("batch_call() does not accept keyword arguments")
				}
				if len(args) != 2 {
					return pluginErr("batch_call() requires library and calls")
				}
				library, errObj := args[0].AsString()
				if errObj != nil {
					return errObj
				}
				rawCalls, errObj := args[1].AsList()
				if errObj != nil {
					return pluginErr("batch_call() calls must be a list")
				}
				client, ok := manager.Get(library)
				if !ok {
					return pluginErr("plugin not found: " + library)
				}
				calls, errObj := parseBatchCallSpecs(rawCalls)
				if errObj != nil {
					return errObj
				}
				return batchCallPluginFunctions(ctx, client, calls)
			},
			HelpText: `batch_call(library, calls) - Call multiple functions on one plugin process in a JSON-RPC batch.

calls must be a list of dictionaries:
  {"name": "method", "args": [1, 2], "kwargs": {"flag": True}}

For scriptling=False clients, each name is sent directly as the raw JSON-RPC
method. For scriptling=True clients, each item is sent as a function.call
request. Results are returned in the same order as calls. Callback arguments
are not supported in batch_call.`,
		},
		"call_method": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 2 {
					return pluginErr("call_method() requires object and method name")
				}
				methodName, errObj := args[1].AsString()
				if errObj != nil {
					return errObj
				}
				instance, ok := args[0].(*object.Instance)
				if !ok {
					return pluginErr("call_method() requires a plugin object")
				}
				remote, ok := remoteFromInstance(instance)
				if !ok {
					return pluginErr("call_method() requires a plugin object")
				}
				return callPluginMethod(ctx, remote, methodName, kwargs, args[2:]...)
			},
			HelpText: "call_method(obj, name, *args, **kwargs) - Call a method on a plugin object.",
		},
		"_new_object": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 2 {
					return pluginErr("_new_object() requires library and class name")
				}
				library, errObj := args[0].AsString()
				if errObj != nil {
					return errObj
				}
				className, errObj := args[1].AsString()
				if errObj != nil {
					return errObj
				}
				client, ok := manager.Get(library)
				if !ok {
					return pluginErr("plugin not found: " + library)
				}
				return newPluginObject(ctx, client, library, className, kwargs, args[2:]...)
			},
			HelpText: "_new_object(library, class, *args, **kwargs) - Internal plugin wrapper helper.",
		},
		"release": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return pluginErr("release() requires one plugin object")
				}
				if err := ReleaseWithContext(ctx, args[0]); err != nil {
					return pluginErr(err.Error())
				}
				return &object.Null{}
			},
			HelpText: "release(obj) - Explicitly release a remote plugin object.",
		},
		"load": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 2 {
					return pluginErr("load() requires name and path")
				}
				name, errObj := args[0].AsString()
				if errObj != nil {
					return errObj
				}
				path, errObj := args[1].AsString()
				if errObj != nil {
					return errObj
				}
				scriptling, kwErr := kwargs.GetBool("scriptling", false)
				if kwErr != nil {
					return kwErr
				}
				var execArgs []string
				if kwargs.Has("args") {
					rawArgs := kwargs.Get("args")
					list, ok := rawArgs.(*object.List)
					if !ok {
						return pluginErr("load() args must be a list of strings")
					}
					for _, elem := range list.Elements {
						s, errObj := elem.AsString()
						if errObj != nil {
							return errObj
						}
						execArgs = append(execArgs, s)
					}
				}
				insecureSkipTLS, kwErr := kwargs.GetBool("insecure_skip_tls", false)
				if kwErr != nil {
					return kwErr
				}
				headers, errObj := parseHeaderKwarg(kwargs)
				if errObj != nil {
					return errObj
				}
				var client *Client
				var err error
				if isHTTPURL(path) {
					client, err = manager.LoadURL(ctx, name, path, scriptling, insecureSkipTLS, headers)
				} else {
					if len(headers) > 0 {
						return pluginErr("load() headers are only supported for HTTP(S) endpoints")
					}
					client, err = manager.LoadPath(ctx, name, path, scriptling, execArgs)
				}
				if err != nil {
					return pluginErr(err.Error())
				}
				if client.HandshakeDone() {
					registerClientLibrary(registrar, scriptRegistrar, client)
				}
				return object.NewString(client.Metadata().Name)
			},
			HelpText: `load(name, path, scriptling=False, args=None, insecure_skip_tls=False, headers=None) - Register a JSON-RPC peer.

path may be a filesystem executable path, or an http:// / https:// JSON-RPC
endpoint. Executable peers use newline-delimited JSON-RPC over stdio; HTTP
peers send one JSON-RPC object or batch per POST.

When scriptling=False, call_function sends the requested function name directly
as the JSON-RPC method. When scriptling=True, the executable must implement the
Scriptling plugin handshake and function.call dispatch method. The loaded
client is reachable via call_function, describe, and list. Handshaken
scriptling=True peers also register an importable plugin.* proxy library, which
unload() removes.

Parameters:
  name (str): Library name to register the executable under. Normalised into
    the plugin.* namespace (e.g. "widgets" becomes "plugin.widgets"). Must
    not collide with an existing plugin library name.
  path (str): Filesystem path to the executable, or http(s) JSON-RPC endpoint.
  scriptling (bool, optional): If True, perform the plugin protocol handshake
    so describe()/list() report version and schema from the executable. If
    False (default), the handshake is skipped.
  args (list[str], optional): Command-line arguments passed to the executable
    (e.g. ["--json-rpc", "./setup.py"]). Ignored for HTTP endpoints.
  insecure_skip_tls (bool, optional): Skip HTTPS certificate verification for
    HTTP endpoints. Intended for local/self-signed development servers.
  headers (dict[str, str], optional): Additional HTTP headers sent with every
    HTTP(S) JSON-RPC request, including handshake, calls, and batches.

Identity is by absolute path for executables and by URL for HTTP endpoints. A
second load() of the same path or URL with the same name is a no-op (returns
the existing client, ignoring scriptling/args/insecure_skip_tls/headers).
Loading an already-loaded peer under a different name, or loading a new peer
under a name already in use, raises an error.

Returns the normalised library name (e.g. "plugin.widgets"); the short form
("widgets") may be used with call_function, describe, and unload.`,
		},
		"unload": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return pluginErr("unload() requires a library name")
				}
				name, errObj := args[0].AsString()
				if errObj != nil {
					return errObj
				}
				if err := manager.Unload(name); err != nil {
					return pluginErr(err.Error())
				}
				unregisterClientLibrary(unregistrar, NormalizeLibraryName(name))
				return &object.Null{}
			},
			HelpText: "unload(name) - Close a loaded executable and remove it from the registry.",
		},
	}
	return object.NewLibrary(ControlLibraryName, functions, nil, "Plugin control library")
}

func parseHeaderKwarg(kwargs object.Kwargs) (map[string]string, object.Object) {
	if !kwargs.Has("headers") {
		return nil, nil
	}
	rawHeaders := kwargs.Get("headers")
	if _, ok := rawHeaders.(*object.Null); ok {
		return nil, nil
	}
	dict, ok := rawHeaders.(*object.Dict)
	if !ok {
		return nil, pluginErr("load() headers must be a dict of strings")
	}
	headers := make(map[string]string, len(dict.Pairs))
	for _, pair := range dict.Pairs {
		value, errObj := pair.Value.AsString()
		if errObj != nil {
			return nil, pluginErr("load() headers must be a dict of strings")
		}
		headers[pair.StringKey()] = value
	}
	return headers, nil
}

func parseBatchCallSpecs(items []object.Object) ([]batchCallSpec, object.Object) {
	calls := make([]batchCallSpec, len(items))
	for i, item := range items {
		dict, ok := item.(*object.Dict)
		if !ok {
			return nil, pluginErrf("batch_call() call %d must be a dict", i)
		}
		namePair, ok := dict.GetByString("name")
		if !ok {
			return nil, pluginErrf("batch_call() call %d missing name", i)
		}
		name, errObj := namePair.Value.AsString()
		if errObj != nil {
			return nil, pluginErrf("batch_call() call %d name must be a string", i)
		}

		var callArgs []object.Object
		if argsPair, ok := dict.GetByString("args"); ok {
			callArgs, errObj = argsPair.Value.AsList()
			if errObj != nil {
				return nil, pluginErrf("batch_call() call %d args must be a list or tuple", i)
			}
		}

		callKwargs := map[string]object.Object{}
		if kwargsPair, ok := dict.GetByString("kwargs"); ok {
			kwargsDict, ok := kwargsPair.Value.(*object.Dict)
			if !ok {
				return nil, pluginErrf("batch_call() call %d kwargs must be a dict", i)
			}
			for _, pair := range kwargsDict.Pairs {
				key, ok := pair.Key.(*object.String)
				if !ok {
					return nil, pluginErrf("batch_call() call %d kwargs keys must be strings", i)
				}
				callKwargs[key.StringValue()] = pair.Value
			}
		}

		calls[i] = batchCallSpec{
			Name:   name,
			Args:   callArgs,
			Kwargs: object.NewKwargs(callKwargs),
		}
	}
	return calls, nil
}

func metadataToDict(meta Metadata) *object.Dict {
	functions := make([]object.Object, 0, len(meta.Schema.Functions))
	for _, fn := range meta.Schema.Functions {
		functions = append(functions, object.NewString(fn.Name))
	}
	classes := make([]object.Object, 0, len(meta.Schema.Classes))
	for _, class := range meta.Schema.Classes {
		classes = append(classes, object.NewString(class.Name))
	}
	constants := make([]object.Object, 0, len(meta.Schema.Constants))
	for _, constant := range meta.Schema.Constants {
		constants = append(constants, object.NewString(constant.Name))
	}
	return object.NewStringDict(map[string]object.Object{
		"name":        object.NewString(meta.Name),
		"version":     object.NewString(meta.Version),
		"description": object.NewString(meta.Description),
		"transport":   object.NewString(meta.Transport),
		"functions":   &object.List{Elements: functions},
		"classes":     &object.List{Elements: classes},
		"constants":   &object.List{Elements: constants},
	})
}
