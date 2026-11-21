# Sequence, class and mapping patterns in match statements

def seq(value):
    match value:
        case ():
            return "empty"
        case (1, y):
            return ("one-then", y)
        case [a, b]:
            return ("pair", a, b)
        case [first, *rest] if first == "head":
            return ("head", rest)
        case (x, (y, z)):
            return ("nested", x, y, z)
        case [_, _, _]:
            return "triple"
        case [*init, last] if len(init) > 2:
            return ("init", init, last)
        case _:
            return "other"

assert seq([]) == "empty"
assert seq(()) == "empty"
assert seq((1, 5)) == ("one-then", 5)
assert seq([1, 5]) == ("one-then", 5)
assert seq([2, 3]) == ("pair", 2, 3)
assert seq(("head", 1, 2)) == ("head", [1, 2])
assert seq(["head"]) == ("head", [])
assert seq((0, (8, 9))) == ("pair", 0, (8, 9))
assert seq((0, [8, 9], 7)) == "triple"
assert seq([1, 2, 3, 4]) == ("init", [1, 2, 3], 4)
assert seq("ab") == "other"
assert seq({"a": 1, "b": 2}) == "other"
assert seq(5) == "other"

# Nested sequences, wildcards and starred names in the middle
def nested(value):
    match value:
        case [(x, y), [z, *_]]:
            return x + y + z
        case [a, *mid, b] if mid:
            return (a, mid, b)
        case [a, *_, b]:
            return (a, b)
    return None

assert nested([(1, 2), [3, 4, 5]]) == 6
assert nested([1, 2, 3, 4]) == (1, [2, 3], 4)
assert nested([1, 4]) == (1, 4)
assert nested([1]) is None

# Open sequence patterns and an open-tuple subject
def open_seq(a, b):
    match a, b:
        case 0, 0:
            return "origin"
        case 0, y:
            return ("y-axis", y)
        case x, *others:
            return ("x", x, others)

assert open_seq(0, 0) == "origin"
assert open_seq(0, 3) == ("y-axis", 3)
assert open_seq(2, 3) == ("x", 2, [3])

# Parenthesised subjects
point = (1, 2)
match (point):
    case (1, y):
        found = y
assert found == 2

match (1, 2):
    case (a, b):
        total = a + b
assert total == 3

match [3, 4]:
    case [a, b]:
        total = a * b
assert total == 12

# OR patterns inside sequences
def or_inside(value):
    match value:
        case [1 | 2, x]:
            return ("small", x)
        case ("a" | "b") as letter:
            return ("letter", letter)
        case [x, 3 | 4] | [3 | 4, x]:
            return ("has34", x)
    return "none"

assert or_inside([1, 9]) == ("small", 9)
assert or_inside([2, 9]) == ("small", 9)
assert or_inside("b") == ("letter", "b")
assert or_inside([9, 4]) == ("has34", 9)
assert or_inside([3, 7]) == ("has34", 7)
assert or_inside([5, 5]) == "none"

# Class patterns
class Point:
    __match_args__ = ("x", "y")
    def __init__(self, x, y):
        self.x = x
        self.y = y

class Point3(Point):
    def __init__(self, x, y, z):
        super().__init__(x, y)
        self.z = z

class Plain:
    def __init__(self):
        self.kind = "plain"

def where(p):
    match p:
        case Point(x=0, y=0):
            return "origin"
        case Point(x=3):
            return "x is 3"
        case Point(x=px, y=0):
            return ("on x axis", px)
        case Point3(a, b, z=zz):
            return ("3d", a, b, zz)
        case Point(a, b) if a == b:
            return ("diagonal", a)
        case Point(a, b):
            return ("point", a, b)
        case Plain(kind=k):
            return ("plain", k)
        case Plain(missing=m):
            return "unreachable"
    return "unknown"

assert where(Point(0, 0)) == "origin"
assert where(Point(3, 5)) == "x is 3"
assert where(Point(7, 0)) == ("on x axis", 7)
assert where(Point3(1, 2, 3)) == ("3d", 1, 2, 3)
assert where(Point(4, 4)) == ("diagonal", 4)
assert where(Point(4, 5)) == ("point", 4, 5)
assert where(Plain()) == ("plain", "plain")
assert where("text") == "unknown"

