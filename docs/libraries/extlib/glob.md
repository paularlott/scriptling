# Glob Library

The glob library provides Unix shell-style wildcards for filename matching, similar to Python's glob module. It allows you to find files and directories matching specific patterns.

## Basic Usage

```python
import glob

# Find all Python files in the current directory
files = glob.glob("*.py")
print(files)  # ["script.py", "app.py", ...]

# Find all text files recursively
all_txt = glob.glob("**/*.txt")
```

## Functions

### `glob(pattern[, root_dir="."])`

Find all pathnames matching a pattern. Returns a list of filenames.

**Pattern Syntax:**
- `*` - Matches everything
- `?` - Matches any single character
- `[seq]` - Matches any character in seq
- `[!seq]` - Matches any character not in seq
- `**` - Matches all files and directories recursively (when supported)

```python
import glob

# Match all files ending in .txt
txt_files = glob.glob("*.txt")

# Match files starting with "test"
test_files = glob.glob("test*")

# Match single character variations
files = glob.glob("file?.txt")  # file1.txt, filea.txt, etc.

# Match character ranges
logs = glob.glob("log[0-9].txt")  # log0.txt, log1.txt, etc.

# Match from specific directory
configs = glob.glob("*.json", "/etc/myapp")
```

### `iglob(pattern[, root_dir="."])`

Returns an iterator over the filenames matching the pattern. This is more memory efficient for large result sets.

```python
import glob

# Process files one at a time
for filename in glob.iglob("**/*.py"):
    print(f"Found: {filename}")
```

### `escape(pattern)`

Escape all special characters in a pattern so they are treated as literal characters rather than wildcards.

```python
import glob

# Escape special characters to search for literal filenames
pattern = glob.escape("file*.txt")
# Returns "file[*].txt" which matches literal "file*.txt"
```

## Pattern Examples

### Common Patterns

```python
import glob

# All files in current directory
all_files = glob.glob("*")

# All directories
dirs = glob.glob("*/")

# All hidden files (files starting with dot)
hidden = glob.glob(".*")

# Recursively find all files under a directory
all_files_recursive = glob.glob("**/*")

# Find all Python files in subdirectories
python_files = glob.glob("*/*.py")

# Multiple extensions with bracket notation
scripts = glob.glob("*.*[py][sh]")  # Matches .py, .sh, etc.
```

### Recursive Patterns with `**`

The `**` pattern matches zero or more directories:

```python
import glob

# Find all markdown files recursively
docs = glob.glob("**/*.md")

# Find all files under src/
src_files = glob.glob("src/**/*")

# Find specific file pattern recursively
configs = glob.glob("**/config.json")
```

## Examples

### Finding and Processing Files

```python
import glob

# Find all log files and analyze them
logs = glob.glob("logs/*.log")
for log_file in logs:
    print(f"Processing {log_file}")
    # ... process file ...
```

### Working with Different Directories

```python
import glob

# Search in specific directory
configs = glob.glob("*.conf", "/etc/myapp")

# Search from current directory (default)
current = glob.glob("*.txt")
```

### Using Iterators for Large Result Sets

```python
import glob

# More memory efficient for large directories
for filename in glob.iglob("**/*"):
    # Process each file individually
    print(filename)
```

### Combining with Other Libraries

```python
import glob
import pathlib

# Use glob results with pathlib
for file_path in glob.glob("*.txt"):
    p = pathlib.Path(file_path)
    print(f"{file_path}: {p.stat().st_size} bytes")
```

## Security

The glob library respects filesystem security restrictions. If the Scriptling interpreter is configured with allowed paths, all glob operations are restricted to those directories. Attempting to access files outside the allowed directories will result in security errors.

```python
import glob

# This will fail if /etc is outside allowed directories
files = glob.glob("/etc/passwd*")
```

## Notes

- The `**` pattern for recursive matching is supported
- Pattern matching is case-sensitive on Unix systems and may be case-insensitive on Windows depending on the filesystem
- Results are returned in arbitrary order (not sorted)
- Empty list is returned if no matches are found

## Error Handling

```python
import glob

# Security errors will be raised for disallowed paths
try:
    files = glob.glob("/etc/*")
except Exception as e:
    print(f"Access denied: {e}")
```
