failures = 0

class Box:
    def __init__(self, value):
        self.value = value

def check(value, expected):
    global failures
    if value != expected:
        failures += 1

# Attribute targets support every augmented assignment operator.
b = Box(10)
b.value += 5
check(b.value, 15)
b.value -= 3
check(b.value, 12)
b.value *= 2
check(b.value, 24)
b.value //= 5
check(b.value, 4)
b.value **= 3
check(b.value, 64)
b.value %= 10
check(b.value, 4)
b.value |= 8
check(b.value, 12)
b.value &= 10
check(b.value, 8)
b.value ^= 3
check(b.value, 11)
b.value <<= 1
check(b.value, 22)
b.value >>= 2
check(b.value, 5)
b.value /= 2
check(b.value, 2.5)

b.text = "hello"
b.text += " world"
check(b.text, "hello world")

b.items = [1, 2]
b.items += [3]
check(b.items, [1, 2, 3])

# List index targets use the same shorthand operators.
items = [10]
items[0] += 5
check(items[0], 15)
items[0] -= 3
check(items[0], 12)
items[0] *= 2
check(items[0], 24)
items[0] //= 5
check(items[0], 4)
items[0] **= 3
check(items[0], 64)
items[0] %= 10
check(items[0], 4)
items[0] |= 8
check(items[0], 12)
items[0] &= 10
check(items[0], 8)
items[0] ^= 3
check(items[0], 11)
items[0] <<= 1
check(items[0], 22)
items[0] >>= 2
check(items[0], 5)
items[0] /= 2
check(items[0], 2.5)

# Dict value targets too.
values = {"x": 10}
values["x"] += 5
check(values["x"], 15)
values["x"] -= 3
check(values["x"], 12)
values["x"] *= 2
check(values["x"], 24)
values["x"] //= 5
check(values["x"], 4)
values["x"] **= 3
check(values["x"], 64)
values["x"] %= 10
check(values["x"], 4)
values["x"] |= 8
check(values["x"], 12)
values["x"] &= 10
check(values["x"], 8)
values["x"] ^= 3
check(values["x"], 11)
values["x"] <<= 1
check(values["x"], 22)
values["x"] >>= 2
check(values["x"], 5)
values["x"] /= 2
check(values["x"], 2.5)

assert failures == 0
