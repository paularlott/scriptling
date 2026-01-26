package extlibs

import (
	"context"
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

func RegisterHTMLParserLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(HTMLParserLibrary)
}

// attrRegex is compiled once at package init for parsing HTML tag attributes
var attrRegex = regexp.MustCompile(`(\w+)(?:=(?:"([^"]*)"|'([^']*)'|([^\s>]*)))?`)

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
			instance.Fields["_data"] = &object.String{Value: ""}
			instance.Fields["_pos"] = object.NewInteger(0)
			instance.Fields["_line"] = object.NewInteger(1)
			instance.Fields["_offset"] = object.NewInteger(0)
			instance.Fields["_lasttag"] = &object.String{Value: ""}
			instance.Fields["_rawdata"] = &object.String{Value: ""}

			// Check for convert_charrefs kwarg (default True)
			convertCharrefs := true
			if val, ok := kwargs.Kwargs["convert_charrefs"]; ok {
				if boolVal, ok := val.(*object.Boolean); ok {
					convertCharrefs = boolVal.Value
				}
			}
			instance.Fields["convert_charrefs"] = &object.Boolean{Value: convertCharrefs}

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
			return parseHTML(ctx, instance, dataStr.Value)
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
			instance.Fields["_data"] = &object.String{Value: ""}
			instance.Fields["_pos"] = object.NewInteger(0)
			instance.Fields["_line"] = object.NewInteger(1)
			instance.Fields["_offset"] = object.NewInteger(0)
			instance.Fields["_lasttag"] = &object.String{Value: ""}
			instance.Fields["_rawdata"] = &object.String{Value: ""}

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

			line := instance.Fields["_line"]
			offset := instance.Fields["_offset"]
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

			lasttag := instance.Fields["_lasttag"]
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
				instance := args[0].(*object.Instance)
				tag := args[1]
				attrs := args[2]
				// Call handle_starttag
				if method, ok := instance.Class.Methods["handle_starttag"]; ok {
					if builtin, ok := method.(*object.Builtin); ok {
						builtin.Fn(ctx, object.NewKwargs(nil), instance, tag, attrs)
					} else if fn, ok := method.(*object.Function); ok {
						callMethod(ctx, fn, instance, []object.Object{tag, attrs}, nil)
					}
				}
				// Call handle_endtag
				if method, ok := instance.Class.Methods["handle_endtag"]; ok {
					if builtin, ok := method.(*object.Builtin); ok {
						builtin.Fn(ctx, object.NewKwargs(nil), instance, tag)
					} else if fn, ok := method.(*object.Function); ok {
						callMethod(ctx, fn, instance, []object.Object{tag}, nil)
					}
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
		HelpText: `unknown_decl(data) - Called for unrecognized declarations`,
	},
}

// callMethod is a helper to call a user-defined method on an instance
func callMethod(ctx context.Context, fn *object.Function, instance *object.Instance, args []object.Object, kwargs map[string]object.Object) object.Object {
	// This is a simplified version - the actual implementation would use the evaluator
	// For now, we rely on the fact that derived class methods will be called via the evaluator
	return &object.Null{}
}

