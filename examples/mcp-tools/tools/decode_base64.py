import scriptling.mcp.tool as tool
import base64

# Get parameters
encoded = tool.get_string("encoded")

# Decode from base64
try:
    decoded = base64.b64decode(encoded).decode()
    tool.return_string(decoded)
except Exception as e:
    tool.return_error(f"Failed to decode base64: {e}")
