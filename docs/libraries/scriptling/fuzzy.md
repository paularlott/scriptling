# Fuzzy Library

The `scriptling.fuzzy` library provides fuzzy string matching utilities for searching and matching text. It uses a multi-tier matching algorithm that combines exact matching, substring matching, word boundary matching, and Levenshtein distance calculation.

## Import

```python
import scriptling.fuzzy as fuzzy
```

## Available Functions

| Function                                          | Description                                       |
| ------------------------------------------------- | ------------------------------------------------- |
| `search(query, items, max_results, threshold, key)` | Find multiple matches in a list                  |
| `best(query, items, entity_type, key, threshold)`  | Find single best match with error formatting     |
| `score(s1, s2)`                                     | Calculate similarity between two strings         |

## Overview

Fuzzy matching is useful when you need to:

- Find items when users might make typos or use partial names
- Implement "did you mean?" functionality in CLI tools
- Search through lists of items with flexible matching
- Calculate similarity scores between strings

## Functions

### `fuzzy.search(query, items, max_results=5, threshold=0.5, key="name") -> list`

Searches for fuzzy matches in a list of items using a multi-tier algorithm (exact → substring → word boundary → Levenshtein distance).

**Parameters:**

- `query` (str): The search query string
- `items` (list): List of items to search. Each item can be:
  - A string (id will be index)
  - A dict with 'id' and 'name' keys (or keys specified by 'key' param)
- `max_results` (int, optional): Maximum results to return. Default: 5
- `threshold` (float, optional): Minimum similarity threshold (0.0-1.0). Default: 0.5
- `key` (str, optional): Key to use for item name in dicts. Default: "name"

**Returns:**

- list: List of match dictionaries, each with:
  - `id`: The matched item's ID
  - `name`: The matched item's name
  - `score`: Match score (0.0 to 1.0, higher is better)

**Example:**

```python
import scriptling.fuzzy as fuzzy

# Search list of strings
results = fuzzy.search("proj", ["Project Alpha", "Task Beta", "Project Gamma"])
for r in results:
    print(f"{r['name']}: {r['score']}")

# Search list of dicts
projects = [
    {"id": 1, "name": "Website Redesign"},
    {"id": 2, "name": "Mobile App Development"},
    {"id": 3, "name": "Server Migration"},
]
results = fuzzy.search("web", projects, max_results=3)
# Returns: [{"id": 1, "name": "Website Redesign", "score": 0.9}, ...]

# Search with custom key field
items = [{"id": 1, "title": "My Project"}]
results = fuzzy.search("proj", items, key="title")
```

### `fuzzy.best(query, items, entity_type="item", key="name", threshold=0.5) -> dict`

Finds the best match for a query. If no match is found, returns an error message with suggestions. This is ideal for command-line tools where you want to suggest alternatives when a name is not found.

**Parameters:**

- `query` (str): The search query string
- `items` (list): List of items to search. Each item can be:
  - A string (id will be index)
  - A dict with 'id' and 'name' keys (or keys specified by 'key' param)
- `entity_type` (str, optional): Type name for error messages. Default: "item"
- `key` (str, optional): Key to use for item name in dicts. Default: "name"
- `threshold` (float, optional): Minimum similarity threshold (0.0-1.0). Default: 0.5

**Returns:**

- dict: Dictionary with:
  - `found` (bool): True if a match was found
  - `id` (int or None): The matched item's ID
  - `name` (str or None): The matched item's name
  - `score` (float): Match score (0 if not found)
  - `error` (str or None): Error message with suggestions if not found

**Example:**

```python
import scriptling.fuzzy as fuzzy

projects = [
    {"id": 1, "name": "Website Redesign"},
    {"id": 2, "name": "Mobile App Development"},
    {"id": 3, "name": "Server Migration"},
]

# Exact match (case-insensitive)
result = fuzzy.best("website redesign", projects, entity_type="project")
if result['found']:
    print(f"Found project ID: {result['id']}")
    # Output: Found project ID: 1

# Fuzzy match with error handling
result = fuzzy.best("web design", projects, entity_type="project")
if result['found']:
    print(f"Matched: {result['name']}")
else:
    print(result['error'])
    # Output: project 'web design' is unknown. No similar matches found

# Using in MCP tools for parameter validation
import scriptling.mcp.tool as tool

type_name = tool.get_string("type")
types = [{"id": 1, "name": "Customer"}, {"id": 2, "name": "Lead"}]
match = fuzzy.best(type_name, types, entity_type="customer type")

if not match['found']:
    tool.return_error(match['error'])

type_id = match['id']
```

### `fuzzy.score(s1, s2) -> float`

Calculates the similarity between two strings using normalized Levenshtein distance. Returns a value between 0.0 (completely different) and 1.0 (identical).

**Parameters:**

- `s1` (str): First string
- `s2` (str): Second string

**Returns:**

- float: Similarity score (0.0 to 1.0)

**Example:**

```python
import scriptling.fuzzy as fuzzy

score = fuzzy.score("hello", "hello")  # 1.0 (identical)
score = fuzzy.score("hello", "hallo")  # 0.8 (one character different)
score = fuzzy.score("hello", "xyz")    # ~0.2 (completely different)
```

## Matching Algorithm

The search algorithm uses multiple matching strategies in order of precision:

1. **Exact Match** (score: 1.0) - Case-insensitive exact string match
2. **Substring Match** (score: 0.7-0.9) - Query appears within the item name
3. **Word Boundary Match** (score: 0.85) - Query matches the start of a word
4. **Levenshtein Distance** (score: varies) - Fuzzy matching based on edit distance

Results are sorted by score in descending order, with the best matches appearing first.

## Performance

The fuzzy matching algorithm is optimized for performance:

- Early termination on exact matches
- Efficient Levenshtein distance calculation using two-row optimization
- Configurable threshold to skip very different items

For typical use cases with hundreds of items, searches complete in under 1ms.

## Availability

The fuzzy library is an extended library and must be explicitly imported:

```python
import scriptling.fuzzy as fuzzy
```

It is automatically registered by the scriptling CLI.

## See Also

- [LIBRARIES.md](../LIBRARIES.md) - Complete library reference
- [scriptling.mcp](mcp.md) - MCP tool development
