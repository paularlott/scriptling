import shutil
import tempfile
import os

# Create a source tree to work with
root = tempfile.mkdtemp(prefix="shutil_demo_")
os.makedirs(os.path.join(root, "project", "src"))
os.makedirs(os.path.join(root, "project", "tests"))
with open(os.path.join(root, "project", "src", "app.py"), "w") as f:
    f.write("print('hello')\n")
with open(os.path.join(root, "project", "tests", "test_app.py"), "w") as f:
    f.write("assert True\n")
with open(os.path.join(root, "standalone.txt"), "w") as f:
    f.write("single file\n")

# --- copy: copy a single file ---
dst = os.path.join(root, "standalone_copy.txt")
shutil.copy(os.path.join(root, "standalone.txt"), dst)
print(f"copy: {dst}")

# --- copytree: recursively copy a directory ---
backup = os.path.join(root, "project-backup")
shutil.copytree(os.path.join(root, "project"), backup)
print(f"copytree: {backup}")

# Verify the copy
for name in ["src/app.py", "tests/test_app.py"]:
    p = os.path.join(backup, name)
    print(f"  {name}: exists={os.path.exists(p)}")

# --- move: rename/move a file or directory ---
moved = os.path.join(root, "project-moved")
shutil.move(backup, moved)
print(f"\nmove: {backup} -> {moved}")

# --- rmtree: recursively delete a directory tree ---
shutil.rmtree(os.path.join(root, "project"))
print(f"rmtree: removed project/")
print(f"  project still exists: {os.path.exists(os.path.join(root, 'project'))}")

# --- disk_usage: check free space ---
du = shutil.disk_usage(root)
print(f"\ndisk_usage('/'):")
print(f"  total: {du['total'] / (1024*1024*1024):.1f} GiB")
print(f"  used:  {du['used'] / (1024*1024*1024):.1f} GiB")
print(f"  free:  {du['free'] / (1024*1024*1024):.1f} GiB")

# Cleanup
shutil.rmtree(root)
print("\nDone.")
