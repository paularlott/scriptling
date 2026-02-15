import scriptling.mcp.tool as tool

# Get parameters
name = tool.get_string("name")
greeting = tool.get_string("greeting", "Hello")
enthusiasm = tool.get_int("enthusiasm", 1)

# Clamp enthusiasm to valid range
if enthusiasm < 0:
    enthusiasm = 0
if enthusiasm > 5:
    enthusiasm = 5

# Build the message
exclamation = "!" * enthusiasm
message = f"{greeting}, {name}{exclamation}"

tool.return_string(message)
