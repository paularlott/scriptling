# MCP App Package

A complete MCP application shipped as one package: an MCP Apps view (the
sales dashboard), two plain tools, a prompt with arguments, and a skill, all
served from a single zip with no configuration flags.

Built on the [mcp-app-dashboard](../mcp-app-dashboard/) example; this version
adds the plain tools, the prompt, and the packaging story.

## Layout

```
mcp-app-package/
├── manifest.toml   # serve = ["mcp"]: the package IS an MCP server
├── setup.py        # entry point (nothing to register: dirs are conventional)
├── tools/
│   ├── sales_report.toml + .py   # MCP app tool, [ui] links to the dashboard
│   ├── add_sale.toml + .py       # app-only action tool (dashboard's form)
│   ├── sales_total.toml + .py    # plain tool: sum of all sales
│   └── top_product.toml + .py    # plain tool: biggest sale above a threshold
├── prompts/
│   └── sales_insight.toml + .py  # prompt with a required and an optional argument
├── skills/
│   └── dashboard-ops/            # SKILL.md + references/regions.md
├── resources/
│   └── ui/sales-dashboard/       # the dashboard view (Chart.js)
└── client.py       # verification client (not part of the package)
```

The `tools/`, `resources/`, `prompts/`, `skills/` and `webroot/` directories
are convention directories: present means served, no manifest entries needed.
`client.py` lives outside the conventions, so `pack` warns about it: the
warning is the packer telling you it stayed behind.

## Build and run

From the scriptling repo root:

```bash
# 1. Build the package
scriptling pack -o sales-app.zip examples/mcp-app-package

# 2. Verify everything works from the package (tools, app view, prompt, skill)
scriptling examples/mcp-app-package/client.py

# 3. Serve it over HTTP for real MCP clients
scriptling --server :8080 --package sales-app.zip
#    the MCP endpoint is http://localhost:8080/mcp

# or over stdio (Claude Desktop & friends):
scriptling --package sales-app.zip
```

## What the client proves

- `sales_report` is listed as an app tool (`is_app`) and returns the records;
  its `ui://sales-dashboard/dashboard.html` view is readable.
- `sales_total` and `top_product` work as plain model-facing tools.
- `sales_insight` renders with both arguments, defaults the optional
  `region`, and rejects a missing required argument with `-32602` per the
  MCP spec.
- `dashboard-ops` is listed by `skills/list` and both its `SKILL.md` and its
  supporting `references/regions.md` are readable.

See the [Packaging an MCP App](/tutorials/mcp-app-package/) tutorial for the
full walkthrough of every step.
