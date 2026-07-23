import scriptling.mcp.tool as tool

expr = tool.get_string("expr")

# Safe-ish arithmetic: only digits, operators, dots, spaces and parens.
allowed = "0123456789+-*/.() "
if not all(c in allowed for c in expr):
    tool.return_error("invalid characters in expression")

try:
    result = eval(expr)
except:
    tool.return_error("could not evaluate expression")
else:
    tool.return_string(f"{expr} = {result}")
