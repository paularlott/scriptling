# Everything in this file is registered with decorators from
# scriptling.runtime.mcp: a tool, two resources (static and template), a
# prompt and a skill. No .toml sidecars, no separate folders.
#
# Type annotations on the signatures are parsed and ignored, so annotated
# Python pastes in unchanged.

import scriptling.runtime.mcp as mcp

PRICES = {"mug": 4.99, "pen": 1.49, "notebook": 6.25}


@mcp.tool("Look up the price of a product by SKU",
          params={"sku": {"type": "string", "description": "Product SKU", "required": True}},
          keywords=["price", "shop"])
def price_lookup(sku: str) -> str:
    return str(PRICES.get(sku, "unknown SKU"))


@mcp.resource("config://shop", name="Shop configuration", mime_type="application/json")
def shop_config() -> dict:
    # Runs on every resources/read, so config changes are picked up live.
    return {"currency": "USD", "items": len(PRICES)}


@mcp.resource("shop://catalog/{category}", template=True, mime_type="text/markdown")
def catalog(category: str) -> str:
    return "# Catalog: " + category + "\n\n- mug\n- pen\n- notebook"


@mcp.prompt(description="Write a product description for a SKU")
def describe(sku: str, tone: str = "friendly") -> str:
    return "Write a " + tone + " one-sentence description for the product with SKU '" + sku + "'."


@mcp.skill(files={"regions.md": "eu: Europe\nus: United States\n"})
def shipping_regions():
    # Called once when the server starts; the return is the SKILL.md,
    # frontmatter included. The frontmatter name must match the function name.
    return """---
name: shipping_regions
description: How to pick the right shipping region for an order
---

# Shipping regions

Read regions.md for the region codes, then confirm the region with the
customer before quoting a delivery estimate.
"""
