import difflib

# --- ratio ---
assert difflib.ratio("hello", "hello") == 1.0, f"identical ratio: {difflib.ratio('hello', 'hello')}"
assert difflib.ratio("", "") == 1.0, "empty ratio"
assert difflib.ratio("abc", "") == 0.0, f"empty b ratio: {difflib.ratio('abc', '')}"
r = difflib.ratio("hello world", "hello there")
assert 0.0 < r < 1.0, f"partial ratio: {r}"

# --- opcodes ---
ops = difflib.opcodes("line1\nline2\nline3\n", "line1\nLINE2\nline3\n")
tags = [op[0] for op in ops]
assert "replace" in tags, f"expected replace in {tags}"
assert "equal" in tags, f"expected equal in {tags}"

# identical — all equal
ops2 = difflib.opcodes("abc\n", "abc\n")
assert all(op[0] == "equal" for op in ops2), f"identical should be all equal: {ops2}"

# empty a — all insert
ops3 = difflib.opcodes("", "new\n")
assert len(ops3) == 1 and ops3[0][0] == "insert", f"empty a: {ops3}"

# empty b — all delete
ops4 = difflib.opcodes("old\n", "")
assert len(ops4) == 1 and ops4[0][0] == "delete", f"empty b: {ops4}"

# --- unified_diff ---
a = "line1\nline2\nline3\n"
b = "line1\nLINE2\nline3\n"
diff = difflib.unified_diff(a, b, fromfile="a.txt", tofile="b.txt")
assert diff.startswith("--- a.txt\n"), f"header: {diff[:30]}"
assert "+++ b.txt\n" in diff, "tofile header missing"
assert "-line2\n" in diff, f"delete missing: {diff}"
assert "+LINE2\n" in diff, f"insert missing: {diff}"

# no diff on identical
assert difflib.unified_diff("same\n", "same\n") == "", "identical should produce empty diff"

# --- get_close_matches ---
matches = difflib.get_close_matches("appel", ["ape", "apple", "peach", "puppy"])
assert "apple" in matches, f"apple not in {matches}"

# n parameter
matches2 = difflib.get_close_matches("appel", ["ape", "apple", "peach", "puppy"], 1)
assert len(matches2) == 1, f"n=1 should return 1: {matches2}"

# cutoff parameter — high cutoff returns fewer
matches3 = difflib.get_close_matches("appel", ["ape", "apple", "peach", "puppy"], cutoff=0.9)
# apple is very close, ape less so
assert len(matches3) <= len(matches), "higher cutoff should return fewer or equal matches"

# no matches
matches4 = difflib.get_close_matches("xyz", ["ape", "apple", "peach"])
assert isinstance(matches4, list), "should return list"

True
