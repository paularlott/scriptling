# toml - TOML Parsing and Generation

Parse and generate TOML (Tom's Obvious Minimal Language) data.

## Import

```python
import toml
```

## Available Functions

| Function             | Description                            |
| -------------------- | -------------------------------------- |
| `loads(toml_string)` | Parse TOML string to Scriptling object |
| `dumps(obj)`         | Convert object to TOML string          |

## Functions

### toml.loads(toml_string)

Parse a TOML string and return the corresponding Scriptling object.

**Parameters:**

- `toml_string` (string): TOML formatted string to parse

**Returns:** Parsed data as Scriptling object (dict, list, string, number, boolean, or None)

**Example:**

```python
import toml

# Parse TOML string
data = toml.loads("""
[database]
host = "localhost"
port = 5432
enabled = true

[servers]
alpha = "10.0.0.1"
beta = "10.0.0.2"
""")

print(data["database"]["host"])  # localhost
print(data["database"]["port"])  # 5432
print(data["servers"]["alpha"])  # 10.0.0.1
```

### toml.dumps(obj)

Convert a Scriptling object to a TOML formatted string.

**Parameters:**

- `obj`: Scriptling object to convert (dict, list, string, number, boolean, or None)

**Returns:** TOML formatted string

**Example:**

```python
import toml

data = {
    "title": "My App",
    "database": {
        "host": "localhost",
        "port": 5432
    },
    "features": ["auth", "logging", "api"]
}

toml_str = toml.dumps(data)
print(toml_str)
# Output:
# title = "My App"
#
# [database]
# host = "localhost"
# port = 5432
#
# features = ["auth", "logging", "api"]
```

## Complete Example

```python
import toml

# Parse TOML configuration
config_toml = """
[server]
host = "0.0.0.0"
port = 8080
workers = 4

[database]
url = "postgres://localhost/mydb"
pool_size = 10
timeout = 30.5

[features]
debug = true
caching = false

[[servers]]
name = "web1"
ip = "10.0.0.1"

[[servers]]
name = "web2"
ip = "10.0.0.2"
"""

config = toml.loads(config_toml)

# Access parsed data
print("Server host:", config["server"]["host"])
print("Server port:", config["server"]["port"])
print("Database URL:", config["database"]["url"])
print("Debug mode:", config["features"]["debug"])

# Access array of tables
for server in config["servers"]:
    print(f"Server {server['name']}: {server['ip']}")

# Modify and generate TOML
config["server"]["workers"] = 8
config["features"]["caching"] = true

updated_toml = toml.dumps(config)
print("\nUpdated configuration:")
print(updated_toml)
```

## Supported Types

| TOML Type   | Scriptling Type |
| ----------- | --------------- |
| String      | String          |
| Integer     | Integer         |
| Float       | Float           |
| Boolean     | Boolean         |
| Array       | List            |
| Table       | Dict            |
| Inline Table| Dict            |
| Datetime    | String (ISO format) |

## Differences from Python's tomllib

**Similarities:**

- ✅ `loads()` function for parsing TOML strings
- ✅ Handles all standard TOML 1.0 types
- ✅ Python-compatible API

**Differences:**

- ❌ No `load()` for file reading (only string input)
- ➕ `dumps()` for writing TOML (Python's tomllib is read-only)
- ❌ No file I/O (only string input/output)
- ⚠️ Datetime values returned as strings rather than datetime objects

## Differences from tomli/tomli-w

**Similarities:**

- ✅ `loads()` for parsing
- ✅ `dumps()` for writing

**Differences:**

- ❌ No `load()` / `dump()` for file operations
- ❌ No ` TOMLDecodeError` exception class
- ❌ Datetime values as strings instead of datetime objects

## Use Cases

- Configuration files (pyproject.toml, Cargo.toml)
- Build system configuration
- Package metadata
- Application settings
- Infrastructure configuration
