# Tests find.entries opt-in fields: include_hash, include_metadata,
# include_symlinks, and follow_links. Exercises the full path through the
# interpreter (kwargs parse -> builtin -> dict construction), complementing
# the Go-level unit tests in extlibs/find_test.go.
import scriptling.find as find
import tempfile
import os
import os.path
import shutil

d = tempfile.mkdtemp()
os.write_file(d + "/same_a.txt", "identical content")     # 17 bytes
os.write_file(d + "/same_b.txt", "identical content")     # 17 bytes
os.write_file(d + "/different.txt", "totally different bytes")
os.makedirs(d + "/subdir")

# ---------------------------------------------------------------------------
# include_hash
# ---------------------------------------------------------------------------
entries = find.entries(d, type="file", include_hash=True)
by_name = {os.path.basename(e["path"]): e for e in entries}

assert "same_a.txt" in by_name and "same_b.txt" in by_name and "different.txt" in by_name, (
    f"missing expected entries: {list(by_name.keys())}")

# Every entry has a 16-char hex hash string.
for name, e in by_name.items():
    assert "hash" in e, f"missing hash key for {name}: {e}"
    h = e["hash"]
    assert isinstance(h, "str"), f"hash not str for {name}: {type(h)}"
    assert len(h) == 16, f"hash length for {name}: got {len(h)}, want 16 ({h!r})"
    # All hex chars
    for c in h:
        assert c in "0123456789abcdef", f"non-hex char {c!r} in hash {h!r}"

# Same content -> same hash; different content -> different hash.
assert by_name["same_a.txt"]["hash"] == by_name["same_b.txt"]["hash"], (
    f"same content should hash equal: {by_name['same_a.txt']['hash']} vs {by_name['same_b.txt']['hash']}")
assert by_name["same_a.txt"]["hash"] != by_name["different.txt"]["hash"], (
    "different content should hash differently")

# Without include_hash: hash key absent.
plain = find.entries(d, type="file", name="same_a.txt")
assert len(plain) == 1 and "hash" not in plain[0], (
    f"hash should be absent without include_hash: {plain[0]}")

# Directories: hash key absent (never populated for non-files).
dir_entries = find.entries(d, type="dir", include_hash=True)
for e in dir_entries:
    assert "hash" not in e, f"directory should not have hash key: {e}"

# ---------------------------------------------------------------------------
# include_metadata -> file_perm
# ---------------------------------------------------------------------------
meta = find.entries(d, type="file", name="same_a.txt", include_metadata=True)
assert len(meta) == 1, f"expected 1 meta entry, got {len(meta)}"
e = meta[0]
assert "file_perm" in e, f"missing file_perm with include_metadata=True: {e}"
assert isinstance(e["file_perm"], "int"), f"file_perm not int: {type(e['file_perm'])}"
assert e["file_perm"] > 0, f"file_perm should be non-zero: {e['file_perm']}"

# Without include_metadata: file_perm absent.
no_meta = find.entries(d, type="file", name="same_a.txt")
assert len(no_meta) == 1 and "file_perm" not in no_meta[0], (
    f"file_perm should be absent without include_metadata: {no_meta[0]}")

# ---------------------------------------------------------------------------
# include_symlinks / follow_links (platform-dependent)
# ---------------------------------------------------------------------------
try:
    os.symlink("same_a.txt", d + "/link.txt")
    symlink_supported = True
except Exception:
    symlink_supported = False

if symlink_supported:
    # include_symlinks=True: the link is yielded with link_target set; it is
    # NOT followed (no content hash, is_dir False regardless of target).
    link_entries = find.entries(d, type="any", include_symlinks=True)
    link_entry = None
    for e in link_entries:
        if os.path.basename(e["path"]) == "link.txt":
            link_entry = e
            break
    assert link_entry is not None, (
        f"symlink not yielded with include_symlinks=True: {link_entries}")
    assert link_entry["link_target"] == "same_a.txt", (
        f"link_target: got {link_entry['link_target']!r}")
    assert link_entry["is_dir"] is False, "symlink is_dir should be False"
    # Symlink itself has no content -> hash absent even with include_hash=True.
    hashed_links = find.entries(d, type="any", include_symlinks=True, include_hash=True)
    for e in hashed_links:
        if os.path.basename(e["path"]) == "link.txt":
            assert "hash" not in e, f"symlink should not be hashed: {e}"

    # Default (follow_links=False, include_symlinks=False): link absent.
    for e in find.entries(d, type="any"):
        assert os.path.basename(e["path"]) != "link.txt", (
            f"symlink should be absent by default: {e}")

    # follow_links=True: link is followed, target's metadata drives the entry,
    # link_target is NOT set (the symlink was resolved, not surfaced).
    followed = find.entries(d, type="any", follow_links=True)
    followed_link = None
    for e in followed:
        if os.path.basename(e["path"]) == "link.txt":
            followed_link = e
            break
    assert followed_link is not None, (
        f"symlink not yielded with follow_links=True: {followed}")
    assert "link_target" not in followed_link, (
        f"followed symlink should have no link_target: {followed_link}")
    # Size matches the target's content (17 bytes for "identical content").
    assert followed_link["size"] == 17, (
        f"followed symlink size: got {followed_link['size']}, want 17")

    # follow_links=True + include_hash=True: hash matches the target's hash.
    fh = find.entries(d, type="any", follow_links=True, include_hash=True)
    fh_link = None
    fh_target = None
    for e in fh:
        base = os.path.basename(e["path"])
        if base == "link.txt":
            fh_link = e
        elif base == "same_a.txt":
            fh_target = e
    assert fh_link is not None and fh_target is not None, "missing followed/hash entries"
    assert fh_link["hash"] == fh_target["hash"], (
        f"followed symlink hash should match target: link={fh_link['hash']} target={fh_target['hash']}")

    # follow_links takes precedence over include_symlinks when both are True.
    both = find.entries(d, type="any", follow_links=True, include_symlinks=True)
    for e in both:
        if os.path.basename(e["path"]) == "link.txt":
            assert "link_target" not in e, (
                f"follow_links should win over include_symlinks: {e}")

shutil.rmtree(d)
True
