import fs

# Binary I/O Example: Read WAV file header
# WAV files have a well-known binary structure

path = "audio.wav"

# Read the RIFF header
raw = fs.read_bytes(path, 0, 44)

# RIFF magic (bytes 0-3): should be "RIFF"
magic = fs.unpack("<4B", fs.read_bytes(path, 0, 4))
riff = chr(magic[0]) + chr(magic[1]) + chr(magic[2]) + chr(magic[3])
print(f"Format: {riff}")

# File size - 8 (uint32 at offset 4)
file_size = fs.unpack("<I", fs.read_bytes(path, 4, 4))[0]
print(f"Data size: {file_size + 8} bytes")

# WAVE marker (bytes 8-11)
wave = fs.unpack("<4B", fs.read_bytes(path, 8, 4))
wave_str = chr(wave[0]) + chr(wave[1]) + chr(wave[2]) + chr(wave[3])
print(f"Wave marker: {wave_str}")

# fmt chunk - audio format info
audio_format = fs.unpack("<H", fs.read_bytes(path, 20, 2))[0]
num_channels = fs.unpack("<H", fs.read_bytes(path, 22, 2))[0]
sample_rate = fs.unpack("<I", fs.read_bytes(path, 24, 4))[0]
bits_per_sample = fs.unpack("<H", fs.read_bytes(path, 34, 2))[0]

print(f"Audio format: {audio_format} ({'PCM' if audio_format == 1 else 'other'})")
print(f"Channels: {num_channels}")
print(f"Sample rate: {sample_rate} Hz")
print(f"Bits per sample: {bits_per_sample}")
print(f"Duration: {(file_size / (sample_rate * num_channels * bits_per_sample / 8)):.2f} seconds")
