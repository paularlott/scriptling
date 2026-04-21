import scriptling.grep as grep
import sys

pattern = "TODO"
search_dir = "."

if len(sys.argv) >= 2:
    pattern = sys.argv[1]
if len(sys.argv) >= 3:
    search_dir = sys.argv[2]

# Regex search — pattern() interprets the string as a regular expression
print(f"Regex search for '{pattern}' in '{search_dir}'...")
matches = grep.pattern(pattern, search_dir, recursive=True, glob="*.py")

if len(matches) == 0:
    print("No matches found.")
else:
    print(f"Found {len(matches)} match(es):\n")
    for m in matches:
        print(f"  {m['file']}:{m['line']}: {m['text']}")

print()

# Literal search — find() treats the string exactly as written, no regex interpretation
# Useful when searching for strings that contain regex special characters like . ( ) * + ?
literal_term = "TODO:"
print(f"Literal search for '{literal_term}'...")
literal_matches = grep.string(literal_term, search_dir, recursive=True, glob="*.py")
print(f"  {len(literal_matches)} match(es)")

# Case-insensitive literal search
ci_matches = grep.string("todo", search_dir, recursive=True, ignore_case=True, glob="*.py")
print(f"  Case-insensitive 'todo': {len(ci_matches)} match(es)")

# Regex with word boundary — only pattern() supports regex syntax
word_matches = grep.pattern(r"\bTODO\b", search_dir, recursive=True)
print(f"  Word-boundary regex \\bTODO\\b: {len(word_matches)} match(es)")
