# Test exception type matching

# Bare except catches all
caught = False
try:
    x = 1 / 0
except:
    caught = True
assert caught

# except Exception as e catches all
caught = False
try:
    y = 1 / 0
except Exception as e:
    caught = True
    assert "division" in str(e) or len(str(e)) > 0
assert caught

# Specific exception type does NOT catch a different type
caught_inner = False
caught_outer = False
try:
    try:
        z = 1 / 0
    except ValueError as e:
        caught_inner = True
except:
    caught_outer = True
assert not caught_inner
assert caught_outer

# Exception variable binding with raise
msg = ""
try:
    raise Exception("custom message")
except Exception as e:
    msg = str(e)
assert msg == "custom message"

# Multiple except clauses — first matching one runs
result = ""
try:
    raise ValueError("val error")
except TypeError:
    result = "type"
except ValueError:
    result = "value"
except Exception:
    result = "generic"
assert result == "value"

# try/except/else — else runs when no exception
else_ran = False
try:
    x = 1 + 1
except Exception:
    pass
else:
    else_ran = True
assert else_ran

# try/except/else — else does NOT run when exception is caught
else_ran = False
try:
    raise ValueError("oops")
except ValueError:
    pass
else:
    else_ran = True
assert not else_ran

# try/finally — finally always runs
finally_ran = False
try:
    pass
finally:
    finally_ran = True
assert finally_ran

# try/except/finally — finally runs even when exception is caught
finally_ran = False
try:
    raise ValueError("x")
except ValueError:
    pass
finally:
    finally_ran = True
assert finally_ran

# Re-raise with bare raise
caught_outer = False
try:
    try:
        raise ValueError("original")
    except ValueError:
        raise
except ValueError as e:
    caught_outer = True
    assert str(e) == "original"
assert caught_outer
