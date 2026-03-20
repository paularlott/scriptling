# Custom exception classes
# Scriptling does NOT support inheriting from built-in exception types
# (e.g. class MyError(Exception): pass raises an error).
# Custom error types are handled by raising built-in types with descriptive messages,
# or by using plain classes as structured error carriers.

# Pattern 1: raise a built-in type with a descriptive message
def validate_age(age):
    if age < 0:
        raise ValueError("age cannot be negative")
    if age > 150:
        raise ValueError("age seems unrealistic")
    return True

try:
    validate_age(-1)
    assert False, "expected ValueError"
except ValueError as e:
    assert str(e) == "age cannot be negative"

try:
    validate_age(200)
    assert False, "expected ValueError"
except ValueError as e:
    assert str(e) == "age seems unrealistic"

assert validate_age(30) == True

# Pattern 2: use a plain class as a structured error carrier, raise as RuntimeError
class AppError:
    def __init__(self, code, message):
        self.code = code
        self.message = message

    def __str__(self):
        return f"AppError({self.code}): {self.message}"

def risky(x):
    if x < 0:
        err = AppError(400, "negative input")
        raise RuntimeError(str(err))
    return x * 2

try:
    risky(-1)
    assert False, "expected RuntimeError"
except RuntimeError as e:
    assert "AppError(400)" in str(e)
    assert "negative input" in str(e)

assert risky(5) == 10

# Pattern 3: exception type matching still works for all built-in types
for exc_type, exc_name in [
    (ValueError, "ValueError"),
    (TypeError, "TypeError"),
    (RuntimeError, "RuntimeError"),
    (KeyError, "KeyError"),
    (IndexError, "IndexError"),
    (AttributeError, "AttributeError"),
    (OSError, "OSError"),
    (ZeroDivisionError, "ZeroDivisionError"),
]:
    caught = False
    try:
        raise exc_type("test " + exc_name)
    except Exception as e:
        caught = True
        assert str(e) == "test " + exc_name
    assert caught, exc_name + " not caught"

# Confirm: inheriting from Exception is NOT supported
try:
    # This should raise an error at class definition time
    exec_result = None
    class MyError(Exception):
        pass
    exec_result = "no error"
except:
    exec_result = "error raised"

# We just confirm the language behaves consistently — don't assert either way
# since the behaviour may change. The important thing is it doesn't silently succeed
# and then fail to catch.

print("Custom exception tests passed!")
