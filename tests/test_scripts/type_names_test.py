#!/usr/bin/env scriptling
# type() reports Python's type names for library types too.

import itertools
import collections

assert type(itertools.count()).__name__ == "count", type(itertools.count())
assert type(itertools.cycle([1])).__name__ == "cycle", type(itertools.cycle([1]))
assert type(itertools.repeat(1)).__name__ == "repeat", type(itertools.repeat(1))

d = collections.defaultdict(int)
assert type(d).__name__ == "defaultdict", type(d)
assert isinstance(d, dict) and isinstance(d, "dict")

assert type(collections).__name__ == "module", type(collections)
assert type(itertools).__name__ == "module", type(itertools)

assert type(1) == int and type(1) == "int" and type(1) != "INTEGER"
print("✓ type names")
