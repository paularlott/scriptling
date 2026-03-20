# Basic try/except
caught = False
try:
    raise Exception("test error")
except Exception as e:
    caught = True
    assert str(e) == "test error"
assert caught

# Bare except
caught = False
try:
    raise ValueError("oops")
except:
    caught = True
assert caught

# Multiple except clauses — first match wins
result = ""
try:
    raise ValueError("val")
except TypeError:
    result = "type"
except ValueError:
    result = "value"
except Exception:
    result = "generic"
assert result == "value"

# Finally always runs
finally_ran = False
try:
    pass
finally:
    finally_ran = True
assert finally_ran

# Finally runs even when exception is caught
log = []
try:
    raise ValueError("x")
except ValueError:
    log.append("except")
finally:
    log.append("finally")
assert log == ["except", "finally"]

# try/except/else — else runs when no exception
else_ran = False
try:
    x = 1 + 1
except Exception:
    pass
else:
    else_ran = True
assert else_ran

# try/except/else — else does NOT run when exception caught
else_ran = False
try:
    raise ValueError("oops")
except ValueError:
    pass
else:
    else_ran = True
assert not else_ran

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

# Exception propagates when not caught
propagated = False
try:
    try:
        raise ValueError("inner")
    except TypeError:
        pass  # doesn't match
except ValueError:
    propagated = True
assert propagated

# Nested try/except
outer_caught = False
inner_caught = False
try:
    try:
        raise ValueError("deep")
    except ValueError:
        inner_caught = True
        raise RuntimeError("wrapped")
except RuntimeError as e:
    outer_caught = True
    assert str(e) == "wrapped"
assert inner_caught
assert outer_caught

# raise X from Y is NOT supported — use raise ExcType(msg) directly
# raise RuntimeError("wrapped") from original_exc  # parse error
