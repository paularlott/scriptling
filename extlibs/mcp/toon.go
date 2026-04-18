package mcp

import (
	"context"
	"sync"

	mcptoon "github.com/paularlott/mcp/toon"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

const (
	ToonLibraryName = "scriptling.toon"
	ToonLibraryDesc = "TOON (Token-Oriented Object Notation) encoding/decoding library"
)

var (
	toonLibrary     *object.Library
	toonLibraryOnce sync.Once
)

// Register registers the toon library with the given registrar
// First call builds the library, subsequent calls just register it
func RegisterToon(registrar interface{ RegisterLibrary(*object.Library) }) {
	toonLibraryOnce.Do(func() {
		toonLibrary = buildToonLibrary()
	})
	registrar.RegisterLibrary(toonLibrary)
}

// buildToonLibrary builds the TOON library
func buildToonLibrary() *object.Library {
	return object.NewLibrary(ToonLibraryName, map[string]*object.Builtin{
		"encode": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}

				encoded, err := mcptoon.Encode(conversion.ToGo(args[0]))
				if err != nil {
					return &object.Error{Message: err.Error()}
				}

				return &object.String{Value: encoded}
			},
			HelpText: `encode(data) - Encode data to TOON format

Encodes a scriptling value (string, int, float, bool, list, dict) to TOON format.

Parameters:
  data: Any scriptling value to encode

Returns:
  str: TOON formatted string

Example:
  text = toon.encode({"name": "Alice", "age": 30})
  # Returns: TOON formatted string`,
		},
		"decode": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}

				text, err := args[0].AsString()
				if err != nil {
					return err
				}

				decoded, decodeErr := mcptoon.Decode(text)
				if decodeErr != nil {
					return &object.Error{Message: decodeErr.Error()}
				}

				return conversion.FromGo(decoded)
			},
			HelpText: `decode(text) - Decode TOON format to scriptling objects

Decodes a TOON formatted string to scriptling objects (strings, ints, floats, bools, lists, dicts).

Parameters:
  text (str): TOON formatted string

Returns:
  object: Decoded scriptling value

Example:
  data = toon.decode(text)
  # Returns: decoded dict/list/string/etc`,
		},
		"encode_options": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 3); err != nil {
					return err
				}

				indent, err := args[1].AsInt()
				if err != nil {
					return err
				}
				delimiter, err := args[2].AsString()
				if err != nil {
					return err
				}

				encoded, encodeErr := mcptoon.EncodeWithOptions(conversion.ToGo(args[0]), &mcptoon.EncodeOptions{
					Indent:    int(indent),
					Delimiter: delimiter,
				})
				if encodeErr != nil {
					return &object.Error{Message: encodeErr.Error()}
				}

				return &object.String{Value: encoded}
			},
			HelpText: `encode_options(data, indent, delimiter) - Encode data to TOON format with custom options

Encodes a scriptling value to TOON format with custom indentation and delimiter.

Parameters:
  data: Any scriptling value to encode
  indent (int): Number of spaces per indentation level (default: 2)
  delimiter (str): Delimiter for arrays and tabular data (default: ",")

Returns:
  str: TOON formatted string`,
		},
		"decode_options": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 3); err != nil {
					return err
				}

				text, err := args[0].AsString()
				if err != nil {
					return err
				}
				strict, err := args[1].AsBool()
				if err != nil {
					return err
				}
				indentSize, err := args[2].AsInt()
				if err != nil {
					return err
				}

				decoded, decodeErr := mcptoon.DecodeWithOptions(text, &mcptoon.DecodeOptions{
					Strict:     strict,
					IndentSize: int(indentSize),
				})
				if decodeErr != nil {
					return &object.Error{Message: decodeErr.Error()}
				}

				return conversion.FromGo(decoded)
			},
			HelpText: `decode_options(text, strict, indent_size) - Decode TOON format with custom options

Decodes a TOON formatted string with custom parsing options.

Parameters:
  text (str): TOON formatted string
  strict (bool): Enable strict validation (default: true)
  indent_size (int): Expected indentation size (0 = auto-detect, default: 0)

Returns:
  object: Decoded scriptling value`,
		},
	}, nil, ToonLibraryDesc)
}
