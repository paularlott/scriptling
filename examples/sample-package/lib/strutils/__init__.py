"""
sample.strings module

String utility functions.
"""

def slugify(text):
    """Convert text to a URL-friendly slug."""
    result = text.lower()
    slug = ""
    for ch in result:
        if ch.isalnum():
            slug = slug + ch
        elif ch in " -_":
            slug = slug + "-"
    # Collapse repeated dashes
    while "--" in slug:
        slug = slug.replace("--", "-")
    return slug.strip("-")

def truncate(text, max_len, suffix="..."):
    """Truncate text to max_len characters, appending suffix if trimmed."""
    if len(text) <= max_len:
        return text
    return text[:max_len - len(suffix)] + suffix

def pad_left(text, width, char=" "):
    """Left-pad text to the given width."""
    return text.rjust(width, char)

def pad_right(text, width, char=" "):
    """Right-pad text to the given width."""
    return text.ljust(width, char)
