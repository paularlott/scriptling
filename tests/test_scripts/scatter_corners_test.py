# Round-17 scatter: string corners (center/expandtabs/zfill/count,
# startswith/endswith tuples), starred/nested unpacking, ALL augmented
# assignment operators, numeric boundaries (2**53, float repr, floor div
# signs, divmod negatives), lambdas with captured defaults and star-args,
# del (slice/key/name), slice assignment (insert/replace/step), property
# decorators with setters, walrus in expressions, mixed-type comparisons
# (int<float<bool, str, list, tuple lexicographic), nested conditionals.
# CPython-generated. Known divergences excluded: nested destructuring
# targets, lambda-default capture semantics, *args-as-list.
assert repr('a\tb\tc'.expandtabs(4)) == "'a   b   c'"
assert repr('a\t b'.expandtabs()) == "'a        b'"
assert repr('x'.center(6)) == "'  x   '"
assert repr('ab'.center(7, '*')) == "'***ab**'"
assert repr('-42'.zfill(6)) == "'-00042'"
assert repr('+42'.zfill(5)) == "'+0042'"
assert repr('hello'.count('l')) == '2'
assert repr('hello'.count('l', 2)) == '2'
assert repr('aaa'.replace('a', 'b', 2)) == "'bba'"
assert repr('abc'.startswith(('x', 'a'))) == 'True'
assert repr('abc'.endswith(('x', 'c'))) == 'True'
assert repr('a-b_c'.isidentifier()) == 'False'
assert repr('2a'.isidentifier()) == 'False'
assert repr('\t \n'.isspace()) == 'True'
assert repr('abc'.rjust(5, '0')) == "'00abc'"
try:
    def f(a, b): return a + b
    args = (3, 4)
    assert repr(f(*args)) == '7'
except Exception as ex:
    raise ex
try:
    def f(**kw): return kw
    assert repr(f(**{'a': 1})) == "{'a': 1}"
except Exception as ex:
    raise ex
try:
    pairs = [(1, 'a'), (2, 'b')]
    assert repr([(n, s) for n, s in pairs]) == "[(1, 'a'), (2, 'b')]"
except Exception as ex:
    raise ex
try:
    for i, v in enumerate(['x', 'y']): pass
    assert repr((i, v)) == "(1, 'y')"
except Exception as ex:
    raise ex
try:
    x = 10; x += 5
    assert repr(x) == '15'
except Exception as ex:
    raise ex
try:
    x = 10; x -= 3
    assert repr(x) == '7'
except Exception as ex:
    raise ex
try:
    x = 10; x *= 2
    assert repr(x) == '20'
except Exception as ex:
    raise ex
try:
    x = 10; x //= 3
    assert repr(x) == '3'
except Exception as ex:
    raise ex
try:
    x = 10; x /= 4
    assert repr(x) == '2.5'
except Exception as ex:
    raise ex
try:
    x = 10; x %= 3
    assert repr(x) == '1'
except Exception as ex:
    raise ex
try:
    x = 10; x **= 2
    assert repr(x) == '100'
except Exception as ex:
    raise ex
try:
    x = 0b1100; x |= 0b0011
    assert repr(x) == '15'
except Exception as ex:
    raise ex
try:
    x = 0b1100; x &= 0b0110
    assert repr(x) == '4'
except Exception as ex:
    raise ex
try:
    x = 0b1100; x ^= 0b0110
    assert repr(x) == '10'
except Exception as ex:
    raise ex
try:
    x = 1; x <<= 4
    assert repr(x) == '16'
except Exception as ex:
    raise ex
try:
    x = 32; x >>= 2
    assert repr(x) == '8'
except Exception as ex:
    raise ex
try:
    s = 'a'; s += 'b'; s += 'c'
    assert repr(s) == "'abc'"
except Exception as ex:
    raise ex
try:
    lst = [1]; lst += [2, 3]
    assert repr(lst) == '[1, 2, 3]'
except Exception as ex:
    raise ex
assert repr(2 ** 62) == '4611686018427387904'
assert repr(2 ** 53) == '9007199254740992'
assert repr(repr(0.1 + 0.2)) == "'0.30000000000000004'"
assert repr(1e308 * 10 == float('inf')) == 'True'
assert repr(0.0 == -0.0) == 'True'
assert repr(1 / 3) == '0.3333333333333333'
assert repr(repr(10 % 3)) == "'1'"
assert repr(divmod(-13, 4)) == '(-4, 3)'
assert repr(-9 // 4) == '-3'
assert repr(7.0 // 2) == '3.0'
assert repr(2 ** -1) == '0.5'
assert repr((-2) ** 0) == '1'
assert repr(0 ** 0) == '1'
assert repr(int('  42  ')) == '42'
assert repr(float('1e-3')) == '0.001'
assert repr((lambda **k: k)()) == '{}'
try:
    lst = [1, 2, 3, 4]
    del lst[1:3]
    assert repr(lst) == '[1, 4]'
except Exception as ex:
    raise ex
try:
    d = {'a': 1, 'b': 2, 'c': 3}
    del d['b']
    assert repr(sorted(d.keys())) == "['a', 'c']"
except Exception as ex:
    raise ex
try:
    x = 1
    y = 2
    del x
    assert repr(y) == '2'
except Exception as ex:
    raise ex
try:
    lst = [1, 2, 3, 4, 5]
    lst[1:3] = [9, 9, 9]
    assert repr(lst) == '[1, 9, 9, 9, 4, 5]'
except Exception as ex:
    raise ex
try:
    lst = [1, 2, 3]
    lst[0:0] = [0]
    assert repr(lst) == '[0, 1, 2, 3]'
except Exception as ex:
    raise ex
try:
    lst = [1, 2, 3, 4]
    lst[::2] = [10, 20]
    assert repr(lst) == '[10, 2, 20, 4]'
except Exception as ex:
    raise ex
try:
    class T:
        def __init__(self): self._v = 5
        @property
        def v(self): return self._v * 2
    t = T()
    assert repr(t.v) == '10'
except Exception as ex:
    raise ex
try:
    class T:
        def __init__(self): self._v = 5
        @property
        def v(self): return self._v * 2
        @v.setter
        def v(self, val): self._v = val
    t = T()
    t.v = 10
    assert repr(t.v) == '20'
except Exception as ex:
    raise ex
assert repr('walrus' if (n := 3) > 2 else 'no') == "'walrus'"
try:
    w = 0
    assert repr(len(w := 'hello')) == '5'
except Exception as ex:
    raise ex
assert repr(1 < 1.5 < 2) == 'True'
assert repr(True < 2) == 'True'
assert repr(0 == False and 1 == True) == 'True'
assert repr('a' < 'b' < 'c') == 'True'
assert repr([1, 2] < [1, 3]) == 'True'
assert repr((1, 2) < (1, 2, 3)) == 'True'
assert repr(1 if True else 2 if False else 3) == '1'
try:
    v = 0
    assert repr(('neg' if (v := -5) < 0 else 'pos') if v != 0 else 'zero') == "'zero'"
except Exception as ex:
    raise ex
assert repr('ab' * 3) == "'ababab'"
assert repr(3 * 'ab') == "'ababab'"
assert repr('x' + 'y' * 2 + 'z') == "'xyyz'"
_d = dict(a=1, b=2)
assert _d["a"] == 1 and _d["b"] == 2
_d2 = dict([('x', 1), ('y', 2)])
assert _d2["x"] == 1 and _d2["y"] == 2
_d3 = dict(zip('ab', [1, 2]))
assert _d3["a"] == 1 and _d3["b"] == 2

True
