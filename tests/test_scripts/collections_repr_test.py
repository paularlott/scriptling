# Container reprs and str() match Python 3.12+.
import collections
import copy
from collections import OrderedDict, Counter, defaultdict, deque

od = OrderedDict([("b", 2), ("a", 1)])
assert repr(od) == "OrderedDict({'b': 2, 'a': 1})"
assert str(od) == repr(od)
assert repr(OrderedDict()) == "OrderedDict()"
assert repr([OrderedDict(a=1)]) == "[OrderedDict({'a': 1})]"
assert repr(od.copy()) == "OrderedDict({'b': 2, 'a': 1})"
assert repr(copy.copy(od)) == repr(od)
assert repr(copy.deepcopy(od)) == repr(od)
assert repr(dict(od)) == "{'b': 2, 'a': 1}"
assert type(od).__name__ == "OrderedDict"
assert isinstance(od, dict) and isinstance(od, OrderedDict)
assert not isinstance({}, OrderedDict)
assert issubclass(OrderedDict, dict)
assert od == {"a": 1, "b": 2}
od.move_to_end("b")
assert repr(od) == "OrderedDict({'a': 1, 'b': 2})"

assert repr(Counter("abacb")) == "Counter({'a': 2, 'b': 2, 'c': 1})"
assert str(Counter({"x": 3, "y": 5})) == "Counter({'y': 5, 'x': 3})"
assert repr(Counter()) == "Counter()"
assert repr([Counter("aab")]) == "[Counter({'a': 2, 'b': 1})]"

assert repr(defaultdict(list, {"a": [1]})) == "defaultdict(<class 'list'>, {'a': [1]})"
assert str(defaultdict(int)) == "defaultdict(<class 'int'>, {})"
assert repr(defaultdict(None)) == "defaultdict(None, {})"

assert repr(deque([1, 2], maxlen=2)) == "deque([1, 2], maxlen=2)"
assert repr(deque([1, 2, 3], maxlen=2)) == "deque([2, 3], maxlen=2)"
assert str(deque()) == "deque([])"
print("OK")
