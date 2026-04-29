import fs
import os

# Build test data using only ASCII-safe bytes (0-127) to avoid UTF-8 encoding issues
# We'll construct a known binary pattern: 0x00 0x01 0x02 0x7F 0x0A 0x0D 0x41 0x42
test_path = "/tmp/scriptling_fs_test.bin"
data = chr(0x00) + chr(0x01) + chr(0x02) + chr(0x7F) + chr(0x0A) + chr(0x0D) + "AB"
os.write_file(test_path, data)

# Test fs.read_bytes
raw = fs.read_bytes(test_path, 0, 4)
assert fs.byte_at(raw, 0) == 0x00
assert fs.byte_at(raw, 1) == 0x01
assert fs.byte_at(raw, 2) == 0x02
assert fs.byte_at(raw, 3) == 0x7F

# Test fs.read_bytes with offset
raw2 = fs.read_bytes(test_path, 4, 4)
assert fs.byte_at(raw2, 0) == 0x0A
assert fs.byte_at(raw2, 1) == 0x0D
assert fs.byte_at(raw2, 2) == 0x41  # 'A'
assert fs.byte_at(raw2, 3) == 0x42  # 'B'

# Test fs.unpack with uint16 little-endian
# Bytes 0-1: 0x00 0x01 -> little-endian uint16 = 0x0100 = 256
pair = fs.unpack("<H", fs.read_bytes(test_path, 0, 2))
assert len(pair) == 1
assert pair[0] == 256

# Test fs.unpack with two uint8s
vals = fs.unpack("<BB", fs.read_bytes(test_path, 0, 2))
assert len(vals) == 2
assert vals[0] == 0
assert vals[1] == 1

# Test fs.unpack with int8 (signed)
# Byte 3: 0x7F = 127 as signed int8
signed = fs.unpack("<b", fs.read_bytes(test_path, 3, 1))
assert signed[0] == 127

# Test fs.unpack with repeat count
four = fs.unpack("<4B", fs.read_bytes(test_path, 0, 4))
assert len(four) == 4
assert four[0] == 0x00
assert four[1] == 0x01
assert four[2] == 0x02
assert four[3] == 0x7F

# Test big endian uint16
# Bytes 0-1: 0x00 0x01 -> big-endian uint16 = 0x0001 = 1
be_val = fs.unpack(">H", fs.read_bytes(test_path, 0, 2))
assert be_val[0] == 1

# Test default endian (little)
default_val = fs.unpack("H", fs.read_bytes(test_path, 0, 2))
assert default_val[0] == 256

# Test uint32 little-endian
# Bytes 0-3: 0x00 0x01 0x02 0x7F -> LE uint32 = 0x7F020100
val32 = fs.unpack("<I", fs.read_bytes(test_path, 0, 4))
assert val32[0] == 0x7F020100

# Test float64
float_path = "/tmp/scriptling_fs_float.bin"
float_str = "A" + chr(0x00) + chr(0x00) + chr(0x00) + chr(0x00) + chr(0x00) + chr(0x00)
os.write_file(float_path, float_str)

# Test fs.pack - pack a uint16 and unpack it back
packed = fs.pack("<H", [256])
assert fs.unpack("<H", packed)[0] == 256

# Test fs.pack big endian
packed_be = fs.pack(">H", [1])
assert fs.unpack(">H", packed_be)[0] == 1

# Test fs.pack multiple values roundtrip
packed_multi = fs.pack("<BH", [0x42, 1000])
unpacked = fs.unpack("<BH", packed_multi)
assert unpacked[0] == 0x42
assert unpacked[1] == 1000

# Test fs.write_bytes and read back
write_path = "/tmp/scriptling_fs_write.bin"
fs.write_bytes(write_path, 0, "hello")
readback = fs.read_bytes(write_path, 0, 5)
assert fs.byte_at(readback, 0) == ord('h')
assert fs.byte_at(readback, 4) == ord('o')

# Test fs.write_bytes at offset (patching)
patch_path = "/tmp/scriptling_fs_patch.bin"
fs.write_bytes(patch_path, 0, "AAAA")
fs.write_bytes(patch_path, 2, "BB")
patched = fs.read_bytes(patch_path, 0, 4)
assert fs.byte_at(patched, 0) == ord('A')
assert fs.byte_at(patched, 1) == ord('A')
assert fs.byte_at(patched, 2) == ord('B')
assert fs.byte_at(patched, 3) == ord('B')

# Test fs.len - byte length
binary_data = fs.read_bytes(test_path, 0, 4)
assert fs.len(binary_data) == 4

# Test fs.slice - byte-safe slicing
raw_all = fs.read_bytes(test_path, 0, 6)
sliced = fs.slice(raw_all, 2, 5)
assert fs.len(sliced) == 3
assert fs.byte_at(sliced, 0) == 0x02
assert fs.byte_at(sliced, 1) == 0x7F
assert fs.byte_at(sliced, 2) == 0x0A

# Test fs.slice without end
sliced_no_end = fs.slice(raw_all, 4)
assert fs.len(sliced_no_end) == 2
assert fs.byte_at(sliced_no_end, 0) == 0x0A
assert fs.byte_at(sliced_no_end, 1) == 0x0D

# Clean up
os.remove(test_path)
os.remove(float_path)
os.remove(write_path)
os.remove(patch_path)

True
