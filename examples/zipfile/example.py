import zipfile
import tempfile
import shutil
import os

# Create a few sample files to archive
scratch = tempfile.mkdtemp(prefix="zip_demo_")
os.makedirs(os.path.join(scratch, "src"))
with open(os.path.join(scratch, "src", "main.py"), "w") as f:
    f.write("print('hello')\n")
with open(os.path.join(scratch, "src", "config.toml"), "w") as f:
    f.write('name = "demo"\n')

# --- Create a ZIP archive ---
zip_path = os.path.join(scratch, "bundle.zip")
print(f"Creating {zip_path} ...")
zf = zipfile.ZipFile(zip_path, "w")
zf.write(os.path.join(scratch, "src", "main.py"), "app/main.py")
zf.write(os.path.join(scratch, "src", "config.toml"), "app/config.toml")
zf.writestr("app/README.md", "# Demo Bundle\n\nCreated with scriptling zipfile.\n")
zf.close()

print(f"  is_zipfile: {zipfile.is_zipfile(zip_path)}")

# --- Read it back ---
print("\nContents:")
zf = zipfile.ZipFile(zip_path)
for name in zf.namelist():
    print(f"  {name} ({len(zf.read(name))} bytes)")

# --- Extract one file ---
content = zf.read("app/README.md")
print(f"\napp/README.md:\n{content}")
zf.close()

# --- Extract all ---
extract_dir = os.path.join(scratch, "extracted")
print(f"Extracting to {extract_dir} ...")
zf = zipfile.ZipFile(zip_path)
paths = zf.extractall(extract_dir)
zf.close()
print(f"  Extracted {len(paths)} files")

# Cleanup
shutil.rmtree(scratch)
print("\nDone.")
