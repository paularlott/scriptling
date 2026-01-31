#!/usr/bin/env scriptling
"""
o2s - OpenAPI to Scriptling

Converts OpenAPI v3 specifications into pure Scriptling HTTP client libraries.

Usage:
  scriptling o2s.py -- <spec_file> [--list]
  scriptling o2s.py -- <spec_file> --generate [options]
  ./o2s.py <spec_file> [options]  (if executable)
"""

import json
import re
import sys

try:
    import os
except:
    pass

try:
    import yaml
except:
    pass

def parse_args():
    """Parse command line arguments"""
    args = {"mode": "list", "spec": None, "filter": None, "output": None}
    
    # First positional arg is spec file
    if len(sys.argv) > 1 and not sys.argv[1].startswith("--"):
        args["spec"] = sys.argv[1]
        i = 2
    else:
        i = 1
    
    while i < len(sys.argv):
        arg = sys.argv[i]
        if arg == "--filter" and i + 1 < len(sys.argv):
            args["filter"] = sys.argv[i + 1]
            i += 2
        elif arg == "--output" and i + 1 < len(sys.argv):
            args["output"] = sys.argv[i + 1]
            i += 2
        elif arg == "--generate":
            args["mode"] = "generate"
            i += 1
        elif arg == "--list":
            args["mode"] = "list"
            i += 1
        elif arg == "--help" or arg == "-h":
            print_usage()
            sys.exit(0)
        else:
            i += 1
    return args

def print_usage():
    """Print usage information"""
    print("o2s - OpenAPI to Scriptling")
    print()
    print("Usage:")
    print("  scriptling o2s.py -- <spec_file> [--list]")
    print("  scriptling o2s.py -- <spec_file> --generate [--filter <file>] [--output <base>]")
    print("  ./o2s.py <spec_file> [options]  (if executable)")
    print()
    print("Note: Use '--' separator when running with scriptling CLI")
    print()
    print("Modes:")
    print("  --list              List all endpoints (default)")
    print("  --generate          Generate Scriptling library")
    print()
    print("Options:")
    print("  --filter <file>     File with endpoints to include (one per line)")
    print("  --output <base>     Output file base (default: api_client)")
    print("                      Generates <base>.py and <base>.md")
    print()
    print("Examples:")
    print("  # List all endpoints")
    print("  scriptling o2s.py -- api.json")
    print()
    print("  # Generate library (creates api_client.py and api_client.md)")
    print("  scriptling o2s.py -- api.json --generate")
    print()
    print("  # Generate with custom name (creates petstore.py and petstore.md)")
    print("  scriptling o2s.py -- api.json --generate --output petstore")

def load_spec(filepath):
    """Load OpenAPI spec from JSON or YAML file with $ref support"""
    content = ""
    try:
        content = os.read_file(filepath)
    except:
        print("Error: Cannot read file: " + filepath)
        sys.exit(1)

    # Try JSON first
    spec = None
    try:
        spec = json.parse(content)
    except:
        pass
    
    # Try YAML
    if spec is None:
        try:
            spec = yaml.safe_load(content)
        except:
            print("Error: Could not parse file as JSON or YAML")
            print("Make sure the file is valid JSON or YAML format")
            sys.exit(1)
    
    # Resolve $ref entries
    spec = resolve_refs(spec, filepath)
    return spec

def resolve_refs(spec, base_file):
    """Resolve $ref entries in spec"""
    if "paths" not in spec:
        return spec
    
    # Get base directory
    base_dir = "."
    if "/" in base_file:
        parts = base_file.split("/")
        base_dir = "/".join(parts[:-1])
    
    paths = spec["paths"]
    merged_paths = {}
    
    # First, add all inline paths (non-$ref entries)
    for key in list(paths.keys()):
        if key != "$ref":
            merged_paths[key] = paths[key]
    
    # Then, if there's a $ref, load and merge those paths
    if "$ref" in paths:
        ref_value = paths["$ref"]
        ref_parts = ref_value.split("#")
        ext_file = base_dir + "/" + ref_parts[0]
        
        try:
            ext_content = os.read_file(ext_file)
            try:
                ext_spec = json.parse(ext_content)
            except:
                ext_spec = yaml.safe_load(ext_content)
            
            if "paths" in ext_spec:
                # Merge external paths (don't overwrite inline ones)
                for ext_path, ext_item in ext_spec["paths"].items():
                    if ext_path not in merged_paths:
                        merged_paths[ext_path] = ext_item
        except:
            pass
    
    spec["paths"] = merged_paths
    return spec

