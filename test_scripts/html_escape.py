# Test html library
import html

# Test escape
escaped = html.escape("<script>alert('xss')</script>")
"&lt;" in escaped
"&gt;" in escaped

# Test unescape
unescaped = html.unescape("&lt;script&gt;")
unescaped == "<script>"

# Test roundtrip
original = "<div>"
html.unescape(html.escape(original)) == original
