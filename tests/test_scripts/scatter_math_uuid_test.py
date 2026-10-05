# Round-16 scatter: math trig/log family + new sinh/cosh/exp2/
# ldexp/frexp + log(base), hashlib sha512/384/224, uuid objects
# (version/variant/hex, UUID(), uuid3/uuid5 determinism), html, string
# module, Counter arithmetic, 3-level inheritance with super, decorator
# stacks and decorator factories, closures over loop variables,
# try/else/finally, sorted stability, nested comprehensions.
# CPython-generated. Dict-comprehension repr excluded (nondeterministic order).
try:
    import math
    assert repr(round(math.sin(0), 6)) == '0.0'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(round(math.cos(math.pi), 6)) == '-1.0'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(round(math.tan(0), 6)) == '0.0'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(round(math.asin(1), 6)) == '1.570796'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(round(math.acos(1), 6)) == '0.0'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(round(math.atan2(1, 1), 6)) == '0.785398'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(round(math.sinh(0), 6)) == '0.0'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(round(math.cosh(0), 6)) == '1.0'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(round(math.tanh(0), 6)) == '0.0'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(round(math.exp(1), 6)) == '2.718282'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(round(math.exp2(10), 6)) == '1024.0'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(round(math.expm1(1e-10), 15)) == '1e-10'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(round(math.log1p(1e-10), 15)) == '1e-10'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(round(math.log(8, 2), 6)) == '3.0'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(round(math.log10(1000), 6)) == '3.0'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(math.degrees(math.pi)) == '180.0'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(round(math.radians(180), 6)) == '3.141593'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(math.isfinite(1) and math.isinf(float('inf')) and math.isnan(float('nan'))) == 'True'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(math.isclose(0.1 + 0.2, 0.3)) == 'True'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(math.isclose(100, 100.0001, rel_tol=1e-3)) == 'True'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(math.copysign(3, -0.0)) == '-3.0'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(math.ldexp(3, 4)) == '48.0'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(math.frexp(8)[0]) == '0.5'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(math.trunc(-3.7)) == '-3'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(math.comb(5, 2)) == '10'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(math.perm(4, 2)) == '12'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(math.prod([2, 3, 4])) == '24'
except Exception as ex:
    raise ex
try:
    import math
    assert repr(math.dist((0, 0), (3, 4))) == '5.0'
except Exception as ex:
    raise ex
try:
    import hashlib
    assert repr(hashlib.sha1(b'abc').hexdigest()) == "'a9993e364706816aba3e25717850c26c9cd0d89d'"
except Exception as ex:
    raise ex
try:
    import hashlib
    assert repr(hashlib.sha512(b'abc').hexdigest()[:32]) == "'ddaf35a193617abacc417349ae204131'"
except Exception as ex:
    raise ex
try:
    import hashlib
    assert repr(hashlib.sha384(b'abc').hexdigest()[:32]) == "'cb00753f45a35e8bb5a03d699ac65007'"
except Exception as ex:
    raise ex
try:
    import hashlib
    assert repr(hashlib.sha224(b'abc').hexdigest()[:32]) == "'23097d223405d8228642a477bda255b3'"
except Exception as ex:
    raise ex
try:
    import uuid
    u = uuid.uuid4()
    assert repr(u.version) == '4'
except Exception as ex:
    raise ex
try:
    import uuid
    u = uuid.uuid4()
    assert repr(u.variant) == "'specified in RFC 4122'"
except Exception as ex:
    raise ex
try:
    import uuid
    u = uuid.uuid4()
    assert repr(len(u.hex)) == '32'
except Exception as ex:
    raise ex
try:
    import uuid
    assert repr(str(uuid.UUID('12345678-1234-5678-1234-567812345678'))) == "'12345678-1234-5678-1234-567812345678'"
except Exception as ex:
    raise ex
try:
    import uuid
    assert repr(uuid.uuid5(uuid.NAMESPACE_DNS, 'python.org') == uuid.uuid5(uuid.NAMESPACE_DNS, 'python.org')) == 'True'
except Exception as ex:
    raise ex
try:
    import uuid
    assert repr(uuid.uuid3(uuid.NAMESPACE_DNS, 'python.org') == uuid.uuid3(uuid.NAMESPACE_DNS, 'python.org')) == 'True'
except Exception as ex:
    raise ex
try:
    import html
    assert repr(html.escape("<b>&'")) == "'&lt;b&gt;&amp;&#x27;'"
except Exception as ex:
    raise ex
try:
    import html
    assert repr(html.unescape('&lt;b&gt;&amp;&#39;&#x27;')) == '"<b>&\'\'"'
except Exception as ex:
    raise ex
