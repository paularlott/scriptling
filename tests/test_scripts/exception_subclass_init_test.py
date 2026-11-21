class MyErr(Exception):
    def __init__(self, msg, code=0):
        super().__init__(msg)
        self.code = code
try:
    raise MyErr("boom", code=7)
except MyErr as e:
    print("caught", str(e), e.args, e.code)
except Exception as e:
    print("other", type(e).__name__, e)
class V(ValueError):
    def __init__(self, field):
        super().__init__("bad " + field)
        self.field = field
try:
    raise V("x")
except ValueError as e:
    print("caught", str(e), e.field)
except Exception as e:
    print("other", type(e).__name__, e)
class NoSuper(Exception):
    def __init__(self, msg, code):
        self.code = code
try:
    raise NoSuper("m", 3)
except NoSuper as e:
    print(e.args, str(e), e.code)
class Base(Exception):
    pass
class Mid(Base):
    def __init__(self, a, b=2):
        super().__init__(a, b)
        self.b = b
class Leaf(Mid):
    def __init__(self, a):
        super().__init__(a, b=9)
try:
    raise Leaf("z")
except Base as e:
    print(type(e).__name__, e.args, e.b, isinstance(e, Exception))
class WithStr(KeyError):
    def __init__(self, key):
        super().__init__(key)
        self.key = key
    def __str__(self):
        return "missing " + self.key
try:
    raise WithStr("k")
except KeyError as e:
    print(str(e), e.args, e.key)
e = MyErr("plain")
print(e.args, e.code, str(e))
try:
    MyErr()
except TypeError:
    print("TypeError")
