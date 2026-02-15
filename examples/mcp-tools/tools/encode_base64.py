import scriptling.mcp.tool as tool
import base64

# Get parameters
text = tool.get_string("text")

# Encode to base64
encoded = base64.b64encode(text.encode()).decode()

tool.return_string(encoded)
