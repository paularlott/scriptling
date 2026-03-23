# numutils

Numeric utility functions.

## Functions

### clamp(value, min_val, max_val)

Clamp `value` so it stays within `[min_val, max_val]`.

```python
import numutils

numutils.clamp(150, 0, 100)   # 100
numutils.clamp(-5, 0, 100)    # 0
numutils.clamp(42, 0, 100)    # 42
```

### lerp(a, b, t)

Linear interpolation between `a` and `b` by factor `t` (0.0–1.0).

```python
numutils.lerp(0, 100, 0.5)   # 50.0
numutils.lerp(0, 100, 0.25)  # 25.0
```

### remap(value, in_min, in_max, out_min, out_max)

Remap `value` from one numeric range to another.

```python
numutils.remap(0.5, 0, 1, 0, 255)   # 127.5
numutils.remap(50, 0, 100, -1, 1)   # 0.0
```

### sign(value)

Return `-1`, `0`, or `1` depending on the sign of `value`.

```python
numutils.sign(-42)   # -1
numutils.sign(0)     # 0
numutils.sign(7)     # 1
```
