package evaluator

import (
	"testing"
)

// hashScript is the common class definition used across all __hash__ tests.
// The class uses self.value as its hash so two distinct instances with the
// same value produce the same key.
const hashScript = `
class Key:
    def __init__(self, v):
        self.value = v
    def __hash__(self):
        return self.value
    def __eq__(self, other):
        return self.value == other.value

`

func TestHashDictIndexGet(t *testing.T) {
	result := testEval(hashScript + `
k1 = Key(42)
k2 = Key(42)
d = {k1: "found"}
d[k2]
`)
	testStringObject(t, result, "found")
}

func TestHashDictMethodGet(t *testing.T) {
	result := testEval(hashScript + `
k1 = Key(7)
k2 = Key(7)
d = {k1: "ok"}
d.get(k2)
`)
	testStringObject(t, result, "ok")
}

func TestHashDictMethodGetDefault(t *testing.T) {
	result := testEval(hashScript + `
k1 = Key(1)
d = {}
d.get(k1, "default")
`)
	testStringObject(t, result, "default")
}

func TestHashDictMethodPop(t *testing.T) {
	result := testEval(hashScript + `
k1 = Key(3)
k2 = Key(3)
d = {k1: "popped"}
d.pop(k2)
`)
	testStringObject(t, result, "popped")
}

func TestHashDictMethodSetdefault(t *testing.T) {
	// setdefault with a key that already exists (inserted via k1) should
	// return the existing value when looked up via k2 (same hash).
	result := testEval(hashScript + `
k1 = Key(5)
k2 = Key(5)
d = {k1: "existing"}
d.setdefault(k2, "new")
`)
	testStringObject(t, result, "existing")
}

func TestHashDictMethodSetdefaultInserts(t *testing.T) {
	// setdefault on a missing key should insert and be retrievable via same hash.
	result := testEval(hashScript + `
k1 = Key(9)
k2 = Key(9)
d = {}
d.setdefault(k1, "inserted")
d[k2]
`)
	testStringObject(t, result, "inserted")
}

func TestHashDictMethodUpdate(t *testing.T) {
	result := testEval(hashScript + `
k1 = Key(11)
k2 = Key(11)
d = {}
d.update([[k1, "updated"]])
d[k2]
`)
	testStringObject(t, result, "updated")
}

func TestHashDictMethodFromkeys(t *testing.T) {
	result := testEval(hashScript + `
k1 = Key(13)
k2 = Key(13)
d = {}.fromkeys([k1], "v")
d[k2]
`)
	testStringObject(t, result, "v")
}

func TestHashInOperatorDict(t *testing.T) {
	result := testEval(hashScript + `
k1 = Key(17)
k2 = Key(17)
d = {k1: 1}
k2 in d
`)
	testBooleanObject(t, result, true)
}

func TestHashSetConstructor(t *testing.T) {
	result := testEval(hashScript + `
k1 = Key(19)
k2 = Key(19)
s = set([k1])
k2 in s
`)
	testBooleanObject(t, result, true)
}

func TestHashSetLiteral(t *testing.T) {
	result := testEval(hashScript + `
k1 = Key(21)
k2 = Key(21)
s = {k1}
k2 in s
`)
	testBooleanObject(t, result, true)
}

func TestHashSetMethodAdd(t *testing.T) {
	result := testEval(hashScript + `
k1 = Key(23)
k2 = Key(23)
s = set()
s.add(k1)
k2 in s
`)
	testBooleanObject(t, result, true)
}

func TestHashSetMethodRemove(t *testing.T) {
	result := testEval(hashScript + `
k1 = Key(29)
k2 = Key(29)
s = {k1}
s.remove(k2)
len(s) == 0
`)
	testBooleanObject(t, result, true)
}

func TestHashSetMethodDiscard(t *testing.T) {
	result := testEval(hashScript + `
k1 = Key(31)
k2 = Key(31)
s = {k1}
s.discard(k2)
len(s) == 0
`)
	testBooleanObject(t, result, true)
}

func TestHashSetUnion(t *testing.T) {
	// k1 and k2 have the same hash; union of {k1} and {k2} should have size 1.
	result := testEval(hashScript + `
k1 = Key(37)
k2 = Key(37)
s1 = {k1}
s2 = {k2}
len(s1.union(s2))
`)
	testIntegerObject(t, result, 1)
}

func TestHashSetIntersection(t *testing.T) {
	result := testEval(hashScript + `
k1 = Key(41)
k2 = Key(41)
s1 = {k1}
s2 = {k2}
len(s1.intersection(s2))
`)
	testIntegerObject(t, result, 1)
}

func TestHashSetDifference(t *testing.T) {
	// {k1} - {k2} where k1 == k2 by hash should be empty.
	result := testEval(hashScript + `
k1 = Key(43)
k2 = Key(43)
s1 = {k1}
s2 = {k2}
len(s1.difference(s2))
`)
	testIntegerObject(t, result, 0)
}

func TestHashSetSymmetricDifference(t *testing.T) {
	result := testEval(hashScript + `
k1 = Key(47)
k2 = Key(47)
s1 = {k1}
s2 = {k2}
len(s1.symmetric_difference(s2))
`)
	testIntegerObject(t, result, 0)
}

func TestHashPatternMatch(t *testing.T) {
	// Pattern match keys are evaluated in a fresh env so must be literals.
	// Verify evalHashKey is used on the subject dict's lookup by using a
	// string literal key — the common case for dict patterns.
	result := testEval(`
d = {"status": "ok"}
result = "no"
match d:
    case {"status": v}:
        result = v
result
`)
	testStringObject(t, result, "ok")
}