try:
    import html
    assert repr(html.escape('a', quote=False)) == "'a'"
except Exception as ex:
    raise ex
try:
    import string
    assert repr(string.ascii_uppercase[-3:]) == "'XYZ'"
except Exception as ex:
    raise ex
try:
    import string
    assert repr(string.octdigits) == "'01234567'"
except Exception as ex:
    raise ex
try:
    import string
    assert repr(string.punctuation[:5]) == '\'!"#$%\''
except Exception as ex:
    raise ex
try:
    import string
    assert repr(string.printable[:10]) == "'0123456789'"
except Exception as ex:
    raise ex
try:
    import collections
    c = collections.Counter()
    c['x'] += 1
    assert repr(c['x']) == '1'
except Exception as ex:
    raise ex
try:
    import collections
    c = collections.Counter('aab')
    assert repr(c - collections.Counter('a')) == "Counter({'a': 1, 'b': 1})"
except Exception as ex:
    raise ex
try:
    import collections
    c = collections.Counter(a=1)
    c.update({'a': 2, 'b': 1})
    assert repr(sorted(c.elements())) == "['a', 'a', 'a', 'b']"
except Exception as ex:
    raise ex
try:
    import collections
    assert repr(list(collections.Counter(x=3, y=1).most_common())) == "[('x', 3), ('y', 1)]"
except Exception as ex:
    raise ex
try:
    class A:
        def who(self): return 'A'
        def hello(self): return 'hi-A'
    class B(A):
        def who(self): return 'B+'
    class C(B):
        def who(self): return 'C++' + super().who()
        def greet(self): return super().hello()
    assert repr((C().who(), C().greet())) == "('C++B+', 'hi-A')"
except Exception as ex:
    raise ex
try:
    class A:
        def __init__(self): self.v = 'a'
    class B(A):
        def __init__(self):
            super().__init__()
            self.v += 'b'
    assert repr(B().v) == "'ab'"
except Exception as ex:
    raise ex
try:
    class A:
        x = 1
    class B(A):
        y = 2
    class C(B):
        z = 3
    assert repr(C.x + C.y + C.z) == '6'
except Exception as ex:
    raise ex
try:
    def deco1(f):
        def w(n): return f(n) + 1
        return w
    def deco2(f):
        def w(n): return f(n) * 2
        return w
    @deco1
    @deco2
    def base(n): return n
    assert repr(base(5)) == '11'
except Exception as ex:
    raise ex
try:
    def with_arg(mult):
        def deco(f):
            def w(n): return f(n) * mult
            return w
        return deco
    @with_arg(3)
    def val(n): return n + 1
    assert repr(val(4)) == '15'
except Exception as ex:
    raise ex
try:
    def make_adders():
        out = []
        for k in [1, 2, 3]:
            out.append(lambda x: x + k)
        return [f(10) for f in out]
    assert repr(make_adders()) == '[13, 13, 13]'
except Exception as ex:
    raise ex
try:
    def outer(x):
        def middle(y):
            def inner(z):
                return x + y + z
            return inner
        return middle
    assert repr(outer(1)(2)(3)) == '6'
except Exception as ex:
    raise ex
try:
    def counter_gen():
        n = 0
        def bump():
            nonlocal n
            n += 1
            return n
        return bump
    c = counter_gen()
    c(); c()
    assert repr(c()) == '3'
except Exception as ex:
    raise ex
try:
    def t():
        out = []
        try:
            out.append('t')
        except ValueError:
            out.append('e')
        else:
            out.append('el')
        finally:
            out.append('f')
        return out
    assert repr(t()) == "['t', 'el', 'f']"
except Exception as ex:
    raise ex
try:
    def t():
        out = []
        try:
            raise ValueError('x')
        except ValueError:
            out.append('e')
        else:
            out.append('el')
        finally:
            out.append('f')
        return out
    assert repr(t()) == "['e', 'f']"
except Exception as ex:
    raise ex
assert repr(sorted([(2, 'a'), (1, 'b'), (2, 'c'), (1, 'd')], key=lambda p: p[0])) == "[(1, 'b'), (1, 'd'), (2, 'a'), (2, 'c')]"
assert repr(sorted(['BB', 'a', 'CC', 'b'], key=str.lower)) == "['a', 'b', 'BB', 'CC']"
assert repr(max(['apple', 'fig', 'pear'], key=len)) == "'apple'"
assert repr([x * y for x in range(1, 4) for y in range(1, 4) if x * y > 4]) == '[6, 6, 9]'
assert repr([x for x in [y * 2 for y in [1, 2, 3]] if x > 2]) == '[4, 6]'
assert repr(sorted({x % 3 for x in range(10)})) == '[0, 1, 2]'

True
