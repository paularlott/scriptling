# Test textwrap library
import textwrap

# Test wrap
lines = textwrap.wrap("Hello world this is a test of text wrapping", 20)
assert len(lines) > 1

# Test wrap with width parameter
lines = textwrap.wrap("Short text", width=50)
assert len(lines) == 1

# Test fill
result = textwrap.fill("Hello world this is a test", 15)
assert "\n" in result

# Test dedent
text = "    Hello\n    World\n    Test"
result = textwrap.dedent(text)
lines = result.split("\n")
assert lines[0] == "Hello"
assert lines[1] == "World"

# Test indent
text = "Hello\nWorld"
result = textwrap.indent(text, "  ")
lines = result.split("\n")
assert lines[0] == "  Hello"
assert lines[1] == "  World"

# Test shorten
result = textwrap.shorten("Hello World this is a very long text", 20)
assert len(result) <= 20
assert "[...]" in result

# Test shorten with custom placeholder
result = textwrap.shorten("Hello World this is a very long text", 20, placeholder="...")
assert len(result) <= 20
assert "..." in result

# Test shorten when text fits
result = textwrap.shorten("Short", 20)
assert result == "Short"
