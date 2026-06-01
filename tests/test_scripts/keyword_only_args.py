failures = 0

def resize(image, *, width, height=100):
    return image + ":" + str(width) + "x" + str(height)

if resize("avatar", width=64) != "avatar:64x100":
    failures += 1

if resize("avatar", width=64, height=32) != "avatar:64x32":
    failures += 1

missing_required = False
try:
    resize("avatar")
except Exception:
    missing_required = True

if not missing_required:
    failures += 1

positional_rejected = False
try:
    resize("avatar", 64)
except Exception:
    positional_rejected = True

if not positional_rejected:
    failures += 1

def collect(prefix, *items, sep):
    result = prefix
    for item in items:
        result += sep + str(item)
    return result

if collect("items", 1, 2, 3, sep=",") != "items,1,2,3":
    failures += 1

lambda_scale = lambda value, *, scale=2: value * scale

if lambda_scale(4) != 8:
    failures += 1

if lambda_scale(4, scale=3) != 12:
    failures += 1

lambda_positional_rejected = False
try:
    lambda_scale(4, 3)
except Exception:
    lambda_positional_rejected = True

if not lambda_positional_rejected:
    failures += 1

assert failures == 0
