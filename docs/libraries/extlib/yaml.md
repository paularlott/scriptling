# yaml - YAML Parsing and Generation

Parse and generate YAML (YAML Ain't Markup Language) data.

## Import

```python
import yaml
```

## Functions

### yaml.safe_load(yaml_string)

Parse a YAML string and return the corresponding Scriptling object.

**Parameters:**
- `yaml_string` (string): YAML formatted string to parse

**Returns:** Parsed data as Scriptling object (dict, list, string, number, boolean, or None)

**Example:**

```python
import yaml

# Parse YAML string
data = yaml.safe_load("""
name: John Doe
age: 30
active: true
tags:
  - python
  - yaml
""")

print(data["name"])  # John Doe
print(data["age"])   # 30
print(data["tags"])  # ["python", "yaml"]
```

### yaml.load(yaml_string)

Alias for `safe_load()`. Both functions are safe and identical in Scriptling.

**Note:** In PyYAML, `load()` is deprecated in favor of `safe_load()`. Scriptling provides both for compatibility, but they behave identically.

### yaml.safe_dump(obj)

Convert a Scriptling object to a YAML formatted string.

**Parameters:**
- `obj`: Scriptling object to convert (dict, list, string, number, boolean, or None)

**Returns:** YAML formatted string

**Example:**

```python
import yaml

data = {
    "name": "Jane Smith",
    "age": 25,
    "skills": ["Go", "Python", "JavaScript"],
    "active": true
}

yaml_str = yaml.safe_dump(data)
print(yaml_str)
# Output:
# active: true
# age: 25
# name: Jane Smith
# skills:
# - Go
# - Python
# - JavaScript
```

### yaml.dump(obj)

Alias for `safe_dump()`. Both functions are identical in Scriptling.

## Complete Example

```python
import yaml

# Parse YAML configuration
config_yaml = """
database:
  host: localhost
  port: 5432
  credentials:
    username: admin
    password: secret

features:
  - authentication
  - logging
  - caching

debug: false
"""

config = yaml.safe_load(config_yaml)

# Access parsed data
print("Database host:", config["database"]["host"])
print("Database port:", config["database"]["port"])
print("Features:", config["features"])
print("Debug mode:", config["debug"])

# Modify and generate YAML
config["debug"] = true
config["features"].append("monitoring")

updated_yaml = yaml.safe_dump(config)
print("\nUpdated configuration:")
print(updated_yaml)
```

## Supported Types

| YAML Type | Scriptling Type |
|-----------|----------------|
| String    | String         |
| Integer   | Integer        |
| Float     | Float          |
| Boolean   | Boolean        |
| Null      | None           |
| Sequence  | List           |
| Mapping   | Dict           |

## Differences from PyYAML

**Similarities:**
- ✅ `safe_load()` and `safe_dump()` functions
- ✅ `load()` and `dump()` aliases
- ✅ Handles all standard YAML types
- ✅ Python-compatible API

**Differences:**
- ❌ No `load_all()` / `dump_all()` for multiple documents
- ❌ No file I/O (only string input/output)
- ❌ No custom constructors or representers
- ❌ No `Loader` / `Dumper` class parameters
- ❌ No YAML tags or anchors support
- ⚠️ `load()` and `safe_load()` are identical (both safe)

## Use Cases

- Configuration files
- API specifications (OpenAPI/Swagger)
- Data serialization
- Infrastructure as Code (Kubernetes, Docker Compose)
- CI/CD pipelines (GitHub Actions, GitLab CI)
