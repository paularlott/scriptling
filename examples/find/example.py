import scriptling.find as find
import tempfile
import os
import time

# Build a small project tree
root = tempfile.mkdtemp(prefix="find_demo_")
for path in ["src/main.py", "src/utils.py", "src/.hidden.py", "tests/test_main.py",
             "docs/readme.md", ".git/config", "node_modules/lib/index.js"]:
    full = os.path.join(root, path)
    os.makedirs(os.path.dirname(full), exist_ok=True)
    with open(full, "w") as f:
        f.write("# " + path + "\n")

# Make one file "old"
old_time = time.time() - 7 * 86400  # 7 days ago
os.chdir(root)
old_file = os.path.join(root, "src", "utils.py")
with open(old_file, "w") as f:
    f.write("# old file\n")

# --- Find all Python files (recursive by default) ---
py_files = find.path(root, name="*.py", type="file")
print(f"Python files ({len(py_files)}):")
for p in py_files:
    print(f"  {os.path.relpath(p, root)}")

# --- Find with include_hidden (descends into .git, .hidden.py) ---
all_py = find.path(root, name="*.py", type="file", include_hidden=True)
print(f"\nIncluding hidden ({len(all_py)}):")
for p in all_py:
    print(f"  {os.path.relpath(p, root)}")

# --- Find directories only ---
dirs = find.path(root, type="dir")
print(f"\nDirectories ({len(dirs)}):")
for p in dirs:
    print(f"  {os.path.relpath(p, root)}/")

# --- Find files modified in the last 24 hours ---
recent = find.path(root, type="file", mtime_min=time.time() - 86400)
print(f"\nRecently modified ({len(recent)}):")
for p in recent:
    print(f"  {os.path.relpath(p, root)}")

# --- Non-recursive (immediate children only) ---
top = find.path(root, recursive=False)
print(f"\nImmediate children ({len(top)}):")
for p in top:
    print(f"  {os.path.relpath(p, root)}")

# Cleanup
import shutil
shutil.rmtree(root)
print("\nDone.")
