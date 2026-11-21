# Dunder methods of built-in values and types, bound and unbound, as in Python.
import operator

assert ('a'.__len__()) == 1, 1
assert ([1].__contains__(1)) == True, 2
assert (1 .__add__(2)) == 3, 3
assert (1 .__eq__(1)) == True, 4
assert ({'k': 1}.__getitem__('k')) == 1, 5
assert ('a'.__class__.__name__) == 'str', 6
assert ((lambda l: (list.__iadd__(l, [2]), l))([1])) == ([1, 2], [1, 2]), 7
assert (2.7.__trunc__()) == 2, 8
assert (2.5.__round__()) == 2, 9
assert (2.7.__floor__()) == 2, 10
assert (2.1.__ceil__()) == 3, 11
assert (5 .__index__()) == 5, 12
assert ('x'.__str__()) == 'x', 14
assert (str.upper('a')) == 'A', 15
assert ((lambda l: (list.append(l, 1), l))([])) == (None, [1]), 16
assert (dict.get({'a': 1}, 'a')) == 1, 17
assert (list(map(str.strip, [' a ', 'b ']))) == ['a', 'b'], 18
assert ([].__class__.__name__) == 'list', 20
assert (3 .__mul__(4)) == 12, 21
assert ('ab'.__getitem__(1)) == 'b', 22
assert (1 .__lt__(2)) == True, 23
assert ('a'.__add__('b')) == 'ab', 24
assert ([1, 2].__len__()) == 2, 25
assert (1 .__radd__(2)) == 3, 26
assert (5 .__sub__(2)) == 3, 27
assert (5 .__rsub__(2)) == -3, 28
assert (5 .__neg__()) == -5, 29
assert (5 .__hash__() == hash(5)) == True, 30
assert ('a'.__hash__() == hash('a')) == True, 31
assert (5 .__bool__()) == True, 32
assert ([].__bool__() if hasattr([], '__bool__') else 'nobool') == 'nobool', 33
assert (5 .__repr__()) == '5', 34
assert ([1].__repr__()) == '[1]', 35
assert (5 .__str__()) == '5', 36
assert (1.5.__int__()) == 1, 37
assert (3 .__float__()) == 3.0, 38
assert (5 .__abs__()) == 5, 39
assert ((-5).__abs__()) == 5, 40
assert (7 .__floordiv__(2)) == 3, 41
assert (7 .__mod__(3)) == 1, 42
assert (2 .__pow__(3)) == 8, 43
assert (7 .__truediv__(2)) == 3.5, 44
assert (1 .__ne__(2)) == True, 45
assert (1 .__eq__('a')) == NotImplemented, 46
assert (1 .__add__('a')) == NotImplemented, 47
assert (list(iter([1, 2].__iter__()))) == [1, 2], 48
assert ({1, 2}.__contains__(1)) == True, 49
assert ({'a': 1}.__contains__('a')) == True, 50
assert ((lambda d: (d.__setitem__('k', 2), d))({})) == (None, {'k': 2}), 51
assert ((lambda d: (d.__delitem__('k'), d))({'k': 1})) == (None, {}), 52
assert ([1, 2].__getitem__(slice(0, 1))) == [1], 53
assert ((1, 2).__len__()) == 2, 54
assert (str.__len__('abc')) == 3, 55
assert (int.__add__(1, 2)) == 3, 56
assert (str.join(',', ['a', 'b'])) == 'a,b', 57
assert (list(map(str.upper, 'ab'))) == ['A', 'B'], 58
assert (sorted(['b', 'A'], key=str.lower)) == ['A', 'b'], 59
assert (int.__name__) == 'int', 61
assert (str.__name__) == 'str', 62
assert ((lambda x: x.__class__ is int)(1)) == True, 63
assert (type('a').__name__ if not isinstance(type('a'), str) else 'a'.__class__.__name__) == 'str', 64
assert ('abc'.__iter__().__next__()) == 'a', 65
assert (5 .__format__('03d')) == '005', 66
assert ('a'.__format__('>3')) == '  a', 67
assert (1 .__and__(3)) == 1, 68
assert (1 .__or__(2)) == 3, 69
assert (6 .__xor__(3)) == 5, 70
assert (1 .__lshift__(3)) == 8, 71
assert (8 .__rshift__(1)) == 4, 72
assert (5 .__invert__()) == -6, 73
assert (2.5.__add__(1)) == 3.5, 74
assert (1 .__divmod__(2) if hasattr(1, '__divmod__') else 0) == (0, 1), 75
assert ([3, 1].__reversed__().__next__() if hasattr([], '__reversed__') else 0) == 1, 76
assert ('a'.__mul__(3)) == 'aaa', 77
assert ('a'.__rmul__(3)) == 'aaa', 78
assert ([1].__add__([2])) == [1, 2], 79
assert ([1].__mul__(2)) == [1, 1], 80
assert ((1, 2).__contains__(2)) == True, 81
assert ((1, 2).__getitem__(0)) == 1, 82
assert ('a'.__lt__('b')) == True, 83
assert ('a'.__ge__('b')) == False, 84
assert ('a'.__contains__('a')) == True, 85
assert ('%s'.__mod__(1)) == '1', 86
assert (list.__len__([1])) == 1, 87
assert (float.__round__(2.567, 2)) == 2.57, 88
assert (2.567.__round__(2)) == 2.57, 89
assert (7 .__round__(-1)) == 10, 90
assert (int.__index__(5)) == 5, 91
assert (str.__str__('q')) == 'q', 92
assert (list.__iadd__([1], (2,))) == [1, 2], 93
assert ([1].__iadd__([2])) == [1, 2], 94
assert (1 .__class__.__name__) == 'int', 95
assert (getattr(3, '__add__')(4)) == 7, 96
assert (hasattr(3, '__add__')) == True, 97
assert (hasattr('a', '__len__')) == True, 98
assert (callable(getattr([], '__len__'))) == True, 99
assert ((lambda d: (d.__setitem__('k', 1), list(d)))({})) == (None, ['k']), 100
assert (object.__eq__(1, 1)) == True, 101
assert (object.__str__(1)) == '1', 102
assert (list(filter(str.isdigit, ['1', 'a']))) == ['1'], 104
assert (list(map(int.__neg__, [1, 2])) if True else 0) == [-1, -2], 105
assert (list(map(operator.methodcaller('upper'), ['a']))) == ['A'], 106
assert (bytes.decode(b'a')) == 'a', 107
assert (b'a'.__len__()) == 1, 108
assert (str.split('a b')) == ['a', 'b'], 109
assert (str.replace('aa', 'a', 'b')) == 'bb', 110
assert ((lambda s: (set.add(s, 1), s))(set())) == (None, {1}), 112
assert (str.format('{}-{}', 1, 2)) == '1-2', 113
assert (str.startswith('abc', 'a')) == True, 114
assert (tuple.count((1, 1), 1)) == 2, 115
assert (list.index([1, 2], 2)) == 1, 116
assert (dict.fromkeys('ab')) == {'a': None, 'b': None}, 118
assert (str.maketrans('a', 'b')) == {97: 98}, 119

# Unsupported operands give NotImplemented; non-dunders stay AttributeError.
assert (1).__add__("a") is NotImplemented
assert [1].__eq__((1,)) is NotImplemented
assert (1).__eq__(1.0) is NotImplemented and (1.0).__eq__(1) is True
assert hasattr(1, "__add__") and hasattr("a", "__len__")
assert not hasattr("a", "__neg__")
assert object.__repr__(object()).startswith("<")
assert object.__str__(5) == "5"
class P:
    def __init__(self):
        object.__setattr__(self, "x", 3)
assert object.__getattribute__(P(), "x") == 3
try:
    "a".__nope__()
    assert False
except AttributeError:
    pass
try:
    int.__add__("a", 1)
    assert False
except TypeError:
    pass

print("OK")
