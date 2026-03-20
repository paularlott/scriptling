# Test import statement functionality

# Single import
import math
assert math.sqrt(4) == 2.0

# Multiple imports on one line
import json, re
assert json.dumps({"a": 1}) != ""
assert re.match(r"\d+", "123") is not None

# from import
from math import pi, floor
assert floor(3.9) == 3
assert pi > 3.14 and pi < 3.15

# import as
import math as m
assert m.pow(2, 3) == 8.0

# from import as
from math import sqrt as sq
assert sq(9) == 3.0
