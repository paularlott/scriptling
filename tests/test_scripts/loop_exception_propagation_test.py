# Regression coverage for exception/error propagation out of loops. Previously a
# `raise` (or any EXCEPTION_OBJ) in a for/while body was silently swallowed: the
# loop's result switch handled ERROR/RETURN/BREAK/CONTINUE but not EXCEPTION, so
# the exception was overwritten by the next iteration. Now all loop paths return
# EXCEPTION_OBJ immediately, aborting the loop and propagating to the caller.

failures = 0

# (1) raise in a for-list propagates out of the function.
def f_list():
    for x in [1, 2, 3]:
        if x == 2:
            raise ValueError("stop")
    return "done"
try:
    f_list()
    failures += 1
    print("FAIL for-list did not propagate")
except Exception as e:
    if str(e) != "stop":
        failures += 1
        print("FAIL for-list wrong msg:", e)

# (2) raise in a for-dict-items loop (the 2-var fast path) propagates.
def f_items():
    d = {"a": 1, "b": 2, "c": 3}
    for k, v in d.items():
        if k == "b":
            raise ValueError("stopI")
    return "done"
try:
    f_items()
    failures += 1
    print("FAIL for-items did not propagate")
except Exception as e:
    if str(e) != "stopI":
        failures += 1
        print("FAIL for-items wrong msg:", e)

# (3) raise in a while-loop propagates.
def f_while():
    i = 0
    while i < 5:
        if i == 2:
            raise ValueError("stopW")
        i += 1
    return "done"
try:
    f_while()
    failures += 1
    print("FAIL while did not propagate")
except Exception as e:
    if str(e) != "stopW":
        failures += 1
        print("FAIL while wrong msg:", e)

# (4) The loop must STOP at the raise — remaining iterations must not run.
ran = []
def f_stop():
    for x in [1, 2, 3, 4]:
        ran.append(x)
        if x == 2:
            raise ValueError("halt")
try:
    f_stop()
except Exception:
    pass
# x==3 and x==4 must never have been appended (deterministic list order).
if ran != [1, 2]:
    failures += 1
    print("FAIL loop did not stop at raise, ran=", ran)

# (5) An enclosing try/except catches the loop-raised exception.
def f_caught():
    total = 0
    try:
        for x in [1, 2, 3]:
            if x == 2:
                raise ValueError("boom")
            total += x
    except ValueError:
        total = -1
    return total
if f_caught() != -1:
    failures += 1
    print("FAIL enclosing try did not catch, total=", f_caught())

# (6) A runtime error (ERROR_OBJ) in a loop still propagates (no regression).
def f_err():
    for x in [1, 0, 2]:
        y = 10 / x
    return "done"
errored = False
try:
    f_err()
except Exception as e:
    errored = "division" in str(e) or "zero" in str(e)
if not errored:
    failures += 1
    print("FAIL runtime error did not propagate")

# (7) break / continue / for-else still behave (no regression from the fix).
out = []
for x in [1, 2, 3, 4]:
    if x == 3:
        break
    out.append(x)
ran_else = False
for x in []:
    pass
else:
    ran_else = True
if out != [1, 2] or not ran_else:
    failures += 1
    print("FAIL control-flow regression: break=", out, "else=", ran_else)

assert failures == 0
