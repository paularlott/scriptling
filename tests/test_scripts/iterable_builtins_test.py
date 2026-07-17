# Test that sorted/sum/min/max/str.join accept any iterable
# (dict views, sets, strings, dicts), not just lists and tuples.

d = {"a": 3, "c": 1, "b": 2}

# sorted on every iterable form
assert sorted(d.keys()) == ["a", "b", "c"]
assert sorted(d.values()) == [1, 2, 3]
assert sorted(d.items(), key=lambda x: x[1]) == [("c", 1), ("b", 2), ("a", 3)]
assert sorted(set([3, 1, 2])) == [1, 2, 3]
assert sorted("cab") == ["a", "b", "c"]
assert sorted(d) == ["a", "b", "c"]   # dict yields its keys

# sum on dict_values, set, tuple
assert sum(d.values()) == 6
assert sum(set([1, 2, 3])) == 6
assert sum((10, 20, 30)) == 60

# min / max on dict views, sets, strings
assert min(d.keys()) == "a"
assert max(d.keys()) == "c"
assert min(d.values()) == 1
assert max(d.values()) == 3
assert min(set([5, 2, 8])) == 2
assert max(set([5, 2, 8])) == 8
assert min("cab") == "a"
assert max("cab") == "c"

# multi-argument min/max still works (regression guard)
assert min(5, 2, 8) == 2
assert max(5, 2, 8) == 8

# str.join accepts any iterable of strings
# (dict/set iteration order isn't guaranteed, so normalise before comparing)
assert "-".join(["a", "b", "c"]) == "a-b-c"
assert "+".join(("x", "y", "z")) == "x+y+z"
jk = ",".join(d.keys()).split(",")
jk.sort()
assert jk == ["a", "b", "c"]
js = "".join(set(["x", "y"]))
assert js == "xy" or js == "yx"

# sorted must not mutate its input
orig = [3, 1, 2]
assert sorted(orig) == [1, 2, 3]
assert orig == [3, 1, 2]

print("Iterable builtins tests passed!")
