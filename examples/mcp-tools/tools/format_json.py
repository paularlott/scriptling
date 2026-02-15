import scriptling.mcp.tool as tool
import json

# Get parameters
json_string = tool.get_string("json_string")
indent = tool.get_int("indent", 2)

# Parse and reformat
try:
    data = json.loads(json_string)
    formatted = json.dumps(data, indent=indent)
    tool.return_string(formatted)
except json.JSONDecodeError as e:
    tool.return_error(f"Invalid JSON: {e}")
