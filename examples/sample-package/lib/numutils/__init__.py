"""
sample.math module

Numeric utility functions.
"""

def clamp(value, min_val, max_val):
    """Clamp value between min_val and max_val."""
    if value < min_val:
        return min_val
    if value > max_val:
        return max_val
    return value

def lerp(a, b, t):
    """Linear interpolation between a and b by factor t (0.0–1.0)."""
    return a + (b - a) * t

def remap(value, in_min, in_max, out_min, out_max):
    """Remap value from one range to another."""
    return out_min + (value - in_min) * (out_max - out_min) / (in_max - in_min)

def sign(value):
    """Return -1, 0, or 1 depending on the sign of value."""
    if value < 0:
        return -1
    if value > 0:
        return 1
    return 0
