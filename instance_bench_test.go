package scriptling

import "testing"

// Instance creation, the path class-instance memory accounting sits on.
// Memory accounting is off here; BenchmarkLimits_Instances is the same
// script with it on.
const instanceBenchScript = `
class Node:
    def __init__(self, value, nxt):
        self.value = value
        self.next = nxt

head = None
for i in range(3000):
    head = Node(i, head)
total = 0
n = head
while n is not None:
    total = total + n.value
    n = n.next
`

func BenchmarkInstance_Create(b *testing.B) {
	benchScript(b, instanceBenchScript)
}

// Building lists and tuples from other values in Go code, the paths that
// report their backing arrays to memory accounting.
const containerBenchScript = `
import itertools
src = [i for i in range(64)]
total = 0
for n in range(400):
    a = list(src)
    b = tuple(a)
    c = list(itertools.chain(a, a))
    d = list(zip(a, b))
    total = total + len(a) + len(b) + len(c) + len(d)
`

func BenchmarkContainer_Builtins(b *testing.B) {
	benchScript(b, containerBenchScript)
}
