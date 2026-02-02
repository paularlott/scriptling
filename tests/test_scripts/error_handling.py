# Try/except
caught = False
try:
    raise "test error"
except Exception as e:
    caught = True
assert caught

# Finally
finally_exec = False
try:
    pass
finally:
    finally_exec = True
assert finally_exec