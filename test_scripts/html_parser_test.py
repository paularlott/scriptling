# Tests for html.parser library (Python-compatible HTMLParser)
import html.parser

# Get the HTMLParser class
HTMLParser = html.parser.HTMLParser

# Test 1: Basic subclass that collects tags
class TagCollector(HTMLParser):
    def __init__(self):
        # super() isn't available, but inherited __init__ will be called
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

assert len(parser.tags) == 6, f"Expected 6 tag events, got {len(parser.tags)}"
assert parser.tags[0] == ("start", "html", []), f"First tag wrong: {parser.tags[0]}"
assert parser.tags[1] == ("start", "body", []), f"Second tag wrong: {parser.tags[1]}"
assert parser.tags[2] == ("start", "p", []), f"Third tag wrong: {parser.tags[2]}"
assert parser.tags[3] == ("end", "p"), f"Fourth tag wrong: {parser.tags[3]}"
assert parser.tags[4] == ("end", "body"), f"Fifth tag wrong: {parser.tags[4]}"
assert parser.tags[5] == ("end", "html"), f"Sixth tag wrong: {parser.tags[5]}"
assert "Hello" in parser.data, f"Expected 'Hello' in data: {parser.data}"
print("Test 1 passed: Basic tag collection")

# Test 2: Attributes parsing
class AttrParser(HTMLParser):
    def __init__(self):
        self.attrs = []

    def handle_starttag(self, tag, attrs):
        self.attrs.append((tag, attrs))

parser2 = AttrParser()
parser2.feed('<a href="https://example.com" class="link">Link</a>')

assert len(parser2.attrs) == 1, f"Expected 1 start tag, got {len(parser2.attrs)}"
tag, attrs = parser2.attrs[0]
assert tag == "a", f"Expected tag 'a', got '{tag}'"
assert len(attrs) == 2, f"Expected 2 attrs, got {len(attrs)}"
assert attrs[0] == ("href", "https://example.com"), f"First attr wrong: {attrs[0]}"
assert attrs[1] == ("class", "link"), f"Second attr wrong: {attrs[1]}"
print("Test 2 passed: Attributes parsing")

# Test 3: Comments handling
class CommentParser(HTMLParser):
    def __init__(self):
        self.comments = []

    def handle_comment(self, data):
        self.comments.append(data)

parser3 = CommentParser()
parser3.feed("<!-- This is a comment --><p>Text</p><!-- Another one -->")

assert len(parser3.comments) == 2, f"Expected 2 comments, got {len(parser3.comments)}"
assert parser3.comments[0].strip() == "This is a comment", f"First comment wrong: {parser3.comments[0]}"
assert parser3.comments[1].strip() == "Another one", f"Second comment wrong: {parser3.comments[1]}"
print("Test 3 passed: Comments handling")

# Test 4: DOCTYPE handling
class DeclParser(HTMLParser):
    def __init__(self):
        self.decls = []

    def handle_decl(self, decl):
        self.decls.append(decl)

parser4 = DeclParser()
parser4.feed("<!DOCTYPE html><html></html>")

assert len(parser4.decls) == 1, f"Expected 1 declaration, got {len(parser4.decls)}"
assert "DOCTYPE" in parser4.decls[0], f"Expected DOCTYPE in decl: {parser4.decls[0]}"
print("Test 4 passed: DOCTYPE handling")

# Test 5: Self-closing tags
class SelfClosingParser(HTMLParser):
    def __init__(self):
        self.self_closing = []
        self.regular = []

    def handle_starttag(self, tag, attrs):
        self.regular.append(tag)

    def handle_startendtag(self, tag, attrs):
        self.self_closing.append(tag)

parser5 = SelfClosingParser()
parser5.feed("<br/><img src='test.png'/><p>Text</p>")

assert len(parser5.self_closing) == 2, f"Expected 2 self-closing tags, got {len(parser5.self_closing)}"
assert "br" in parser5.self_closing, f"Expected 'br' in self-closing: {parser5.self_closing}"
assert "img" in parser5.self_closing, f"Expected 'img' in self-closing: {parser5.self_closing}"
assert "p" in parser5.regular, f"Expected 'p' in regular tags: {parser5.regular}"
print("Test 5 passed: Self-closing tags")

# Test 6: get_starttag_text method
class TextParser(HTMLParser):
    def __init__(self):
        self.last_tag_text = None

    def handle_starttag(self, tag, attrs):
        self.last_tag_text = self.get_starttag_text()

parser6 = TextParser()
parser6.feed('<div class="container" id="main">Content</div>')

assert parser6.last_tag_text is not None, "Expected last_tag_text to be set"
assert "div" in parser6.last_tag_text.lower(), f"Expected 'div' in tag text: {parser6.last_tag_text}"
print("Test 6 passed: get_starttag_text method")

# Test 7: Mixed content
class MixedParser(HTMLParser):
    def __init__(self):
        self.events = []

    def handle_starttag(self, tag, attrs):
        self.events.append(("starttag", tag))

    def handle_endtag(self, tag):
        self.events.append(("endtag", tag))

    def handle_data(self, data):
        if data.strip():
            self.events.append(("data", data.strip()))

    def handle_comment(self, data):
        self.events.append(("comment", data.strip()))

parser7 = MixedParser()
parser7.feed("""
<html>
    <head>
        <title>Test</title>
    </head>
    <body>
        <!-- Main content -->
        <h1>Hello World</h1>
        <p>This is a <strong>test</strong>.</p>
    </body>
</html>
""")

# Verify we got expected event types
event_types = [e[0] for e in parser7.events]
assert "starttag" in event_types, "Missing starttag events"
assert "endtag" in event_types, "Missing endtag events"
assert "data" in event_types, "Missing data events"
assert "comment" in event_types, "Missing comment events"
print("Test 7 passed: Mixed content parsing")

# Test 8: reset() method
parser8 = TagCollector()
parser8.feed("<p>First</p>")
assert len(parser8.tags) > 0, "Should have tags after first feed"
parser8.reset()
parser8.tags = []  # Reset our custom field too
parser8.data = []
parser8.feed("<div>Second</div>")
assert ("start", "div", []) in parser8.tags, f"Should have div tag after reset: {parser8.tags}"
print("Test 8 passed: reset() method")

# Test 9: HTML entity handling (with convert_charrefs=True, the default)
class EntityParser(HTMLParser):
    def __init__(self):
        self.text = []

    def handle_data(self, data):
        self.text.append(data)

parser9 = EntityParser()
parser9.feed("<p>Hello &amp; goodbye &lt;test&gt;</p>")

full_text = "".join(parser9.text)
assert "&" in full_text, f"Expected decoded & in text: {full_text}"
assert "<test>" in full_text, f"Expected decoded <test> in text: {full_text}"
print("Test 9 passed: HTML entity handling")

# Test 10: Nested tags
class NestedParser(HTMLParser):
    def __init__(self):
        self.depth = 0
        self.max_depth = 0

    def handle_starttag(self, tag, attrs):
        self.depth = self.depth + 1
        if self.depth > self.max_depth:
            self.max_depth = self.depth

    def handle_endtag(self, tag):
        self.depth = self.depth - 1

parser10 = NestedParser()
parser10.feed("<div><ul><li><a>Link</a></li></ul></div>")

assert parser10.max_depth == 4, f"Expected max depth 4, got {parser10.max_depth}"
assert parser10.depth == 0, f"Expected final depth 0, got {parser10.depth}"
print("Test 10 passed: Nested tags")

print("All html.parser tests passed!")
True