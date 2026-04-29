import fs

# Binary I/O Example: Read a GGUF file header
# GGUF is the file format used by llama.cpp for LLM model files

path = "model.gguf"

# Read the magic number (first 4 bytes) to verify it's a GGUF file
raw = fs.read_bytes(path, 0, 4)
magic = fs.unpack("<I", raw)[0]
if magic == 0x46554747:
    print("Valid GGUF file detected!")

    # Read version (uint32 at offset 4)
    version = fs.unpack("<I", fs.read_bytes(path, 4, 4))[0]
    print(f"GGUF version: {version}")

    # Read tensor count (uint64 at offset 8)
    tensor_count = fs.unpack("<Q", fs.read_bytes(path, 8, 8))[0]
    print(f"Tensor count: {tensor_count}")

    # Read metadata KV count (uint64 at offset 16)
    metadata_count = fs.unpack("<Q", fs.read_bytes(path, 16, 8))[0]
    print(f"Metadata KV pairs: {metadata_count}")
else:
    print(f"Not a GGUF file (magic: 0x{magic:08X})")

# Example: Reading individual bytes from raw data
print()
raw = fs.read_bytes(path, 0, 8)
print(f"First 8 bytes:")
for i in range(8):
    print(f"  byte {i}: 0x{fs.byte_at(raw, i):02X}")
