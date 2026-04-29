import fs

# Binary I/O Example: Read PNG file header
# PNG files start with a well-known 8-byte signature

path = "image.png"

# Read the 8-byte PNG signature
sig = fs.read_bytes(path, 0, 8)

# Check signature bytes
expected = [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A]
valid = True
for i in range(8):
    b = fs.byte_at(sig, i)
    if b != expected[i]:
        valid = False
        break

if valid:
    print("Valid PNG file!")

    # Read IHDR chunk (starts at byte 8)
    # Chunk length (4 bytes) + type (4 bytes) + data (13 bytes) + CRC (4 bytes)
    chunk_len = fs.unpack(">I", fs.read_bytes(path, 8, 4))[0]
    chunk_type = fs.unpack(">4B", fs.read_bytes(path, 12, 4))

    type_str = chr(chunk_type[0]) + chr(chunk_type[1]) + chr(chunk_type[2]) + chr(chunk_type[3])
    print(f"First chunk: {type_str} ({chunk_len} bytes)")

    if type_str == "IHDR":
        # IHDR data: width(4), height(4), bit_depth(1), color_type(1), compression(1), filter(1), interlace(1)
        ihdr = fs.unpack(">II", fs.read_bytes(path, 16, 8))
        width = ihdr[0]
        height = ihdr[1]
        bit_depth = fs.byte_at(fs.read_bytes(path, 24, 1), 0)
        color_type = fs.byte_at(fs.read_bytes(path, 25, 1), 0)

        print(f"Dimensions: {width}x{height}")
        print(f"Bit depth: {bit_depth}")
        color_names = {0: "Grayscale", 2: "RGB", 3: "Indexed", 4: "Gray+Alpha", 6: "RGBA"}
        print(f"Color type: {color_names.get(color_type, 'Unknown')}")
else:
    print("Not a valid PNG file")
