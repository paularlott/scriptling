import scriptling.runtime as runtime
import scriptling.mcp.tool as tool

kv = runtime.kv.default

records = [kv.get(key) for key in sorted(kv.keys("sale:*"))]
tool.return_string("total: %.2f" % sum(r["amount"] for r in records))
