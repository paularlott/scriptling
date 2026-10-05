# Test int and float methods (bit_length, bit_count, is_integer, hex,
# fromhex, as_integer_ratio)

# int.bit_length
assert (0).bit_length() == 0
assert (1).bit_length() == 1
assert (5).bit_length() == 3
assert (-5).bit_length() == 3
assert (255).bit_length() == 8
assert (256).bit_length() == 9
assert (2 ** 62).bit_length() == 63
assert 9223372036854775807 .bit_length() == 63

# int.bit_count
assert (0).bit_count() == 0
assert (5).bit_count() == 2
assert (-5).bit_count() == 2
assert (255).bit_count() == 8

# int.is_integer (Python 3.12): always True
assert (5).is_integer()
assert (-5).is_integer()

# float.is_integer
assert (2.0).is_integer()
assert (2.5).is_integer() is False
assert (-8.0).is_integer()
assert (0.0).is_integer()

# float.hex: full 13-hex-digit mantissa, unpadded binary exponent
assert (2.0).hex() == "0x1.0000000000000p+1"
assert (0.1).hex() == "0x1.999999999999ap-4"
assert (255.5).hex() == "0x1.ff00000000000p+7"
assert (-2.0).hex() == "-0x1.0000000000000p+1"
assert (0.0).hex() == "0x0.0p+0"

# float.fromhex
assert float.fromhex("0x1.8p+1") == 3.0
assert float.fromhex("0x1.8") == 1.5
assert float.fromhex("-0x1.8p+0") == -1.5
assert float.fromhex("0x1p+2") == 4.0
assert float.fromhex("inf") == float("inf")
assert float.fromhex((2.0).hex()) == 2.0
assert float.fromhex((0.1).hex()) == 0.1

def raises(exc, fn):
    try:
        fn()
        return False
    except exc:
        return True

assert raises(ValueError, lambda: float.fromhex("not-a-hex-float"))

# float.as_integer_ratio: exact reduced ratio
assert (0.5).as_integer_ratio() == [1, 2]
assert (2.0).as_integer_ratio() == [2, 1]
assert (0.0).as_integer_ratio() == [0, 1]
assert (0.1).as_integer_ratio() == [3602879701896397, 36028797018963968]
assert (-0.5).as_integer_ratio() == [-1, 2]
assert raises(OverflowError, lambda: (5e-324).as_integer_ratio())

# Methods work as values and through their type, like str methods
bl = (5).bit_length
assert bl() == 3
assert int.bit_count(5) == 2
assert int.bit_length(-5) == 3
assert float.fromhex("0x1.8p+1") == 3.0
assert hasattr(5, "bit_length")
assert not hasattr(5, "nope")
assert "bit_length" in dir(5)
assert "as_integer_ratio" in dir(2.5)

# getattr form agrees with dot access
assert getattr(5, "bit_count")() == 2

True

# OverflowError is an ArithmeticError: parent handlers catch it
try:
    (5e-324).as_integer_ratio()
    caught = "none"
except ArithmeticError:
    caught = "arithmetic"
assert caught == "arithmetic"
