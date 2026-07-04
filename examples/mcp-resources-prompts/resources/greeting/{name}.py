import scriptling.mcp.tool as tool

# This file lives at resources/greeting/{name}.py. Because its path contains a
# {var} segment, it is a resource TEMPLATE. Reading greeting://Ada invokes this
# script with the variable "name" set to "Ada".
name = tool.get_string("name")
tool.return_string("Hello, " + name + "! (from a resource template)")
