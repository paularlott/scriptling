# Round-11 scatter: dunder protocol on user classes (__add__/__sub__/__eq__/
# __lt__/__len__/__bool__/__getitem__/__setitem__/__contains__/__next__/
# __call__/staticmethod/classmethod/super/inheritance/__neg__/__invert__/
# __abs__/slice-in-__getitem__), unicode strings, json deep, statement
# corners (del/global/nonlocal/with/nested comprehensions), os.path.
# CPython-generated. Excluded: json spacing and *args-as-list (documented).
try:
    class V:
        def __init__(self, n): self.n = n
        def __add__(self, o): return V(self.n + o.n)
        def __repr__(self): return f'V({self.n})'
    assert repr(repr(V(2) + V(3))) == "'V(5)'"
except Exception as ex:
    raise ex
try:
    class V:
        def __init__(self, n): self.n = n
        def __sub__(self, o): return self.n - o.n
    assert repr(V(5) - V(2)) == '3'
except Exception as ex:
    raise ex
try:
    class V:
        def __init__(self, n): self.n = n
        def __mul__(self, o): return self.n * o
    assert repr(V(4) * 2) == '8'
except Exception as ex:
    raise ex
try:
    class V:
        def __init__(self, n): self.n = n
        def __eq__(self, o): return self.n == o.n
    assert repr(V(1) == V(1) and not (V(1) == V(2))) == 'True'
except Exception as ex:
    raise ex
try:
    class V:
        def __init__(self, n): self.n = n
        def __lt__(self, o): return self.n < o.n
    assert repr(V(1) < V(2) and not (V(2) < V(1))) == 'True'
except Exception as ex:
    raise ex
try:
    class V:
        def __init__(self, n): self.n = n
        def __len__(self): return self.n
    assert repr(len(V(7))) == '7'
except Exception as ex:
    raise ex
try:
    class F:
        def __len__(self): return 0
    assert repr(bool(F())) == 'False'
except Exception as ex:
    raise ex
try:
    class T:
        def __len__(self): return 3
    assert repr(bool(T())) == 'True'
except Exception as ex:
    raise ex
try:
    class Box:
        def __getitem__(self, k): return k * 10
    assert repr(Box()[4]) == '40'
except Exception as ex:
    raise ex
try:
    class Grid:
        def __init__(self): self.d = {}
        def __setitem__(self, k, v): self.d[k] = v
    g = Grid()
    g['a'] = 1
    assert repr(g.d) == "{'a': 1}"
except Exception as ex:
    raise ex
try:
    class Bag:
        def __contains__(self, x): return x in (1, 2)
    assert repr([3 in Bag(), 2 in Bag()]) == '[False, True]'
except Exception as ex:
    raise ex
try:
    class Counter2:
        def __init__(self): self.i = 0
        def __iter__(self): return self
        def __next__(self):
            self.i += 1
            if self.i > 3: raise StopIteration
            return self.i
    assert repr(list(Counter2())) == '[1, 2, 3]'
except Exception as ex:
    raise ex
try:
    class Addable:
        def __call__(self, a, b): return a + b
    assert repr(Addable()(2, 3)) == '5'
except Exception as ex:
    raise ex
try:
    class P:
        def __init__(self): self._x = 1
        def get_x(self): return self._x
    assert repr(P().get_x()) == '1'
except Exception as ex:
    raise ex
try:
    class A:
        def who(self): return 'A'
    class B(A):
        def who(self): return 'B+' + super().who()
    assert repr(B().who()) == "'B+A'"
except Exception as ex:
    raise ex
try:
    class A:
        x = 'base'
    class C(A):
        pass
    assert repr(C.x) == "'base'"
except Exception as ex:
    raise ex
try:
    class S:
        @staticmethod
        def sm(a): return a * 2
    assert repr(S.sm(21)) == '42'
except Exception as ex:
    raise ex
try:
    class K:
        count = 0
        @classmethod
        def bump(cls): cls.count += 1
    assert repr(None if K.bump() else K.count) == '1'
except Exception as ex:
    raise ex
try:
    class R:
        def __str__(self): return 'str-form'
    assert repr(str(R()) + '/' + f'{R()}') == "'str-form/str-form'"
except Exception as ex:
    raise ex
try:
    class Neg:
        def __neg__(self): return 42
    assert repr(-Neg()) == '42'
except Exception as ex:
    raise ex
try:
    class Abs:
        def __abs__(self): return 7
    assert repr(abs(Abs())) == '7'
except Exception as ex:
    raise ex
try:
    class Idx:
        def __getitem__(self, k):
            if isinstance(k, slice): return 'slice'
            return 'index'
    assert repr([Idx()[0], Idx()[1:2]]) == "['index', 'slice']"
except Exception as ex:
    raise ex
