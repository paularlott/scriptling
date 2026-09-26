import scriptling.mcp.tool as tool

# A tiny in-memory catalogue standing in for a real backend.
PRICES = {
    "mug": {"price": 4.99, "currency": "USD", "in_stock": True},
    "pen": {"price": 1.49, "currency": "USD", "in_stock": True},
    "notebook": {"price": 6.25, "currency": "USD", "in_stock": False},
}

sku = tool.get_string("sku").lower()

if sku in PRICES:
    item = PRICES[sku]
    tool.return_object({
        "sku": sku,
        "price": item["price"],
        "currency": item["currency"],
        "in_stock": item["in_stock"],
    })
else:
    tool.return_error("unknown SKU: " + sku)