// parseHTML parses HTML data and calls the appropriate handler methods
func parseHTML(ctx context.Context, instance *object.Instance, data string) object.Object {
	// Get convert_charrefs setting
	convertCharrefs := true
	if val, ok := instance.Fields["convert_charrefs"]; ok {
		if boolVal, ok := val.(*object.Boolean); ok {
			convertCharrefs = boolVal.Value
		}
	}

	// Simple HTML parser implementation
	i := 0
	for i < len(data) {
		if data[i] == '<' {
			// Check what type of tag this is
			if i+1 < len(data) {
				if data[i+1] == '!' {
					// Comment, DOCTYPE, or CDATA
					if i+4 < len(data) && data[i+2] == '-' && data[i+3] == '-' {
						// Comment
						end := strings.Index(data[i:], "-->")
						if end != -1 {
							comment := data[i+4 : i+end]
							callHandler(ctx, instance, "handle_comment", &object.String{Value: comment})
							i = i + end + 3
							continue
						}
					} else if i+9 < len(data) && strings.ToUpper(data[i+2:i+9]) == "DOCTYPE" {
						// DOCTYPE
						end := strings.Index(data[i:], ">")
						if end != -1 {
							decl := data[i+2 : i+end]
							callHandler(ctx, instance, "handle_decl", &object.String{Value: decl})
							i = i + end + 1
							continue
						}
					} else if i+9 < len(data) && data[i+2:i+9] == "[CDATA[" {
						// CDATA section
						end := strings.Index(data[i:], "]]>")
						if end != -1 {
							cdata := data[i+9 : i+end]
							callHandler(ctx, instance, "handle_data", &object.String{Value: cdata})
							i = i + end + 3
							continue
						}
					}
				} else if data[i+1] == '?' {
					// Processing instruction
					end := strings.Index(data[i:], "?>")
					if end != -1 {
						pi := data[i+2 : i+end]
						callHandler(ctx, instance, "handle_pi", &object.String{Value: pi})
						i = i + end + 2
						continue
					}
				} else if data[i+1] == '/' {
					// End tag
					end := strings.Index(data[i:], ">")
					if end != -1 {
						tag := strings.TrimSpace(strings.ToLower(data[i+2 : i+end]))
						callHandler(ctx, instance, "handle_endtag", &object.String{Value: tag})
						i = i + end + 1
						continue
					}
				} else {
					// Start tag or self-closing tag
					end := findTagEnd(data[i:])
					if end != -1 {
						tagContent := data[i+1 : i+end]
						selfClosing := strings.HasSuffix(strings.TrimSpace(tagContent), "/")
						if selfClosing {
							tagContent = strings.TrimSuffix(strings.TrimSpace(tagContent), "/")
						}

						tag, attrs := parseTag(tagContent)
						tag = strings.ToLower(tag)

						// Store the last start tag text
						instance.Fields["_lasttag"] = &object.String{Value: data[i : i+end+1]}

						// Convert attrs to list of tuples
						attrsList := &object.List{Elements: make([]object.Object, len(attrs))}
						for j, attr := range attrs {
							attrsList.Elements[j] = &object.Tuple{Elements: []object.Object{
								&object.String{Value: attr[0]},
								&object.String{Value: attr[1]},
							}}
						}

						if selfClosing {
							callHandler(ctx, instance, "handle_startendtag", &object.String{Value: tag}, attrsList)
						} else {
							callHandler(ctx, instance, "handle_starttag", &object.String{Value: tag}, attrsList)
						}
						i = i + end + 1
						continue
					}
				}
			}
			i++
		} else {
			// Text data
			end := strings.Index(data[i:], "<")
			var textData string
			if end == -1 {
				textData = data[i:]
				i = len(data)
			} else {
				textData = data[i : i+end]
				i = i + end
			}

			if len(textData) > 0 {
				// Handle character references
				if convertCharrefs {
					textData = html.UnescapeString(textData)
				} else {
					// Call handle_entityref and handle_charref for each reference
					textData = processCharRefs(ctx, instance, textData)
				}
				if len(strings.TrimSpace(textData)) > 0 || len(textData) > 0 {
					callHandler(ctx, instance, "handle_data", &object.String{Value: textData})
				}
			}
		}
	}

	return &object.Null{}
}

// findTagEnd finds the end of a tag, handling quoted attributes
func findTagEnd(data string) int {
	inQuote := false
	quoteChar := byte(0)
	for i := 0; i < len(data); i++ {
		if !inQuote {
			if data[i] == '"' || data[i] == '\'' {
				inQuote = true
				quoteChar = data[i]
			} else if data[i] == '>' {
				return i
			}
		} else {
			if data[i] == quoteChar {
				inQuote = false
			}
		}
	}
	return -1
}

// parseTag parses a tag and returns the tag name and attributes
func parseTag(content string) (string, [][2]string) {
	content = strings.TrimSpace(content)
	parts := strings.Fields(content)
	if len(parts) == 0 {
		return "", nil
	}

	tagName := parts[0]
	attrs := [][2]string{}

	// Parse attributes using pre-compiled regex
	attrPart := content[len(tagName):]
	matches := attrRegex.FindAllStringSubmatch(attrPart, -1)

	for _, match := range matches {
		name := strings.ToLower(match[1])
		value := ""
		if match[2] != "" {
			value = html.UnescapeString(match[2])
		} else if match[3] != "" {
			value = html.UnescapeString(match[3])
		} else if match[4] != "" {
			value = html.UnescapeString(match[4])
		}
		attrs = append(attrs, [2]string{name, value})
	}

	return tagName, attrs
}

