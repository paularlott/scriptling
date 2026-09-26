import scriptling.runtime as runtime
import scriptling.mcp.tool as tool

kv = runtime.kv.default

date = tool.get_string("date")
product = tool.get_string("product")
amount = tool.get_float("amount")

# kv.incr() is atomic, so concurrent add_sale calls each get a distinct
# sequence number and never collide or overwrite one another's record.
seq = kv.incr("sale_seq")
kv.set("sale:%06d" % seq, {"date": date, "product": product, "amount": amount})

records = [kv.get(key) for key in sorted(kv.keys("sale:*"))]
tool.return_structured({"records": records})
