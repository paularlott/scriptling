# Regression coverage for the DictItems 2-variable for-loop fast path
# (evalForStatementWithContext), which binds k,v directly from each pair instead
# of allocating a throwaway Tuple per iteration. Every case here must match the
# generic iteration path's semantics.

failures = 0

# (1) Basic: all pairs seen, k and v bound correctly.
d = {"a": 1, "b": 2, "c": 3}
seen = []
for k, v in d.items():
    seen.append((k, v))
seen.sort()
if seen != [("a", 1), ("b", 2), ("c", 3)]:
    failures += 1
    print("FAIL basic:", seen)

# (2) break stops the loop and suppresses else.
out = []
ran_else = False
for k, v in d.items():
    out.append(k)
    break
else:
    ran_else = True
if len(out) != 1 or ran_else:
    failures += 1
    print("FAIL break/else: out=", out, "else=", ran_else)

# (3) continue skips the rest of a body iteration.
collected = []
for k, v in d.items():
    if k == "b":
        continue
    collected.append(k)
collected.sort()
if collected != ["a", "c"]:
    failures += 1
    print("FAIL continue:", collected)

# (4) for/else: else runs on normal completion.
ran_else = False
for k, v in d.items():
    pass
else:
    ran_else = True
if not ran_else:
    failures += 1
    print("FAIL else-on-completion")

# (5) Value mutation is visible live (keys snapshotted, values read fresh).
d2 = {"x": 10}
seen_val = None
for k, v in d2.items():
    if k == "x":
        d2["x"] = 999
    seen_val = v  # first iteration sees the original value
# After the loop the live value is updated:
if seen_val != 10 or d2["x"] != 999:
    failures += 1
    print("FAIL value-mutation: seen=", seen_val, "final=", d2["x"])

# (6) Deletion during iteration: not-yet-visited keys are skipped (view semantics),
#     no crash. At least the visited + the deleted key's slot are handled.
d3 = {"p": 1, "q": 2, "r": 3, "s": 4}
remaining = []
try:
    for k, v in d3.items():
        remaining.append(k)
        if k == "p":
            del d3["q"]  # delete a possibly-unvisited key mid-loop
except Exception as e:
    failures += 1
    print("FAIL delete-during-iter raised:", e)
# 'p' always visited first; the loop must complete without error.
if "p" not in remaining:
    failures += 1
    print("FAIL delete-during-iter: didn't visit p:", remaining)

# (7) Nested tuple-key unpacking via the fast path (setForVariable handles it).
d4 = {("a", 1): "x", ("b", 2): "y"}
got = []
for (a, b), v in d4.items():
    got.append((a, b, v))
got.sort()
if got != [("a", 1, "x"), ("b", 2, "y")]:
    failures += 1
    print("FAIL nested-unpack:", got)

# (8) Fallback: single variable still receives a Tuple (generic path).
d5 = {"k": 7}
kv_seen = None
for kv in d5.items():
    kv_seen = kv
if kv_seen[0] != "k" or kv_seen[1] != 7:
    failures += 1
    print("FAIL 1-var fallback:", kv_seen)

# (9) Fallback: 3 variables into 2-element items is an error (generic path).
errored = False
try:
    for a, b, c in d5.items():
        pass
except Exception:
    errored = True
if not errored:
    failures += 1
    print("FAIL 3-var fallback should error")

# (10) Empty dict: body never runs, else runs on completion.
d_empty = {}
visited = []
ran_else = False
for k, v in d_empty.items():
    visited.append(k)
else:
    ran_else = True
if visited != [] or not ran_else:
    failures += 1
    print("FAIL empty-dict: visited=", visited, "else=", ran_else)

# (11) return inside the loop propagates out of the enclosing function.
def find_key(target):
    for k, v in d.items():
        if k == target:
            return v
    return -1
if find_key("b") != 2 or find_key("missing") != -1:
    failures += 1
    print("FAIL return-in-loop:", find_key("b"), find_key("missing"))

# (12) Exception handling inside the loop body works (try/except within body).
#     (A raise that propagates OUT of a bare loop is a separate, pre-existing
#     limitation affecting every for-loop path — not specific to this fast path.)
caught = None
d_exc = {"a": 1, "b": 2}
for k, v in d_exc.items():
    try:
        if k == "b":
            raise ValueError("inner")
    except Exception as e:
        caught = str(e)
        continue
if caught != "inner":
    failures += 1
    print("FAIL in-body exception handling:", caught)

# (13) Semantic equivalence: the 2-var fast path must yield exactly the same
#      (k, v) pairs as the generic 1-var path that unpacks the Tuple manually.
d6 = {"m": 100, "n": 200, "o": 300, "p": 400, "q": 500}
fast_pairs = []
for k, v in d6.items():
    fast_pairs.append((k, v))
generic_pairs = []
for kv in d6.items():
    generic_pairs.append((kv[0], kv[1]))
fast_pairs.sort()
generic_pairs.sort()
if fast_pairs != generic_pairs:
    failures += 1
    print("FAIL equivalence: fast=", fast_pairs, "generic=", generic_pairs)

# (14) Stress deletion: delete every not-yet-seen key on each iteration; the
#      loop must still terminate without error (view skip semantics).
d7 = {chr(c): c for c in range(ord("a"), ord("a") + 6)}
seen_count = 0
keys_at_start = list(d7.keys())
try:
    for k, v in d7.items():
        seen_count += 1
        # delete a different key each step (one we hopefully haven't visited)
        for other in keys_at_start:
            if other != k and other in d7:
                del d7[other]
                break
except Exception as e:
    failures += 1
    print("FAIL stress-delete raised:", e)

assert failures == 0
