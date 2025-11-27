failures = 0

# Try/except
caught = False
try:
    raise "test error"
except Exception as e:
    caught = True
if not caught:
    failures += 1

# Finally
finally_exec = False
try:
    pass
finally:
    finally_exec = True
if not finally_exec:
    failures += 1

failures == 0