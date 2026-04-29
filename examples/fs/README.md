# fs Library Examples

Examples of using the `fs` library for binary file I/O.

## Examples

- **read_gguf.py** - Parse a GGUF (LLM model) file header
- **read_wav.py** - Parse a WAV audio file header
- **read_png.py** - Parse a PNG image file header

## Functions

### Reading

| Function | Description |
|----------|-------------|
| `fs.read_bytes(path, offset, length)` | Read a byte range from a file |
| `fs.unpack(format, data)` | Unpack binary data using format strings |
| `fs.byte_at(data, index)` | Get unsigned byte value (0-255) at index |

### Writing

| Function | Description |
|----------|-------------|
| `fs.pack(format, values)` | Pack values into a binary string |
| `fs.write_bytes(path, offset, data)` | Write raw bytes at an offset |

### Byte-safe operations

| Function | Description |
|----------|-------------|
| `fs.len(data)` | Byte length (not Unicode code points) |
| `fs.slice(data, start[, end])` | Byte-safe slicing (not rune-based) |

### Format characters

| Format | C Type | Size |
|--------|--------|------|
| `b`/`B` | int8/uint8 | 1 |
| `h`/`H` | int16/uint16 | 2 |
| `i`/`I` | int32/uint32 | 4 |
| `q`/`Q` | int64/uint64 | 8 |
| `f` | float32 | 4 |
| `d` | float64 | 8 |
| `e` | float16 | 2 |

Prefix: `<` little-endian (default), `>` big-endian. Repeat count: `"<4f"` reads 4 float32s.

## Quick Start

```python
import fs

# Read and unpack
raw = fs.read_bytes("model.gguf", 0, 4)
magic = fs.unpack("<I", raw)[0]

# Pack and write
header = fs.pack("<I2Q", [magic, 0, 3])
fs.write_bytes("output.bin", 0, header)

# Byte-safe operations
fs.len(raw)           # byte count
fs.slice(raw, 0, 2)   # first two bytes
fs.byte_at(raw, 0)    # 0-255
```
