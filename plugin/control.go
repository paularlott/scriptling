package plugin

import (
	"context"

	"github.com/paularlott/scriptling/object"
)

const ControlLibraryName = "scriptling.plugin"

func NewControlLibrary(manager *Manager) *object.Library {
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
					return object.NewString("describe() requires plugin library name")
				}
				name, errObj := args[0].AsString()
				if errObj != nil {
					return errObj
				}
				client, ok := manager.Get(name)
				if !ok {
					return object.NewString("plugin not found: " + name)
				}
				return metadataToDict(client.Metadata())
			},
			HelpText: "describe(name) - Return metadata for a loaded plugin.",
		},
		"call_function": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 2 {
					return object.NewString("call_function() requires library and function name")
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
					return object.NewString("plugin not found: " + library)
				}
				return callPluginFunction(ctx, client, name, kwargs, args[2:]...)
			},
			HelpText: "call_function(library, name, *args, **kwargs) - Call a plugin function.",
		},
		"call_method": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 2 {
					return object.NewString("call_method() requires object and method name")
				}
				methodName, errObj := args[1].AsString()
				if errObj != nil {
					return errObj
				}
				instance, ok := args[0].(*object.Instance)
				if !ok {
					return object.NewString("call_method() requires a plugin object")
				}
				remote, ok := remoteFromInstance(instance)
				if !ok {
					return object.NewString("call_method() requires a plugin object")
				}
				return callPluginMethod(ctx, remote, methodName, kwargs, args[2:]...)
			},
			HelpText: "call_method(obj, name, *args, **kwargs) - Call a method on a plugin object.",
		},
		"_new_object": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 2 {
					return object.NewString("_new_object() requires library and class name")
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
					return object.NewString("plugin not found: " + library)
				}
				return newPluginObject(ctx, client, library, className, kwargs, args[2:]...)
			},
			HelpText: "_new_object(library, class, *args, **kwargs) - Internal plugin wrapper helper.",
		},
		"release": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return object.NewString("release() requires one plugin object")
				}
				if err := Release(args[0]); err != nil {
					return object.NewString(err.Error())
				}
				return &object.Null{}
			},
			HelpText: "release(obj) - Explicitly release a remote plugin object.",
		},
	}
	return object.NewLibrary(ControlLibraryName, functions, nil, "Plugin control library")
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
