# Test pathlib Path joinpath
import pathlib

# Test joinpath with multiple arguments
p = pathlib.Path("/home/user")
p2 = p.joinpath("docs", "readme.txt")
assert p2["__str__"] == "/home/user/docs/readme.txt"

# Test chaining
p3 = pathlib.Path("a").joinpath("b").joinpath("c")
assert p3["__str__"] == "a/b/c"

# Test with absolute path in join
p4 = pathlib.Path("/home").joinpath("/etc", "passwd")
assert p4["__str__"] == "/etc/passwd"  # joinpath replaces with absolute

assert True