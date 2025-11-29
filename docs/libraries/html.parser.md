# html.parser Library (Extended)

The `html.parser` library provides an HTML/XHTML parser compatible with Python's `html.parser` module. This is an **extended library** that is automatically available as a built-in library.

## Import

```python
import html.parser

HTMLParser = html.parser.HTMLParser
```

## HTMLParser Class

The `HTMLParser` class is the main interface for parsing HTML documents. Create a subclass and override handler methods to process HTML elements.

### Basic Usage

```python
import html.parser

class MyHTMLParser(html.parser.HTMLParser):
    def __init__(self):
        self.data = []

    def handle_starttag(self, tag, attrs):
        print(f"Start tag: {tag}")
        print(f"Attrs: {attrs}")

    def handle_endtag(self, tag):
        print(f"End tag: {tag}")

    def handle_data(self, data):
        self.data.append(data)

parser = MyHTMLParser()
parser.feed("<html><body><p>Hello World!</p></body></html>")
```

## Instance Methods

### feed(data)
Feed HTML data to the parser. Parses the HTML and calls appropriate handler methods.

**Parameters:**
- `data` - String containing HTML to parse

**Example:**
```python
parser.feed("<h1>Title</h1><p>Paragraph</p>")
```

### reset()
Reset the parser instance. Clears internal buffers and parser state.

```python
parser.reset()
```

### close()
Force processing of all buffered data. Call when done feeding data.

```python
parser.close()
```

### get_starttag_text()
Returns the text of the most recently opened start tag.

```python
class MyParser(html.parser.HTMLParser):
    def handle_starttag(self, tag, attrs):
        text = self.get_starttag_text()
        print(f"Raw tag: {text}")
```

### getpos()
Returns a tuple (line, offset) representing the current position in the source.

```python
pos = parser.getpos()
print(f"Line: {pos[0]}, Offset: {pos[1]}")
```

## Handler Methods

Override these methods in your subclass to handle different HTML elements:

### handle_starttag(tag, attrs)
Called when a start tag is encountered.

**Parameters:**
- `tag` - Lowercase tag name (e.g., "div", "p")
- `attrs` - List of (name, value) tuples for attributes

```python
def handle_starttag(self, tag, attrs):
    print(f"<{tag}>")
    for name, value in attrs:
        print(f"  {name}={value}")
```

### handle_endtag(tag)
Called when an end tag is encountered.

**Parameters:**
- `tag` - Lowercase tag name

```python
def handle_endtag(self, tag):
    print(f"</{tag}>")
```

### handle_startendtag(tag, attrs)
Called for self-closing tags like `<br/>` or `<img/>`.

**Parameters:**
- `tag` - Lowercase tag name
- `attrs` - List of (name, value) tuples

```python
def handle_startendtag(self, tag, attrs):
    print(f"<{tag}/>")
```

### handle_data(data)
Called for text data between tags.

**Parameters:**
- `data` - Text content

```python
def handle_data(self, data):
    if data.strip():
        print(f"Text: {data}")
```

### handle_comment(data)
Called when an HTML comment is encountered.

**Parameters:**
- `data` - Comment content (without <!-- and -->)

```python
def handle_comment(self, data):
    print(f"Comment: {data}")
```

### handle_decl(decl)
Called for DOCTYPE and other declarations.

**Parameters:**
- `decl` - Declaration content

```python
def handle_decl(self, decl):
    print(f"Declaration: {decl}")
```

### handle_pi(data)
Called for processing instructions like `<?xml ...?>`.

**Parameters:**
- `data` - Processing instruction content

```python
def handle_pi(self, data):
    print(f"PI: {data}")
```

### handle_entityref(name)
Called for named character references like `&gt;`. Only called when `convert_charrefs` is False.

**Parameters:**
- `name` - Entity name (without & and ;)

### handle_charref(name)
Called for numeric character references like `&#62;`. Only called when `convert_charrefs` is False.

**Parameters:**
- `name` - Character code (without &# and ;)

## Instance Attributes

### convert_charrefs
Boolean indicating whether to automatically convert character references. Default is True.

When True, entities like `&amp;` are converted to `&` before being passed to `handle_data`.

```python
# To handle entities manually:
parser.convert_charrefs = False
```

## Complete Example

```python
import html.parser

class LinkExtractor(html.parser.HTMLParser):
    def __init__(self):
        self.links = []
        self.current_text = ""
        self.in_link = False

    def handle_starttag(self, tag, attrs):
        if tag == "a":
            self.in_link = True
            for name, value in attrs:
                if name == "href":
                    self.links.append({"href": value, "text": ""})

    def handle_endtag(self, tag):
        if tag == "a":
            if self.links and self.in_link:
                self.links[-1]["text"] = self.current_text.strip()
            self.current_text = ""
            self.in_link = False

    def handle_data(self, data):
        if self.in_link:
            self.current_text = self.current_text + data

# Extract links from HTML
parser = LinkExtractor()
parser.feed("""
<html>
<body>
    <a href="https://example.com">Example</a>
    <a href="https://google.com">Google</a>
</body>
</html>
""")

for link in parser.links:
    print(f"{link['text']}: {link['href']}")
# Output:
# Example: https://example.com
# Google: https://google.com
```

## Enabling in Go

The `html.parser` library is automatically available as a built-in library. To explicitly register it:

```go
package main

import (
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/extlibs"
)

func main() {
    p := scriptling.New()

    // Optionally register explicitly (already available by default)
    p.RegisterLibrary("html.parser", extlibs.HTMLParserLibrary)

    code := `
import html.parser

class MyParser(html.parser.HTMLParser):
    def handle_starttag(self, tag, attrs):
        print(tag)

parser = MyParser()
parser.feed("<div>Hello</div>")
`
    p.Eval(code)
}
```

## Differences from Python

- `super().__init__()` is not required (or available) - the parent class __init__ is automatically called
- The `from X import Y` syntax is not supported - use `import html.parser` then `html.parser.HTMLParser`

## See Also

- [html](./html.md) - HTML escaping and unescaping utilities
- [re](./re.md) - Regular expressions for text processing
