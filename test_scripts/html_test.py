import html

# Test escape - basic HTML entities
escaped = html.escape("<div>")
assert "&lt;" in escaped
assert "&gt;" in escaped

# Test escape - single quotes
escaped3 = html.escape("'single'")
assert len(escaped3) > 0

# Test unescape - basic entities
assert html.unescape("&lt;div&gt;") == "<div>"
assert html.unescape("&amp;") == "&"

# Test roundtrip
original = "<script>"
assert html.unescape(html.escape(original)) == original

# Test with plain text (no escaping needed)
plain = "Hello World"
assert html.escape(plain) == plain
assert html.unescape(plain) == plain

# Test html library
escaped = html.escape("<script>alert('xss')</script>")
assert "&lt;" in escaped
assert "&gt;" in escaped

unescaped = html.unescape("&lt;script&gt;")
assert unescaped == "<script>"

original = "<div>"
assert html.unescape(html.escape(original)) == original

# Tests for html.parser library (Python-compatible HTMLParser)
HTMLParser = html.parser.HTMLParser

# Test 1: Basic subclass that collects tags
class TagCollector(HTMLParser):
    def __init__(self):
        self.tags = []
        self.data = []

    def handle_starttag(self, tag, attrs):
        self.tags.append(("start", tag, attrs))

    def handle_endtag(self, tag):
        self.tags.append(("end", tag))

    def handle_data(self, data):
        self.data.append(data)

parser = TagCollector()
parser.feed("<html><body><p>Hello</p></body></html>")

assert len(parser.tags) == 6
item = parser.tags[0]
assert item[0] == "start"
assert item[1] == "html"
assert len(item[2]) == 0
item = parser.tags[1]
assert item[0] == "start"
assert item[1] == "body"
assert len(item[2]) == 0
item = parser.tags[2]
assert item[0] == "start"
assert item[1] == "p"
assert len(item[2]) == 0
item = parser.tags[3]
assert item[0] == "end"
assert item[1] == "p"
item = parser.tags[4]
assert item[0] == "end"
assert item[1] == "body"
item = parser.tags[5]
assert item[0] == "end"
assert item[1] == "html"
assert "Hello" in parser.data

# Test 2: Attributes parsing
class AttrParser(HTMLParser):
    def __init__(self):
        self.attrs = []

    def handle_starttag(self, tag, attrs):
        self.attrs.append((tag, attrs))

parser2 = AttrParser()
parser2.feed('<a href="https://example.com" class="link">Link</a>')

assert len(parser2.attrs) == 1
tag, attrs = parser2.attrs[0]
assert tag == "a"
assert len(attrs) == 2