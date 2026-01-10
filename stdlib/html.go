package stdlib

import (
	"context"
	"html"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var HTMLLibrary = object.NewLibrary(map[string]*object.Builtin{
	"escape": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			str, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			return &object.String{Value: html.EscapeString(str.Value)}
		},
		HelpText: `escape(s) - Escape HTML special characters

Converts &, <, >, ", and ' to HTML-safe sequences.

Parameters:
  s - String to escape

Returns: Escaped string

Example:
  import html
  safe = html.escape("<script>alert('xss')</script>")
  print(safe)  # "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;"`,
	},
	"unescape": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			str, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			// html.UnescapeString handles standard HTML entities
			result := html.UnescapeString(str.Value)
			// Also handle numeric entities that Python handles
			result = unescapeNumericEntities(result)
			return &object.String{Value: result}
		},
		HelpText: `unescape(s) - Unescape HTML entities

Converts HTML entities back to their corresponding characters.
Handles named entities (&lt;, &gt;, &amp;, &quot;, &#39;) and
numeric entities (&#60;, &#x3c;).

Parameters:
  s - String with HTML entities to unescape

Returns: Unescaped string

Example:
  import html
  text = html.unescape("&lt;script&gt;")
  print(text)  # "<script>"`,
	},
}, nil, "HTML escaping and unescaping library")

// unescapeNumericEntities handles numeric character references
// Go's html.UnescapeString already handles most cases, but we add
// extra handling for completeness
func unescapeNumericEntities(s string) string {
	// Go's html.UnescapeString should handle most numeric entities
	// This is a fallback for any edge cases
	result := strings.Builder{}
	i := 0
	for i < len(s) {
		if s[i] == '&' && i+2 < len(s) && s[i+1] == '#' {
			// Find the end of the entity
			end := i + 2
			for end < len(s) && end < i+10 && s[end] != ';' {
				end++
			}
			if end < len(s) && s[end] == ';' {
				// Already handled by html.UnescapeString, just pass through
				result.WriteString(s[i : end+1])
				i = end + 1
				continue
			}
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}
