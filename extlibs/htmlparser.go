package extlibs

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

func RegisterHTMLParserLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(HTMLParserLibrary)
}

// HTMLParserLibrary provides Python-compatible html.parser functionality
var HTMLParserLibrary = object.NewLibrary(HTMLParserLibraryName, nil, map[string]object.Object{
	"HTMLParser": &object.Class{
		Name:    "HTMLParser",
		Methods: htmlParserMethods,
	},
}, "HTML parser library compatible with Python's html.parser module")

var htmlParserMethods = map[string]object.Object{
	"__init__": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 1 {
				return errors.NewError("__init__ requires self argument")
			}
			instance, ok := args[0].(*object.Instance)
			if !ok {
				return errors.NewError("__init__ requires instance as first argument")
			}

			// Initialize parser state
			instance.SetField("_data", object.NewString(""))
			instance.SetField("_pos", object.NewInteger(0))
			instance.SetField("_line", object.NewInteger(1))
			instance.SetField("_offset", object.NewInteger(0))
			instance.SetField("_lasttag", &object.Null{})
			instance.SetField("_rawdata", object.NewString(""))

			// Check for convert_charrefs kwarg (default True)
			convertCharrefs := true
			if val, ok := kwargs.Kwargs["convert_charrefs"]; ok {
				if boolVal, ok := val.(*object.Boolean); ok {
					convertCharrefs = boolVal.BoolValue()
				}
			}
			instance.SetField("convert_charrefs", object.NewBoolean(convertCharrefs))

			return &object.Null{}
		},
		HelpText: `__init__(*, convert_charrefs=True) - Initialize HTMLParser

Parameters:
  convert_charrefs - If True (default), automatically convert character references`,
	},
	"feed": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 2 {
				return errors.NewError("feed() requires self and data arguments")
			}
			instance, ok := args[0].(*object.Instance)
			if !ok {
				return errors.NewError("feed() requires instance as first argument")
			}
			dataStr, ok := args[1].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[1].Type().String())
			}

			// Parse the HTML data
			return parseHTML(ctx, instance, dataStr.StringValue())
		},
		HelpText: `feed(data) - Feed HTML data to the parser

Parses the HTML data and calls handler methods for each element.`,
	},
	"close": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// Process any remaining buffered data
			return &object.Null{}
		},
		HelpText: `close() - Force processing of all buffered data`,
	},
	"reset": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 1 {
				return errors.NewError("reset() requires self argument")
			}
			instance, ok := args[0].(*object.Instance)
			if !ok {
				return errors.NewError("reset() requires instance as first argument")
			}

			// Reset parser state
			instance.SetField("_data", object.NewString(""))
			instance.SetField("_pos", object.NewInteger(0))
			instance.SetField("_line", object.NewInteger(1))
			instance.SetField("_offset", object.NewInteger(0))
			instance.SetField("_lasttag", &object.Null{})
			instance.SetField("_rawdata", object.NewString(""))

			return &object.Null{}
		},
		HelpText: `reset() - Reset the parser instance`,
	},
	"getpos": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 1 {
				return errors.NewError("getpos() requires self argument")
			}
			instance, ok := args[0].(*object.Instance)
			if !ok {
				return errors.NewError("getpos() requires instance as first argument")
			}

			line := instance.Field("_line")
			offset := instance.Field("_offset")
			if line == nil {
				line = object.NewInteger(1)
			}
			if offset == nil {
				offset = object.NewInteger(0)
			}

			return &object.Tuple{Elements: []object.Object{line, offset}}
		},
		HelpText: `getpos() - Return current line number and offset`,
	},
	"get_starttag_text": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 1 {
				return errors.NewError("get_starttag_text() requires self argument")
			}
			instance, ok := args[0].(*object.Instance)
			if !ok {
				return errors.NewError("get_starttag_text() requires instance as first argument")
			}

			lasttag := instance.Field("_lasttag")
			if lasttag == nil {
				return &object.Null{}
			}
			return lasttag
		},
		HelpText: `get_starttag_text() - Return the text of the most recently opened start tag`,
	},
	// Default handler methods - do nothing, meant to be overridden
	"handle_starttag": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return &object.Null{}
		},
		HelpText: `handle_starttag(tag, attrs) - Called when a start tag is encountered

Override this method to handle start tags like <div id="main">.
tag is the tag name (lowercase), attrs is a list of (name, value) tuples.`,
	},
	"handle_endtag": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return &object.Null{}
		},
		HelpText: `handle_endtag(tag) - Called when an end tag is encountered

Override this method to handle end tags like </div>.
tag is the tag name (lowercase).`,
	},
	"handle_startendtag": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// Default: call handle_starttag and handle_endtag
			if len(args) >= 3 {
				instance, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewError("handle_startendtag() requires instance as first argument")
				}
				if r := callHandler(ctx, instance, "handle_starttag", args[1], args[2]); object.IsPropagating(r) {
					return r
				}
				if r := callHandler(ctx, instance, "handle_endtag", args[1]); object.IsPropagating(r) {
					return r
				}
			}
			return &object.Null{}
		},
		HelpText: `handle_startendtag(tag, attrs) - Called for XHTML-style empty tags

Called for tags like <img ... />. Default calls handle_starttag and handle_endtag.`,
	},
	"handle_data": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return &object.Null{}
		},
		HelpText: `handle_data(data) - Called to process text data

Override this method to handle text content between tags.`,
	},
	"handle_entityref": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return &object.Null{}
		},
		HelpText: `handle_entityref(name) - Called for named character references like &gt;

Only called if convert_charrefs is False.`,
	},
	"handle_charref": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return &object.Null{}
		},
		HelpText: `handle_charref(name) - Called for numeric character references like &#62;

Only called if convert_charrefs is False.`,
	},
	"handle_comment": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return &object.Null{}
		},
		HelpText: `handle_comment(data) - Called when a comment is encountered

Override to handle HTML comments like <!-- comment -->.`,
	},
	"handle_decl": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return &object.Null{}
		},
		HelpText: `handle_decl(decl) - Called for DOCTYPE declarations

Override to handle <!DOCTYPE html> and similar.`,
	},
	"handle_pi": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return &object.Null{}
		},
		HelpText: `handle_pi(data) - Called for processing instructions

Override to handle <?xml ...?> and similar.`,
	},
	"unknown_decl": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return &object.Null{}
		},
		HelpText: `unknown_decl(data) - Called for CDATA sections: <![CDATA[x]]> gives "CDATA[x"`,
	},
}

// callHandler calls a handler method on the instance: the script's override
// if it has one, otherwise the library default. A raised exception, error,
// SystemExit or interpreter stop comes back as the result for the caller to
// propagate.
func callHandler(ctx context.Context, instance *object.Instance, methodName string, args ...object.Object) object.Object {
	// Look for method in the instance's class methods (which includes inherited and overridden methods)
	if method, ok := instance.Class.Methods[methodName]; ok {
		switch m := method.(type) {
		case *object.Builtin:
			allArgs := append([]object.Object{instance}, args...)
			return m.Fn(ctx, object.NewKwargs(nil), allArgs...)
		case *object.Function:
			return callUserMethod(ctx, instance, m, args)
		}
	}
	return &object.Null{}
}

// callUserMethod calls a user-defined method through the evaluator
func callUserMethod(ctx context.Context, instance *object.Instance, fn *object.Function, args []object.Object) object.Object {
	eval := evaliface.FromContext(ctx)
	if eval != nil {
		return eval.CallMethod(ctx, instance, fn, args)
	}
	return &object.Null{}
}
