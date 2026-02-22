import io

# Basic write and getvalue
buf = io.StringIO()
buf.write("hello")
buf.write(" world")
assert buf.getvalue() == "hello world", f"getvalue failed: {buf.getvalue()}"

# Initial value
buf2 = io.StringIO("initial")
assert buf2.getvalue() == "initial", f"initial value failed: {buf2.getvalue()}"

# read from beginning
buf3 = io.StringIO("abcdef")
assert buf3.read(3) == "abc", "read(3) failed"
assert buf3.read(3) == "def", "read(3) second failed"
assert buf3.read() == "", "read() at end failed"

# seek and tell
buf4 = io.StringIO("hello")
buf4.seek(2)
assert buf4.tell() == 2, f"tell after seek failed: {buf4.tell()}"
assert buf4.read() == "llo", f"read after seek failed: {buf4.read()}"

# readline
buf5 = io.StringIO("line1\nline2\nline3")
assert buf5.readline() == "line1\n", f"readline 1 failed: {buf5.readline()}"
assert buf5.readline() == "line2\n", f"readline 2 failed"
assert buf5.readline() == "line3", f"readline 3 (no newline) failed"
assert buf5.readline() == "", "readline at end failed"

# truncate
buf6 = io.StringIO("hello world")
buf6.truncate(5)
assert buf6.getvalue() == "hello", f"truncate failed: {buf6.getvalue()}"

# close raises on subsequent operations
buf7 = io.StringIO("data")
buf7.close()
try:
    buf7.write("more")
    assert False, "should have raised on closed buffer"
except Exception as e:
    assert "closed" in str(e), f"wrong error: {e}"

# with statement
with io.StringIO() as buf8:
    buf8.write("context")
    val = buf8.getvalue()
assert val == "context", f"with statement failed: {val}"

# with statement closes on exit
try:
    buf8.write("after close")
    assert False, "should have raised after with block"
except Exception as e:
    assert "closed" in str(e), f"wrong error after with: {e}"

# print with file= kwarg
buf9 = io.StringIO()
print("hello", file=buf9)
print("world", file=buf9)
assert buf9.getvalue() == "hello\nworld\n", f"print file= failed: {buf9.getvalue()}"

# print with sep and end to StringIO
buf10 = io.StringIO()
print("a", "b", "c", sep=",", end="!\n", file=buf10)
assert buf10.getvalue() == "a,b,c!\n", f"print sep/end to file= failed: {buf10.getvalue()}"

True