# A missing attribute fails the match rather than raising
class Empty:
    pass

match Empty():
    case Empty(value=v):
        result = "matched"
    case Empty():
        result = "type only"
assert result == "type only"

# Too many positional sub-patterns is a TypeError
try:
    match Point(1, 2):
        case Point(a, b, c):
            pass
    assert False, "expected TypeError"
except TypeError:
    pass

# Built-in type patterns, with the subject itself as the positional pattern
def kind(v):
    match v:
        case bool():
            return "bool"
        case int(n) if n < 0:
            return "negative int"
        case int():
            return "int"
        case float(f):
            return ("float", f)
        case str() | bytes():
            return "text"
        case list([x, *_]):
            return ("list starting", x)
        case dict():
            return "dict"
        case tuple():
            return "tuple"
    return "other"

assert kind(True) == "bool"
assert kind(-3) == "negative int"
assert kind(3) == "int"
assert kind(1.5) == ("float", 1.5)
assert kind("s") == "text"
assert kind(b"s") == "text"
assert kind([7, 8]) == ("list starting", 7)
assert kind({}) == "dict"
assert kind((1,)) == "tuple"
assert kind(None) == "other"

# Class patterns nested in sequences and mappings
def shapes(items):
    out = []
    for item in items:
        match item:
            case [Point(x=0), Point(x=0)]:
                out.append("vertical")
            case {"at": Point(x, y)}:
                out.append(("at", x, y))
            case _:
                out.append("?")
    return out

assert shapes([[Point(0, 1), Point(0, 2)], {"at": Point(5, 6)}, 1]) == ["vertical", ("at", 5, 6), "?"]

# Literal patterns compare with ==, singletons by identity
def lit(v):
    match v:
        case True:
            return "True"
        case 1:
            return "one"
        case None:
            return "None"
        case -2:
            return "minus two"
        case "x":
            return "x"
    return "no"

assert lit(True) == "True"
assert lit(1) == "one"
assert lit(1.0) == "one"
assert lit(None) == "None"
assert lit(-2) == "minus two"
assert lit("x") == "x"
assert lit(0) == "no"

# Dotted value patterns
class Color:
    RED = "red"
    GREEN = "green"

def color(c):
    match c:
        case Color.RED:
            return "stop"
        case Color.GREEN:
            return "go"
        case other:
            return "unknown " + other

assert color("red") == "stop"
assert color("green") == "go"
assert color("blue") == "unknown blue"

# Mapping patterns, including **rest
def mapping(m):
    match m:
        case {"type": "point", "x": x, "y": y}:
            return ("point", x, y)
        case {"type": "tagged", **rest}:
            return ("tagged", rest)
        case {"items": [first, *_]}:
            return ("items", first)
        case {}:
            return "any mapping"
    return "not a mapping"

assert mapping({"type": "point", "x": 1, "y": 2, "extra": 0}) == ("point", 1, 2)
assert mapping({"type": "tagged", "a": 1, "b": 2}) == ("tagged", {"a": 1, "b": 2})
assert mapping({"items": [9, 8]}) == ("items", 9)
assert mapping({"z": 1}) == "any mapping"
assert mapping([1]) == "not a mapping"

# Guards, captures and no-match fallthrough
def guarded(v):
    match v:
        case [x, y] if x > y:
            return "desc"
        case [x, y] if x < y:
            return "asc"
    return "fallthrough"

assert guarded([2, 1]) == "desc"
assert guarded([1, 2]) == "asc"
assert guarded([1, 1]) == "fallthrough"
assert guarded("no") == "fallthrough"

# Capture variables bound inside a function are locals
def capture_local(v):
    match v:
        case [a, (b, c)]:
            pass
    return a + b + c

assert capture_local([1, (2, 3)]) == 6
a = "global a"
assert capture_local([10, (20, 30)]) == 60
assert a == "global a"

# Captures in a loop body inside a function
def sum_pairs(pairs):
    total = 0
    for p in pairs:
        match p:
            case (x, y):
                total += x * y
            case Point(x=px, y=py):
                total += px - py
    return total

assert sum_pairs([(1, 2), (3, 4), Point(10, 3)]) == 21

# as on the whole pattern
match [1, 2]:
    case [1, _] as whole:
        captured = whole
assert captured == [1, 2]

print("OK")
