import scriptling.runtime as runtime
import scriptling.mcp.tool as tool

kv = runtime.kv.default

SEED_RECORDS = [
    {"date": "2026-09-01", "product": "Widget", "amount": 120.50},
    {"date": "2026-09-03", "product": "Gadget", "amount": 75.00},
    {"date": "2026-09-05", "product": "Widget", "amount": 45.25},
    {"date": "2026-09-10", "product": "Gizmo", "amount": 210.00},
]

# Each sale is its own KV entry ("sale:000001", "sale:000002", ...) rather
# than one shared list value: add_sale.py only ever writes a brand-new key
# (numbered via kv.incr(), which is atomic), so concurrent tool calls can't
# race on a get-modify-set of a single shared list and silently lose an
# update. Sorting the keys preserves insertion order.
if not kv.keys("sale:*"):
    for record in SEED_RECORDS:
        seq = kv.incr("sale_seq")
        kv.set("sale:%06d" % seq, record)

records = [kv.get(key) for key in sorted(kv.keys("sale:*"))]
tool.return_structured({"records": records})