assert repr('héllo'[1]) == "'é'"
assert repr(len('héllo')) == '5'
assert repr('héllo'[1:3]) == "'él'"
assert repr('日本語'[1]) == "'本'"
assert repr('abc'[::-1]) == "'cba'"
assert repr('Å'.lower()) == "'å'"
assert repr('ß'.upper()) == "'SS'"
assert repr(ord('é')) == '233'
assert repr(chr(233)) == "'é'"
assert repr(len('日本'.encode())) == '6'
assert repr('aé'.find('é')) == '1'
assert repr('x—y'.replace('—', '-')) == "'x-y'"
try:
    import json
    assert repr(json.loads('2')) == '2'
except Exception as ex:
    raise ex
try:
    import json
    assert repr(json.loads('2.5')) == '2.5'
except Exception as ex:
    raise ex
try:
    import json
    assert repr(json.loads('null')) == 'None'
except Exception as ex:
    raise ex
try:
    import json
    assert repr(json.loads('[1, [2, [3]]]')) == '[1, [2, [3]]]'
except Exception as ex:
    raise ex
try:
    import json
    def t():
        try:
            return json.loads('{bad')
        except ValueError:
            return 'VE'
    assert repr(t()) == "'VE'"
except Exception as ex:
    raise ex
try:
    import json
    assert repr(json.loads('"\\u0041"')) == "'A'"
except Exception as ex:
    raise ex
try:
    import json
    assert repr(json.dumps('a"b\\c')) == '\'"a\\\\"b\\\\\\\\c"\''
except Exception as ex:
    raise ex
try:
    lst = [1, 2, 3]
    del lst[1]
    assert repr(lst) == '[1, 3]'
except Exception as ex:
    raise ex
try:
    d = {'a': 1, 'b': 2}
    del d['a']
    assert repr(sorted(d.keys())) == "['b']"
except Exception as ex:
    raise ex
try:
    counter = 0
    def bump():
        global counter
        counter += 1
    bump(); bump()
    assert repr(counter) == '2'
except Exception as ex:
    raise ex
try:
    def outer():
        n = 0
        def inner():
            nonlocal n
            n += 1
        inner(); inner()
        return n
    assert repr(outer()) == '2'
except Exception as ex:
    raise ex
try:
    a = b = c = 5
    assert repr((a, b, c)) == '(5, 5, 5)'
except Exception as ex:
    raise ex
try:
    x, y = 1, 2
    x, y = y, x
    assert repr((x, y)) == '(2, 1)'
except Exception as ex:
    raise ex
try:
    class Ctx:
        def __enter__(self): return 'in'
        def __exit__(self, *a): return None
    with Ctx() as v: pass
    assert repr(v) == "'in'"
except Exception as ex:
    raise ex
try:
    out = [x * y for x in [1, 2] for y in [10, 20]]
    assert repr(out) == '[10, 20, 20, 40]'
except Exception as ex:
    raise ex
try:
    out = [x if x > 1 else -x for x in [1, 2, 3]]
    assert repr(out) == '[-1, 2, 3]'
except Exception as ex:
    raise ex
try:
    def f(a, b, /, *, c): return (a, b, c)
    assert repr(f(1, 2, c=3)) == '(1, 2, 3)'
except Exception as ex:
    raise ex
try:
    total = 0
    for i in range(3):
        for j in range(3):
            if j > i: break
            total += 1
    assert repr(total) == '6'
except Exception as ex:
    raise ex
try:
    g = {'k': 1}
    val = g.pop('missing', 'dflt')
    assert repr(val) == "'dflt'"
except Exception as ex:
    raise ex
try:
    lst = [5, 3, 1]
    while lst:
        lst.sort()
        lst.pop()
    assert repr(lst) == '[]'
except Exception as ex:
    raise ex
try:
    import os
    assert repr(os.path.join('a', 'b', 'c')) == "'a/b/c'"
except Exception as ex:
    raise ex
try:
    import os
    assert repr(os.path.basename('/x/y/z.txt')) == "'z.txt'"
except Exception as ex:
    raise ex
try:
    import os
    assert repr(os.path.dirname('/x/y/z.txt')) == "'/x/y'"
except Exception as ex:
    raise ex
try:
    import os
    assert repr(os.path.splitext('archive.tar.gz')) == "('archive.tar', '.gz')"
except Exception as ex:
    raise ex
try:
    import os
    assert repr(os.path.abspath('..') != '') == 'True'
except Exception as ex:
    raise ex
try:
    import os
    assert repr(os.path.isabs('/x')) == 'True'
except Exception as ex:
    raise ex
assert repr(list(zip(*[[1, 2], [3, 4]]))) == '[(1, 3), (2, 4)]'
assert repr(sorted({3, 1, 2}, reverse=True)) == '[3, 2, 1]'
assert repr('abc' > 'abd') == 'False'
assert repr([1] * 3 + [2] * 2) == '[1, 1, 1, 2, 2]'
assert repr(sum(range(10))) == '45'
assert repr('a1b2'.isdigit() if False else 'a1'.isalnum()) == 'True'
assert repr(max(['apple', 'fig'], key=len)) == "'apple'"

True