// processCharRefs processes character references when convert_charrefs is False
func processCharRefs(ctx context.Context, instance *object.Instance, data string) string {
	result := strings.Builder{}
	i := 0
	for i < len(data) {
		if data[i] == '&' && i+1 < len(data) {
			// Find the end of the reference
			end := strings.Index(data[i:], ";")
			if end != -1 && end < 10 {
				ref := data[i+1 : i+end]
				if ref[0] == '#' {
					// Numeric character reference
					callHandler(ctx, instance, "handle_charref", &object.String{Value: ref[1:]})
					i = i + end + 1
					continue
				} else {
					// Named character reference
					callHandler(ctx, instance, "handle_entityref", &object.String{Value: ref})
					i = i + end + 1
					continue
				}
			}
		}
		result.WriteByte(data[i])
		i++
	}
	return result.String()
}

// callHandler calls a handler method on the instance
func callHandler(ctx context.Context, instance *object.Instance, methodName string, args ...object.Object) object.Object {
	// Look for method in the instance's class methods (which includes inherited and overridden methods)
	if method, ok := instance.Class.Methods[methodName]; ok {
		switch m := method.(type) {
		case *object.Builtin:
			allArgs := append([]object.Object{instance}, args...)
			return m.Fn(ctx, object.NewKwargs(nil), allArgs...)
		case *object.Function:
			// For user-defined functions, we need to call through the evaluator
			// This is handled by the ApplyMethod function if we set it up
			return callUserMethod(ctx, instance, m, args)
		}
	}
	return &object.Null{}
}

// ApplyMethodFunc is a function type that the evaluator can set to allow calling user methods
var ApplyMethodFunc func(ctx context.Context, instance *object.Instance, method *object.Function, args []object.Object) object.Object

// callUserMethod calls a user-defined method through the evaluator
func callUserMethod(ctx context.Context, instance *object.Instance, fn *object.Function, args []object.Object) object.Object {
	if ApplyMethodFunc != nil {
		return ApplyMethodFunc(ctx, instance, fn, args)
	}
	// Fallback: can't call user methods without the evaluator hook
	return &object.Null{}
}

// Entity name to codepoint mapping (subset of common entities)
var name2codepoint = map[string]int{
	"lt":     60,
	"gt":     62,
	"amp":    38,
	"quot":   34,
	"apos":   39,
	"nbsp":   160,
	"copy":   169,
	"reg":    174,
	"trade":  8482,
	"mdash":  8212,
	"ndash":  8211,
	"lsquo":  8216,
	"rsquo":  8217,
	"ldquo":  8220,
	"rdquo":  8221,
	"bull":   8226,
	"hellip": 8230,
	"euro":   8364,
	"pound":  163,
	"yen":    165,
	"cent":   162,
	"deg":    176,
	"plusmn": 177,
	"times":  215,
	"divide": 247,
	"frac12": 189,
	"frac14": 188,
	"frac34": 190,
}

// charRefToString converts a character reference to a string
func charRefToString(ref string) string {
	if strings.HasPrefix(ref, "x") || strings.HasPrefix(ref, "X") {
		// Hexadecimal
		if codepoint, err := strconv.ParseInt(ref[1:], 16, 32); err == nil {
			return string(rune(codepoint))
		}
	} else {
		// Decimal
		if codepoint, err := strconv.ParseInt(ref, 10, 32); err == nil {
			return string(rune(codepoint))
		}
	}
	return ""
}

// entityRefToString converts an entity reference to a string
func entityRefToString(name string) string {
	if codepoint, ok := name2codepoint[name]; ok {
		r := rune(codepoint)
		buf := make([]byte, utf8.RuneLen(r))
		utf8.EncodeRune(buf, r)
		return string(buf)
	}
	return ""
}
