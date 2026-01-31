# o2s - OpenAPI to Scriptling

Convert OpenAPI v3 specifications into pure Scriptling HTTP client libraries.

**Written entirely in Scriptling** - 620 lines of pure Scriptling code, no Go dependencies.

## Quick Start

```bash
# List endpoints
scriptling o2s.py -- api.json

# Generate library (creates api_client.py and api_client.md)
scriptling o2s.py -- api.json --generate

# Generate with custom name (creates petstore.py and petstore.md)
scriptling o2s.py -- api.json --generate --output petstore
```

## Usage

```bash
scriptling o2s.py -- <spec_file> [options]
```

**Options:**
- `--list` - List all endpoints (default)
- `--generate` - Generate Scriptling library
- `--filter <file>` - Filter endpoints (one per line: `METHOD /path`)
- `--output <base>` - Output file base (default: api_client)
  - Generates `<base>.py` and `<base>.md`
- `--max-params <n>` - Max parameters before using data dict (default: 5)
  - Below threshold: Individual named parameters
  - At/above threshold: Single `data=None` parameter

**Note:** Use `--` separator when running with scriptling CLI.

## Examples

```bash
# List all endpoints
scriptling o2s.py -- examples/petstore.json

# Generate library (creates api_client.py and api_client.md)
scriptling o2s.py -- examples/petstore.json --generate

# Generate with custom name (creates petstore.py and petstore.md)
scriptling o2s.py -- examples/petstore.json --generate --output petstore

# Use data dict for methods with 3+ parameters
scriptling o2s.py -- examples/petstore.json --generate --max-params 3

# Generate filtered library
scriptling o2s.py -- examples/petstore.json --generate \
  --filter examples/filter.txt --output petstore
```

## Generated Library

The tool generates two files:
- `<base>.py` - Class-based HTTP client with methods for each endpoint
- `<base>.md` - Complete documentation with examples

**Generated library features:**
- Class-based client (supports multiple environments)
- Constructor: `APIClient(base_url, auth_token=None)`
- Methods: `set_auth_token()`, `set_header()`
- Type-safe methods for each endpoint
- Automatic parameter handling (path, query, headers, body)
- Clean API matching OpenAPI operation IDs

## Using Generated Libraries

### Individual Parameters (< threshold)

```python
import api_client

client = api_client.APIClient("https://api.example.com", "your-token")

# Methods with few parameters use named arguments
response = client.get_user("user123")  # 1 param
users = client.list_users(limit=10, offset=0)  # 2 params
```

### Data Dictionary (>= threshold)

```python
# Methods with many parameters use data dict
response = client.complex_operation(data={
    "user_id": "123",
    "filter": "active",
    "sort": "name",
    "limit": 10,
    "offset": 0
})
```

### Multiple Environments

```python
# No conflicts between environments
prod = api_client.APIClient("https://prod.example.com", "prod-token")
dev = api_client.APIClient("https://dev.example.com", "dev-token")

prod_users = prod.get_users()
dev_users = dev.get_users()
```

## Filter File Format

Simple text file with one endpoint per line:

```
GET /api/users
POST /api/users
GET /api/users/{id}
```

Lines starting with `#` are comments. Blank lines are ignored.

## Features

- ✅ OpenAPI v3 support (JSON and YAML)
- ✅ Generates pure Scriptling code
- ✅ Class-based client (multi-environment support)
- ✅ Endpoint filtering
- ✅ Auto-generated documentation
- ✅ Fast (95 endpoints in ~20ms)
- ✅ Zero Go dependencies

## Requirements

- Scriptling with `requests`, `json`, `yaml`, `re`, `sys`, `os` libraries
- OpenAPI v3 specification (JSON or YAML)

## License

MIT
