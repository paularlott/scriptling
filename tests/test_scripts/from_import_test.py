# Test for 'from X import Y' syntax
from lib_with_class import Greeter

# Test that Greeter was imported correctly
g = Greeter("World")
msg = g.say_hello()
print(msg)
assert msg == "Hello, World"

# Test that the module itself wasn't imported
try:
    lib_with_class = lib_with_class  # This should fail
    assert False, "lib_with_class should not be available"
except NameError:
    pass  # Expected

# Test importing from html.parser
from html.parser import HTMLParser

class TestParser(HTMLParser):
    def __init__(self):
        self.tags = []

    def handle_starttag(self, tag, attrs):
        self.tags.append(tag)

parser = TestParser()
parser.feed("<html><body><p>test</p></body></html>")
assert "html" in parser.tags
assert "body" in parser.tags
assert "p" in parser.tags

print("from import test passed")
True