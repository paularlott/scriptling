import scriptling.text as text
import scriptling.grep as grep
import os
import os.path

demo_dir = os.path.dirname(os.path.abspath(__file__))

# ── Literal replacement ────────────────────────────────────────────────────────
# text.replace(old, new, path, ...) treats the search string exactly as written.
# Special characters like . ( ) * are not interpreted as regex.

print("=== Literal replacement ===")

tmp = os.path.join(demo_dir, "_demo.txt")
os.write_file(tmp, "hello world\nhello again\nfoo.bar() called\n")

n = text.replace("hello", "hi", tmp)
print(f"replace 'hello' -> 'hi': {n} file(s) modified")

# "foo.bar()" contains regex special chars — replace() handles them literally
n = text.replace("foo.bar()", "baz.qux()", tmp)
print(f"replace 'foo.bar()' -> 'baz.qux()': {n} file(s) modified")

print(f"Result:\n{os.read_file(tmp)}")
os.remove(tmp)

# ── Regex replacement with capture groups ─────────────────────────────────────
# text.replace_pattern(regex, new, path, ...) uses Go regular expressions.
# Capture groups are referenced as ${1}, ${2}, or ${name} in the replacement.

print("=== Regex replacement with capture groups ===")

tmp2 = os.path.join(demo_dir, "_demo2.py")
os.write_file(tmp2, "def get_user(id):\n    pass\ndef get_order(id):\n    pass\n")

n = text.replace_pattern(r"def get_(\w+)\(", "def fetch_${1}(", tmp2)
print(f"rename get_* -> fetch_*: {n} file(s) modified")
print(f"Result:\n{os.read_file(tmp2)}")
os.remove(tmp2)

# ── Directory + grep workflow ──────────────────────────────────────────────────
# Use grep to preview what will change, then replace across a directory.

print("=== Directory + grep workflow ===")

matches = grep.string("import scriptling.text", demo_dir, recursive=True, glob="*.py")
files = {m["file"] for m in matches}
print(f"Files referencing scriptling.text: {len(files)}")

# ── Case-insensitive replacement ───────────────────────────────────────────────

print("=== Case-insensitive replacement ===")

tmp3 = os.path.join(demo_dir, "_demo3.txt")
os.write_file(tmp3, "TODO: fix this\ntodo: also fix this\nToDo: and this\n")

n = text.replace("todo:", "DONE:", tmp3, ignore_case=True)
print(f"case-insensitive replace: {n} file(s) modified")
print(f"Result:\n{os.read_file(tmp3)}")
os.remove(tmp3)

# ── Extract capture groups ────────────────────────────────────────────────────
# text.extract(regex, path, ...) returns capture groups from every match.
# Returns: [{"file", "line", "text", "groups"}, ...]

print("=== Extract capture groups ===")

tmp4 = os.path.join(demo_dir, "_demo4.py")
os.write_file(tmp4, "def get_user(id):\n    pass\ndef get_order(id):\n    pass\ndef set_value(x):\n    pass\n")

# Extract all function names
matches = text.extract(r"def (\w+)\(", tmp4)
print(f"Functions found: {len(matches)}")
for m in matches:
    print(f"  line {m['line']}: {m['groups'][0]}")

os.remove(tmp4)

tmp5 = os.path.join(demo_dir, "_demo5.txt")
os.write_file(tmp5, "host=localhost\nport=8080\nuser=admin\n")

# Extract key=value pairs — two capture groups per match
matches = text.extract(r"(\w+)=(\S+)", tmp5)
print(f"\nConfig entries:")
for m in matches:
    key, value = m["groups"]
    print(f"  {key} = {value}")

os.remove(tmp5)
