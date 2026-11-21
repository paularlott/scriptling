package scriptling

import "testing"

// Loops that unpack (index, value), zipped and (key, value) pairs: each pair
// is one allocation.
func BenchmarkInterp_PairIteration(b *testing.B) {
	benchScript(b, `
names = ["a", "b", "c", "d", "e", "f", "g", "h"]
d = {"a": 1, "b": 2, "c": 3, "d": 4}
total = 0
for r in range(300):
    for i, name in enumerate(names):
        total += i
    for x, y in zip(names, names):
        total += 1
    for k, v in d.items():
        total += v
    pairs = list(d.items())
`)
}

// Keyword arguments to a lambda, a method and a constructor: each binds
// straight from the call site, with no keyword map.
func BenchmarkInterp_KeywordCallees(b *testing.B) {
	benchScript(b, `
class Point:
    def __init__(self, x, y=0):
        self.x = x
        self.y = y
    def moved(self, dx=0, dy=0):
        return self.x + dx + self.y + dy
scale = lambda v, factor=2: v * factor
p = Point(1, y=2)
total = 0
for i in range(2000):
    total += scale(i, factor=3) + p.moved(dy=i)
    q = Point(i, y=1)
`)
}

// Scopes created per iteration: comprehensions, generator expressions and
// closures each get their own environment.
func BenchmarkInterp_Scopes(b *testing.B) {
	benchScript(b, `
def adder(n):
    def add(x):
        return x + n
    return add
total = 0
for i in range(1500):
    squares = [j * j for j in range(4)]
    total += sum(j for j in squares) + adder(i)(1)
    lookup = {j: j for j in range(3)}
`)
}

// Loops over a dict's keys and values.
func BenchmarkInterp_DictLoops(b *testing.B) {
	benchScript(b, `
d = {"alpha": 1, "bravo": 2, "charlie": 3, "delta": 4, "echo": 5}
total = 0
for i in range(2000):
    for k in d:
        total += 1
    for v in d.values():
        total += v
`)
}
