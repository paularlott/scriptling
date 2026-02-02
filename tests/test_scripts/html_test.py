import html
import html.parser

# Test html.escape() - Go html encoding (&#39; for ', &#34; for ")
assert html.escape("<div>") == "&lt;div&gt;"
assert html.escape("&") == "&amp;"
assert html.escape('"') == "&#34;"
assert html.escape("'") == "&#39;"
assert html.escape("<script>alert('xss')</script>") == "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;"

# Test html.unescape() - handles both numeric and named entities
assert html.unescape("&lt;div&gt;") == "<div>"
assert html.unescape("&amp;") == "&"
assert html.unescape("&quot;") == '"'
assert html.unescape("&#34;") == '"'
assert html.unescape("&#x27;") == "'"
assert html.unescape("&#39;") == "'"

# Test roundtrip
assert html.unescape(html.escape("<script>")) == "<script>"
assert html.unescape(html.escape("Hello World")) == "Hello World"

# Test html.parser.HTMLParser - Python 3 compatible
HTMLParser = html.parser.HTMLParser

# Test 1: Basic tag collection (Python 3 compatible)
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
assert parser.tags[0] == ("start", "html", [])
assert parser.tags[1] == ("start", "body", [])
assert parser.tags[2] == ("start", "p", [])
assert parser.tags[3] == ("end", "p")
assert parser.tags[4] == ("end", "body")
assert parser.tags[5] == ("end", "html")
assert "Hello" in parser.data

# Test 2: Attribute parsing (Python 3 compatible)
class AttrParser(HTMLParser):
    def __init__(self):
        self.attrs = []

    def handle_starttag(self, tag, attrs):
        self.attrs.append((tag, attrs))

parser2 = AttrParser()
parser2.feed('<a href="https://example.com" class="link">Link</a>')

assert len(parser2.attrs) == 1
assert parser2.attrs[0][0] == "a"
assert len(parser2.attrs[0][1]) == 2
assert parser2.attrs[0][1][0] == ("href", "https://example.com")
assert parser2.attrs[0][1][1] == ("class", "link")


# Test 3: Various import combinations to prevent state pollution
import html.parser as hp1
import html as h1

import html.parser as hp2
import html.parser
import html as h2
import html

# All should have escape function
assert h1.escape("<") == "&lt;"
assert h2.escape("<") == "&lt;"
assert html.escape("<") == "&lt;"

# All should have parser
assert hasattr(h1, "parser")
assert hasattr(h2, "parser")
assert hasattr(html, "parser")

# Aliases should work
assert hasattr(hp1, "HTMLParser")
assert hasattr(hp2, "HTMLParser")