def load_filter(filepath):
    """Load endpoint filter list"""
    if not filepath:
        return None

    try:
        content = os.read_file(filepath)

        endpoints = []
        for line in content.split("\n"):
            line = line.strip()
            if line and not line.startswith("#"):
                endpoints.append(line)
        return endpoints
    except:
        print("Error: Cannot read filter file: " + filepath)
        sys.exit(1)

def extract_endpoints(spec):
    """Extract all endpoints from OpenAPI spec"""
    endpoints = []

    if "paths" not in spec:
        return endpoints

    paths = spec["paths"]
    for path in sorted(list(paths.keys())):
        path_item = paths[path]
        methods = ["get", "post", "put", "patch", "delete", "head", "options"]

        for method in methods:
            if method in path_item:
                endpoints.append({
                    "method": method.upper(),
                    "path": path,
                    "operation": path_item[method]
                })

    return endpoints

def filter_endpoints(endpoints, filter_list):
    """Filter endpoints based on filter list"""
    if not filter_list:
        return endpoints

    filtered = []
    for endpoint in endpoints:
        endpoint_str = endpoint["method"] + " " + endpoint["path"]
        if endpoint_str in filter_list:
            filtered.append(endpoint)

    return filtered

def list_endpoints(endpoints):
    """Print list of endpoints"""
    for endpoint in endpoints:
        print(endpoint["method"] + " " + endpoint["path"])

def sanitize_name(name):
    """Convert path/name to valid Python identifier"""
    # Remove leading/trailing slashes
    name = name.strip("/")
    # Replace special chars with underscore
    name = re.sub(r"[^a-zA-Z0-9_]", "_", name)
    # Remove consecutive underscores
    name = re.sub(r"_+", "_", name)
    # Remove leading/trailing underscores
    name = name.strip("_")
    # Ensure doesn't start with number
    if name and len(name) > 0:
        first_char = name[0]
        if first_char in "0123456789":
            name = "n" + name
    return name if name else "endpoint"

def get_operation_id(endpoint):
    """Get or generate operation ID"""
    operation = endpoint["operation"]
    if "operationId" in operation:
        return sanitize_name(operation["operationId"])

    # Generate from method and path
    method = endpoint["method"].lower()
    path = sanitize_name(endpoint["path"])
    return method + "_" + path

def resolve_component_ref(spec, ref_path, base_file, cache=None):
    """Resolve a component $ref (internal or external file)"""
    if cache is None:
        cache = {}
    
    # External file reference (e.g., ./components.yaml#/parameters/PetId)
    if not ref_path.startswith("#/"):
        parts = ref_path.split("#")
        ext_file = parts[0]
        
        # Make path relative to base file
        if "/" in base_file:
            base_dir = "/".join(base_file.split("/")[:-1])
            ext_file = base_dir + "/" + ext_file
        
        # Load external file (with caching)
        if ext_file not in cache:
            try:
                content = os.read_file(ext_file)
                try:
                    cache[ext_file] = json.parse(content)
                except:
                    cache[ext_file] = yaml.safe_load(content)
            except:
                return None
        
        ext_spec = cache[ext_file]
        
        # If there's a fragment (#/path/to/component), resolve it
        if len(parts) > 1 and parts[1]:
            fragment = parts[1].lstrip("/")  # Remove leading slash
            ref_path = "#/" + fragment
            return resolve_internal_ref(ext_spec, ref_path)
        
        return ext_spec
    
    # Internal reference
    return resolve_internal_ref(spec, ref_path)

