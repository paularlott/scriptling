package mcp

import (
	"sync"

	mcptoon "github.com/paularlott/mcp/toon"
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
	return object.NewLibraryBuilder(ToonLibraryName, ToonLibraryDesc).

		// encode(data) - Encode data to TOON format
		FunctionWithHelp("encode", func(data any) (string, error) {
			return mcptoon.Encode(data)
		}, `encode(data) - Encode data to TOON format

Encodes a scriptling value (string, int, float, bool, list, dict) to TOON format.

Parameters:
  data: Any scriptling value to encode

Returns:
  str: TOON formatted string

Example:
  text = toon.encode({"name": "Alice", "age": 30})
  # Returns: TOON formatted string`).

		// decode(text) - Decode TOON format to scriptling objects
		FunctionWithHelp("decode", func(text string) (any, error) {
			return mcptoon.Decode(text)
		}, `decode(text) - Decode TOON format to scriptling objects

Decodes a TOON formatted string to scriptling objects (strings, ints, floats, bools, lists, dicts).

Parameters:
  text (str): TOON formatted string

Returns:
  object: Decoded scriptling value

Example:
  data = toon.decode(text)
  # Returns: decoded dict/list/string/etc`).

		// encode_options(data, indent, delimiter) - Encode with options
		FunctionWithHelp("encode_options", func(data any, indent int, delimiter string) (string, error) {
			return mcptoon.EncodeWithOptions(data, &mcptoon.EncodeOptions{
				Indent:    indent,
				Delimiter: delimiter,
			})
		}, `encode_options(data, indent, delimiter) - Encode data to TOON format with custom options

Encodes a scriptling value to TOON format with custom indentation and delimiter.

Parameters:
  data: Any scriptling value to encode
  indent (int): Number of spaces per indentation level (default: 2)
  delimiter (str): Delimiter for arrays and tabular data (default: ",")

Returns:
  str: TOON formatted string`).

		// decode_options(text, strict, indent_size) - Decode with options
		FunctionWithHelp("decode_options", func(text string, strict bool, indentSize int) (any, error) {
			return mcptoon.DecodeWithOptions(text, &mcptoon.DecodeOptions{
				Strict:     strict,
				IndentSize: indentSize,
			})
		}, `decode_options(text, strict, indent_size) - Decode TOON format with custom options

Decodes a TOON formatted string with custom parsing options.

Parameters:
  text (str): TOON formatted string
  strict (bool): Enable strict validation (default: true)
  indent_size (int): Expected indentation size (0 = auto-detect, default: 0)

Returns:
  object: Decoded scriptling value`).

		Build()
}
