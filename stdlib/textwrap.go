package stdlib

import (
	"context"
	"strings"
	"unicode"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var TextwrapLibrary = object.NewLibrary(TextwrapLibraryName, map[string]*object.Builtin{
	"wrap": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			width, _ := kwargs.GetInt("width", 70)
			widthInt := int(width)

			if len(args) < 1 {
				return errors.NewError("wrap() requires at least 1 argument")
			}

			text, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			// If second positional arg provided, use it as width
			if len(args) >= 2 {
				if intVal, ok := args[1].(*object.Integer); ok {
					widthInt = int(intVal.Value)
				}
			}

			lines := wrapText(text.Value, widthInt)
			elements := make([]object.Object, len(lines))
			for i, line := range lines {
				elements[i] = &object.String{Value: line}
			}
			return &object.List{Elements: elements}
		},
		HelpText: `wrap(text, width=70) - Wrap a single paragraph of text

Parameters:
  text  - Text to wrap
  width - Maximum line width (default 70)

Returns: List of lines

Example:
  import textwrap
  lines = textwrap.wrap("Hello world, this is a long line", 10)`,
	},

	"fill": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			width, _ := kwargs.GetInt("width", 70)
			widthInt := int(width)

			if len(args) < 1 {
				return errors.NewError("fill() requires at least 1 argument")
			}

			text, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			if len(args) >= 2 {
				if intVal, ok := args[1].(*object.Integer); ok {
					widthInt = int(intVal.Value)
				}
			}

			lines := wrapText(text.Value, widthInt)
			return &object.String{Value: strings.Join(lines, "\n")}
		},
		HelpText: `fill(text, width=70) - Wrap text and return a single string

Parameters:
  text  - Text to wrap
  width - Maximum line width (default 70)

Returns: Wrapped text as single string with newlines

Example:
  import textwrap
  result = textwrap.fill("Hello world, this is a long line", 10)`,
	},

	"dedent": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewError("dedent() requires exactly 1 argument")
			}

			text, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			return &object.String{Value: dedentText(text.Value)}
		},
		HelpText: `dedent(text) - Remove common leading whitespace from all lines

Parameters:
  text - Text to dedent

Returns: Dedented text

Example:
  import textwrap
  text = """
      Hello
      World
  """
  result = textwrap.dedent(text)  # Lines start at column 0`,
	},

	"indent": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 2 {
				return errors.NewError("indent() requires at least 2 arguments")
			}

			text, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			prefix, ok := args[1].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[1].Type().String())
			}

			lines := strings.Split(text.Value, "\n")
			result := make([]string, len(lines))
			for i, line := range lines {
				// Only indent non-empty lines by default
				if len(strings.TrimSpace(line)) > 0 {
					result[i] = prefix.Value + line
				} else {
					result[i] = line
				}
			}
			return &object.String{Value: strings.Join(result, "\n")}
		},
		HelpText: `indent(text, prefix) - Add prefix to non-empty lines

Parameters:
  text   - Text to indent
  prefix - String to add to beginning of each line

Returns: Indented text

Example:
  import textwrap
  result = textwrap.indent("Hello\nWorld", "  ")  # "  Hello\n  World"`,
	},

	"shorten": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			placeholder, _ := kwargs.GetString("placeholder", "[...]")

			if len(args) < 2 {
				return errors.NewError("shorten() requires at least 2 arguments")
			}

			text, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			widthObj, ok := args[1].(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", args[1].Type().String())
			}
			width := int(widthObj.Value)

			// Collapse whitespace and truncate
			collapsed := collapseWhitespace(text.Value)
			if len(collapsed) <= width {
				return &object.String{Value: collapsed}
			}

			// Truncate to fit with placeholder
			if width <= len(placeholder) {
				return &object.String{Value: placeholder[:width]}
			}

			// Find word boundary
			maxLen := width - len(placeholder)
			truncated := collapsed[:maxLen]

			// Try to break at word boundary
			lastSpace := strings.LastIndex(truncated, " ")
			if lastSpace > 0 {
				truncated = truncated[:lastSpace]
			}

			return &object.String{Value: strings.TrimSpace(truncated) + placeholder}
		},
		HelpText: `shorten(text, width, placeholder="[...]") - Truncate text to fit width

Parameters:
  text        - Text to shorten
  width       - Maximum width including placeholder
  placeholder - String to indicate truncation (default "[...]")

Returns: Shortened text

Example:
  import textwrap
  result = textwrap.shorten("Hello World!", 10)  # "Hello[...]"`,
	},
}, nil, "Text wrapping and filling utilities")

// wrapText wraps text to the specified width
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	// Collapse and normalize whitespace
	text = collapseWhitespace(text)
	words := strings.Fields(text)

	if len(words) == 0 {
		return []string{}
	}

	var lines []string
	var currentLine strings.Builder

	for _, word := range words {
		if currentLine.Len() == 0 {
			currentLine.WriteString(word)
		} else if currentLine.Len()+1+len(word) <= width {
			currentLine.WriteString(" ")
			currentLine.WriteString(word)
		} else {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		}
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}

// dedentText removes common leading whitespace from all lines
func dedentText(text string) string {
	lines := strings.Split(text, "\n")

	// Find minimum indent (ignoring empty lines)
	minIndent := -1
	for _, line := range lines {
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}

		indent := 0
		for _, ch := range line {
			if unicode.IsSpace(ch) {
				indent++
			} else {
				break
			}
		}

		if minIndent < 0 || indent < minIndent {
			minIndent = indent
		}
	}

	if minIndent <= 0 {
		return text
	}

	// Remove common indent
	result := make([]string, len(lines))
	for i, line := range lines {
		if len(strings.TrimSpace(line)) == 0 {
			result[i] = line
		} else if len(line) >= minIndent {
			result[i] = line[minIndent:]
		} else {
			result[i] = line
		}
	}

	return strings.Join(result, "\n")
}

// collapseWhitespace collapses runs of whitespace to single spaces
func collapseWhitespace(text string) string {
	var result strings.Builder
	inWhitespace := false

	for _, ch := range text {
		if unicode.IsSpace(ch) {
			if !inWhitespace {
				result.WriteRune(' ')
				inWhitespace = true
			}
		} else {
			result.WriteRune(ch)
			inWhitespace = false
		}
	}

	return strings.TrimSpace(result.String())
}