def resolve_internal_ref(spec, ref_path):
    """Resolve internal $ref like #/components/parameters/PetIdParam"""
    if not ref_path.startswith("#/"):
        return None
    
    parts = ref_path[2:].split("/")  # Remove #/ and split
    current = spec
    
    for part in parts:
        if part in current:
            current = current[part]
        else:
            return None
    
    return current

def get_parameters(operation, spec, base_file):
    """Extract parameters from operation"""
    params = {"path": [], "query": [], "header": []}
    cache = {}

    if "parameters" not in operation:
        return params

    for param in operation["parameters"]:
        # Resolve $ref if present
        if "$ref" in param:
            resolved = resolve_component_ref(spec, param["$ref"], base_file, cache)
            if resolved:
                param = resolved
        
        if "in" in param and "name" in param:
            location = param["in"]
            if location in params:
                params[location].append(param)

    return params

def get_request_body(operation, spec, base_file):
    """Extract request body schema"""
    if "requestBody" not in operation:
        return None

    body = operation["requestBody"]
    
    # Resolve $ref if present
    if "$ref" in body:
        cache = {}
        resolved = resolve_component_ref(spec, body["$ref"], base_file, cache)
        if resolved:
            body = resolved
    
    if "content" not in body:
        return None

    content = body["content"]
    # Prefer JSON
    if "application/json" in content:
        return {"type": "json", "schema": content["application/json"]}

    # Fallback to first content type
    for content_type in list(content.keys()):
        return {"type": content_type, "schema": content[content_type]}

    return None

def generate_function(endpoint, spec, base_file):
    """Generate Scriptling method for endpoint"""
    op_id = get_operation_id(endpoint)
    method = endpoint["method"]
    path = endpoint["path"]
    operation = endpoint["operation"]

    params = get_parameters(operation, spec, base_file)
    body = get_request_body(operation, spec, base_file)

    # Build function signature with self
    args = ["self"]

    # Path parameters (required)
    for param in params["path"]:
        args.append(sanitize_name(param["name"]))

    # Query parameters (optional with defaults)
    for param in params["query"]:
        required = param.get("required", False)
        param_name = sanitize_name(param["name"])
        if required:
            args.append(param_name)
        else:
            args.append(param_name + "=None")

    # Body parameter
    if body:
        args.append("body=None")

    # Headers
    if params["header"]:
        args.append("headers=None")

    func_sig = "    def " + op_id + "(" + ", ".join(args) + "):"

    # Build function body
    lines = []
    lines.append('        """')

    # Add description
    if "summary" in operation:
        lines.append("        " + operation["summary"])
    elif "description" in operation:
        desc = operation["description"].split("\n")[0]
        lines.append("        " + desc)
    else:
        lines.append("        " + method + " " + path)

    lines.append('        """')

    # Build URL
    lines.append('        url = self.base_url + "' + path + '"')

    # Replace path parameters
    if params["path"]:
        for param in params["path"]:
            name = param["name"]
            param_name = sanitize_name(name)
            lines.append('        url = url.replace("{' + name + '}", str(' + param_name + '))')

    # Build query parameters
    if params["query"]:
        lines.append('        query_params = {}')
        for param in params["query"]:
            name = param["name"]
            param_name = sanitize_name(name)
            lines.append('        if ' + param_name + ' is not None:')
            lines.append('            query_params["' + name + '"] = ' + param_name)

    # Build headers
    lines.append('        req_headers = self.headers.copy()')
    if params["header"]:
        lines.append('        if headers:')
        lines.append('            req_headers.update(headers)')

    # Build request
    lines.append('        ')
    lines.append('        options = {')
    lines.append('            "method": "' + method + '",')
    lines.append('            "headers": req_headers')

    if params["query"]:
        lines.append('        }')
        lines.append('        if query_params:')
        lines.append('            options["params"] = query_params')

    if body:
        if not params["query"]:
            lines.append('        }')
        lines.append('        if body is not None:')
        lines.append('            options["json"] = body')

    if not params["query"] and not body:
        lines.append('        }')

    lines.append('        ')
    lines.append('        return self._request(url, options)')

    return func_sig + "\n" + "\n".join(lines)

