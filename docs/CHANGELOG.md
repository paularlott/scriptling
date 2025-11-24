# Scriptling Changelog

## Recent Changes

### None Literal Support ✅

**Added:** `None` keyword for representing null/absent values

**Why:** None is fundamental to Python and essential for LLM code generation. Without it, LLMs would constantly hallucinate `None` usage, causing syntax errors.

**Usage:**
```python
x = None
if x == None:
    print("x is None")

# None is falsy
if not x:
    print("x is falsy")
```

**Benefits:**
- Matches Python 3 behavior
- Prevents LLM hallucination errors
- Enables proper null value handling
- No need for sentinel values (0, "", -1)

### True Division (Python 3 Style) ✅

**Changed:** Division operator `/` now always returns float

**Before:**
```python
5 / 2    # Returned 2 (integer division - bug!)
```

**After:**
```python
5 / 2    # Returns 2.5 (true division - correct!)
5 % 2    # Returns 1 (modulo for remainder)
```

**Benefits:**
- Matches Python 3 behavior
- LLM-friendly (trained on Python 3)
- Fewer bugs from unexpected integer division
- Clear semantics: `/` for division, `%` for modulo

### Library API Improvements ✅

**Changed:** JSON and HTTP functions now use dot notation and cleaner API

**JSON:**
```python
# Old: json_parse(), json_stringify()
# New: json.parse(), json.stringify()
data = json.parse('{"name":"Alice"}')
json_str = json.stringify({"key": "value"})
```

**HTTP:**
```python
# Old: Ambiguous argument order
http_get(url, timeout, headers)  # or headers, timeout?

# New: Clear options dictionary
options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token"}
}
response = http.get(url, options)
```

**Benefits:**
- More Pythonic (matches json.loads, requests patterns)
- Unambiguous API (single signature per function)
- Self-documenting (options dictionary)
- Default 5 second timeout (was 30)
- Easy to extend with new options

### Import Behavior Clarification ✅

**Clarified:** `import()` function behavior is now explicitly documented

**How it works:**
```python
# import() loads the library and creates a global object
import("json")    # Creates global 'json' object
import("http")    # Creates global 'http' object

# Use the global objects directly
data = json.parse('...')
response = http.get(url, options)
```

**Side-effect approach:**
- Simple and intuitive for scripting
- Matches Python's import behavior conceptually
- No need for assignment: `json = import("json")`

### Documentation Organization ✅

**Changed:** Moved documentation to `docs/` folder

**Structure:**
```
scriptling/
├── README.md                    # Quick start
├── docs/
│   ├── LANGUAGE_GUIDE.md       # Complete language reference
│   ├── GO_INTEGRATION.md       # Go embedding guide
│   ├── LIBRARIES.md            # Library system
│   ├── QUICK_REFERENCE.md      # Quick reference
│   └── CHANGELOG.md            # This file
└── BUILD_PLAN.md               # Architecture
```

## Summary of Changes

### Core Language
- ✅ Added `None` literal for null values
- ✅ Changed `/` to true division (always returns float)
- ✅ Clarified `import()` creates global objects

### API Changes
- ✅ `json_parse()` → `json.parse()`
- ✅ `json_stringify()` → `json.stringify()`
- ✅ `http_get()` → `http.get()`
- ✅ HTTP functions use options dictionary
- ✅ Default HTTP timeout: 30s → 5s

### Documentation
- ✅ Moved docs to `docs/` folder
- ✅ Updated all examples
- ✅ Clarified import() behavior
- ✅ Removed "No None literal" from differences
- ✅ Added None to all code examples

### Testing
- ✅ All tests updated and passing
- ✅ Added None tests
- ✅ Fixed division tests for float results

## Migration Guide

### For None
No migration needed - this is a new feature. You can now use `None` where you previously used workarounds.

### For Division
If you relied on integer division:
```python
# Before
x = 10 / 3    # Was 3

# After
x = 10 / 3    # Now 3.333...
x = int(10 / 3)  # Use int() to get 3
```

### For JSON/HTTP
Update function names and use options dictionary:
```python
# Before
data = json_parse(text)
response = http_get(url, 10)

# After
data = json.parse(text)
options = {"timeout": 10}
response = http.get(url, options)
```

## For LLMs

When generating Scriptling code:
1. Use `None` for null values (not null, nil, or undefined)
2. Division `/` returns float: `5 / 2 = 2.5`
3. Use `json.parse()` and `json.stringify()` (dot notation)
4. Use `http.get()`, `http.post()`, etc. with options dictionary
5. `import()` creates global objects: `import("json")` then use `json.parse()`
6. Default HTTP timeout is 5 seconds
7. Use `True`, `False`, `None` (all capitalized)
