# strutils

String manipulation utilities.

## Functions

### slugify(text)

Convert text to a URL-friendly slug.

```python
import strutils

strutils.slugify("Hello, World!")   # "hello-world"
strutils.slugify("My Blog Post")    # "my-blog-post"
```

### truncate(text, max_len, suffix="...")

Truncate text to `max_len` characters, appending `suffix` if trimmed.

```python
strutils.truncate("A rather long piece of text", 20)        # "A rather long piec..."
strutils.truncate("Short", 20)                              # "Short"
strutils.truncate("A rather long piece of text", 20, "…")   # "A rather long piec…"
```

### pad_left(text, width, char=" ")

Left-pad `text` to `width` characters using `char`.

```python
strutils.pad_left("42", 6)        # "    42"
strutils.pad_left("42", 6, "0")   # "000042"
```

### pad_right(text, width, char=" ")

Right-pad `text` to `width` characters using `char`.

```python
strutils.pad_right("hi", 6)   # "hi    "
```
