# Test html library - comprehensive
import html

# Test escape - basic HTML entities
escaped = html.escape("<div>")
"&lt;" in escaped
"&gt;" in escaped

# Test escape - single quotes
escaped3 = html.escape("'single'")
len(escaped3) > 0

# Test unescape - basic entities
html.unescape("&lt;div&gt;") == "<div>"
html.unescape("&amp;") == "&"

# Test roundtrip
original = "<script>"
html.unescape(html.escape(original)) == original

# Test with plain text (no escaping needed)
plain = "Hello World"
html.escape(plain) == plain
html.unescape(plain) == plain