def generate_library(spec, endpoints, base_file):
    """Generate complete Scriptling library with class-based client"""
    info = spec.get("info", {})
    title = info.get("title", "API")
    version = info.get("version", "1.0.0")
    description = info.get("description", "")

    # Get base URL from servers
    default_base_url = ""
    if "servers" in spec and len(spec["servers"]) > 0:
        default_base_url = spec["servers"][0].get("url", "")

    lines = []
    lines.append('"""')
    lines.append(title + " Client Library")
    lines.append("")
    lines.append("Version: " + version)
    if description:
        lines.append("")
        lines.append(description)
    lines.append('"""')
    lines.append("")
    lines.append("import requests")
    lines.append("import json")
    lines.append("")
    lines.append("class APIClient:")
    lines.append('    """API Client for ' + title + '"""')
    lines.append("    ")
    lines.append('    def __init__(self, base_url="' + default_base_url + '", auth_token=None):')
    lines.append('        """')
    lines.append('        Initialize API client')
    lines.append('        ')
    lines.append('        Args:')
    lines.append('            base_url: Base URL for API requests')
    lines.append('            auth_token: Optional authentication token')
    lines.append('        """')
    lines.append('        self.base_url = base_url')
    lines.append('        self.headers = {')
    lines.append('            "Content-Type": "application/json",')
    lines.append('            "Accept": "application/json"')
    lines.append('        }')
    lines.append('        if auth_token:')
    lines.append('            self.headers["Authorization"] = "Bearer " + auth_token')
    lines.append('    ')
    lines.append('    def set_auth_token(self, token):')
    lines.append('        """Set authentication token"""')
    lines.append('        self.headers["Authorization"] = "Bearer " + token')
    lines.append('    ')
    lines.append('    def set_header(self, key, value):')
    lines.append('        """Set a custom header"""')
    lines.append('        self.headers[key] = value')
    lines.append('    ')
    lines.append('    def _request(self, url, options):')
    lines.append('        """Internal request handler"""')
    lines.append('        try:')
    lines.append('            method = options["method"]')
    lines.append('            headers = options["headers"]')
    lines.append('            params = options.get("params")')
    lines.append('            json_body = options.get("json")')
    lines.append('            ')
    lines.append('            if method == "GET":')
    lines.append('                response = requests.get(url=url, headers=headers, params=params)')
    lines.append('            elif method == "POST":')
    lines.append('                response = requests.post(url=url, headers=headers, json=json_body)')
    lines.append('            elif method == "PUT":')
    lines.append('                response = requests.put(url=url, headers=headers, json=json_body)')
    lines.append('            elif method == "PATCH":')
    lines.append('                response = requests.patch(url=url, headers=headers, json=json_body)')
    lines.append('            elif method == "DELETE":')
    lines.append('                response = requests.delete(url=url, headers=headers)')
    lines.append('            elif method == "HEAD":')
    lines.append('                response = requests.head(url=url, headers=headers)')
    lines.append('            elif method == "OPTIONS":')
    lines.append('                response = requests.options(url=url, headers=headers)')
    lines.append('            ')
    lines.append('            # Parse JSON response body')
    lines.append('            body = response["body"]')
    lines.append('            if response["headers"].get("Content-Type", "").startswith("application/json"):')
    lines.append('                body = json.loads(body)')
    lines.append('            return {"status": response["status_code"], "body": body, "headers": response["headers"]}')
    lines.append('        except Exception as e:')
    lines.append('            return {"error": str(e)}')
    lines.append('    ')
    lines.append('    # API Endpoints')
    lines.append('    ')

    for endpoint in endpoints:
        lines.append(generate_function(endpoint, spec, base_file))
        lines.append('    ')

    return "\n".join(lines)

