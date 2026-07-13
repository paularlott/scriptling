import tarfile
import tempfile
import shutil
import os

# Create sample files to archive
scratch = tempfile.mkdtemp(prefix="tar_demo_")
os.makedirs(os.path.join(scratch, "logs"))
with open(os.path.join(scratch, "logs", "app.log"), "w") as f:
    f.write("2026-01-01 INFO started\n")
    f.write("2026-01-01 INFO stopped\n")
with open(os.path.join(scratch, "version.txt"), "w") as f:
    f.write("2.0.0\n")

# --- Create a gzipped tar archive ---
tar_path = os.path.join(scratch, "release.tar.gz")
print(f"Creating {tar_path} ...")
tf = tarfile.TarFile(tar_path, "w:gz")
tf.add(os.path.join(scratch, "logs"), "logs")
tf.add(os.path.join(scratch, "version.txt"), "version.txt")
tf.addstr("metadata.json", '{"version": "2.0.0", "archived": true}')
tf.close()

# --- Read the archive back ---
print(f"\nContents of {tar_path}:")
tf = tarfile.TarFile(tar_path, "r:gz")
for name in tf.getnames():
    print(f"  {name} ({len(tf.read(name))} bytes)")

# Read inline
meta = tf.read("metadata.json")
print(f"\nmetadata.json: {meta}")

# --- Extract everything ---
extract_dir = os.path.join(scratch, "extracted")
print(f"\nExtracting to {extract_dir} ...")
paths = tf.extractall(extract_dir)
tf.close()
print(f"  Extracted {len(paths)} entries")

# Cleanup
shutil.rmtree(scratch)
print("\nDone.")
