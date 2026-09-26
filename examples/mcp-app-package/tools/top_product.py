import scriptling.runtime as runtime
import scriptling.mcp.tool as tool

kv = runtime.kv.default

min_amount = tool.get_float("min_amount", 0.0)
records = [kv.get(key) for key in sorted(kv.keys("sale:*"))]
eligible = [r for r in records if r["amount"] >= min_amount]
if not eligible:
    tool.return_error("no sales at or above %s" % min_amount)

top = max(eligible, key=lambda r: r["amount"])
tool.return_string(top["product"] + " (%.2f)" % top["amount"])