def generate_readme(spec, endpoints, base_file):
    """Generate README documentation"""
    info = spec.get("info", {})
    title = info.get("title", "API")
    version = info.get("version", "1.0.0")
    description = info.get("description", "")

    lines = []
    lines.append("# " + title + " Client Library")
    lines.append("")
    lines.append("Version: " + version)
    lines.append("")
    if description:
        lines.append(description)
        lines.append("")

    lines.append("## Installation")
    lines.append("")
    lines.append("Import the library in your Scriptling code:")
    lines.append("")
    lines.append("```python")
    lines.append("import api_client")
    lines.append("```")
    lines.append("")

    lines.append("## Configuration")
    lines.append("")
    lines.append("```python")
    lines.append("import api_client")
    lines.append("")
    lines.append("# Create client instance")
    lines.append('client = api_client.APIClient("https://api.example.com", "your-token-here")')
    lines.append("")
    lines.append("# Or configure after creation")
    lines.append('client = api_client.APIClient("https://api.example.com")')
    lines.append('client.set_auth_token("your-token-here")')
    lines.append('client.set_header("X-Custom-Header", "value")')
    lines.append("")
    lines.append("# Multiple environments")
    lines.append('prod = api_client.APIClient("https://prod.example.com", "prod-token")')
    lines.append('dev = api_client.APIClient("https://dev.example.com", "dev-token")')
    lines.append("```")
    lines.append("")

    lines.append("## Available Endpoints")
    lines.append("")

    for endpoint in endpoints:
        op_id = get_operation_id(endpoint)
        method = endpoint["method"]
        path = endpoint["path"]
        operation = endpoint["operation"]

        lines.append("### " + op_id)
        lines.append("")
        lines.append("`" + method + " " + path + "`")
        lines.append("")

        if "summary" in operation:
            lines.append(operation["summary"])
            lines.append("")

        # Parameters
        params = get_parameters(operation, spec, base_file)
        body = get_request_body(operation, spec, base_file)

        if params["path"] or params["query"] or body:
            lines.append("**Parameters:**")
            lines.append("")

            for param in params["path"]:
                name = param["name"]
                desc = param.get("description", "")
                lines.append("- `" + name + "` (path, required): " + desc)

            for param in params["query"]:
                name = param["name"]
                required = "required" if param.get("required", False) else "optional"
                desc = param.get("description", "")
                lines.append("- `" + name + "` (query, " + required + "): " + desc)

            if body:
                lines.append("- `body` (body, optional): Request body")

            lines.append("")

        # Example
        lines.append("**Example:**")
        lines.append("")
        lines.append("```python")

        # Build example call
        example_args = []
        for param in params["path"]:
            example_args.append('"value"')

        if body:
            example_args.append('body={"key": "value"}')

        lines.append("response = client." + op_id + "(" + ", ".join(example_args) + ")")
        lines.append('print(response["body"])')
        lines.append("```")
        lines.append("")

    return "\n".join(lines)

def main():
    """Main entry point"""
    args = parse_args()

    if not args["spec"]:
        print("Error: spec file is required")
        print()
        print_usage()
        sys.exit(1)

    # Load spec
    spec = load_spec(args["spec"])

    # Extract endpoints
    endpoints = extract_endpoints(spec)

    # Load filter if provided
    filter_list = load_filter(args["filter"])

    # Filter endpoints
    endpoints = filter_endpoints(endpoints, filter_list)

    if len(endpoints) == 0:
        print("No endpoints found")
        sys.exit(0)

    # Execute mode
    if args["mode"] == "list":
        list_endpoints(endpoints)
    elif args["mode"] == "generate":
        # Generate library
        library = generate_library(spec, endpoints, args["spec"])
        readme = generate_readme(spec, endpoints, args["spec"])

        # Determine output file base
        output_base = args["output"] if args["output"] else "api_client"

        # Write files
        lib_file = output_base + ".py"
        readme_file = output_base + ".md"

        try:
            os.write_file(lib_file, library)
            print("Generated: " + lib_file)
        except:
            print("Error: Cannot write to " + lib_file)
            sys.exit(1)

        try:
            os.write_file(readme_file, readme)
            print("Generated: " + readme_file)
        except:
            print("Error: Cannot write to " + readme_file)
            sys.exit(1)

main()
