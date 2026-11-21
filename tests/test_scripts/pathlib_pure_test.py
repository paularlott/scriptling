import os
from pathlib import Path

p = Path("/a/b/c.tar.gz")
assert str(p.with_suffix(".zip")) == "/a/b/c.tar.zip"
assert str(p.with_suffix("")) == "/a/b/c.tar"
assert str(p.with_name("x.y")) == "/a/b/x.y"
assert str(p.with_stem("d")) == "/a/b/d.gz"
assert Path("x/y.txt").with_suffix(".md").name == "y.md"
assert p.suffixes == [".tar", ".gz"]
assert p.parts == ("/", "a", "b", "c.tar.gz")
assert p.is_absolute() and not Path("a").is_absolute()
assert str(p.relative_to("/a")) == "b/c.tar.gz"
assert str(Path("a/b").relative_to(Path("a"))) == "b"
assert p.is_relative_to("/a") and not p.is_relative_to("/x")
assert p.as_posix() == "/a/b/c.tar.gz"
assert repr(p.with_suffix(".x")) == "PosixPath('/a/b/c.tar.x')"


def value_error(fn):
    try:
        fn()
    except ValueError as e:
        return str(e)
    return None


assert value_error(lambda: p.with_suffix("zip")) == "Invalid suffix 'zip'"
assert value_error(lambda: Path("/").with_suffix(".x")) == "PosixPath('/') has an empty name"
assert value_error(lambda: p.with_name("")) == "Invalid name ''"
assert value_error(lambda: p.with_name("a/b")) == "Invalid name 'a/b'"
assert value_error(lambda: p.relative_to("/x")) == "'/a/b/c.tar.gz' is not in the subpath of '/x'"
assert value_error(lambda: Path("a.txt").with_stem("")) == "PosixPath('a.txt') has a non-empty suffix"

# paths compare, hash and sort by their path
assert Path("a") == Path("a/") and Path("a") != Path("b")
assert len({Path("a"), Path("a"), Path("b")}) == 2
assert [str(x) for x in sorted([Path("b"), Path("a")])] == ["a", "b"]

# a Path reads as its path where a string is expected
assert os.path.join(Path("/a"), "b") == "/a/b"

print("OK")
