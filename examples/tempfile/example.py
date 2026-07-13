import tempfile
import shutil
import os

# --- mkstemp: create a temporary file ---
tmp_file = tempfile.mkstemp(prefix="demo_", suffix=".txt")
print(f"Temp file: {tmp_file}")

# Write to it using standard os functions
import os
with open(tmp_file, "w") as f:
    f.write("temporary content")
print(f"  Content: {open(tmp_file).read()}")

# --- mkdtemp: create a temporary directory ---
tmp_dir = tempfile.mkdtemp(prefix="scratch_")
print(f"\nTemp dir: {tmp_dir}")

# Work inside the temp directory
work_file = os.path.join(tmp_dir, "output.txt")
with open(work_file, "w") as f:
    f.write("result data")
print(f"  Created: {work_file}")

# --- Atomic write pattern ---
# Write to a temp file first, then move into place on success.
# If the write fails, the original file is untouched.
config_path = os.path.join(tmp_dir, "config.toml")
new_config = tempfile.mkstemp(prefix=".config_", dir=tmp_dir)
try:
    with open(new_config, "w") as f:
        f.write('name = "production"\nversion = "2.0"\n')
        # Simulate an error mid-write:
        # raise Exception("disk full!")
    shutil.move(new_config, config_path)
    print(f"\nAtomic write succeeded: {config_path}")
    print(f"  Content: {open(config_path).read().strip()}")
except:
    os.unlink(new_config)
    print("\nAtomic write failed, original unchanged")

# --- gettempdir ---
print(f"\nSystem temp dir: {tempfile.gettempdir()}")
print(f"Temp prefix: {tempfile.gettempprefix()}")

# Cleanup
shutil.rmtree(tmp_dir)
os.unlink(tmp_file)
print("\nDone.")
