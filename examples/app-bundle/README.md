# App Bundle Example

A complete app shipped as a single package: HTTP routes, MCP tools and static
assets together.

## Layout

```
app-bundle/
├── manifest.toml     # serve = ["http", "mcp"]
├── setup.py          # registers HTTP routes (the main entry point)
├── lib/
│   ├── handlers.py   # HTTP handler functions
│   └── utils.py      # shared module (demonstrates libs dir imports)
├── tools/
│   ├── calc.toml     # MCP tool metadata
│   └── calc.py       # MCP tool script
├── webroot/
│   ├── index.html    # static assets
│   └── style.css
└── docs/
    └── guide.md
```

## Run from a folder (development)

```bash
# HTTP + MCP over HTTP
scriptling --server :8000 --package examples/app-bundle

# MCP over stdio (for MCP clients)
scriptling --package examples/app-bundle
```

## Run from a zip (production)

```bash
# Build the zip
scriptling pack examples/app-bundle app-bundle.zip

# Serve it
scriptling --server :8000 --package app-bundle.zip
# or from a URL:
scriptling --server :8000 --package https://example.com/app-bundle.zip#sha256=...
```

## What you get

- `GET /` — static page from `webroot/index.html`
- `GET /style.css` — static asset from `webroot/`
- `GET /api/time` — script handler from `lib/handlers.py`
- `POST /api/echo` — script handler that echoes the request body
- MCP tool `calc` — evaluates a math expression (available at `/mcp` over HTTP
  or via stdio)

## Key points

- **One artifact**: everything ships in the folder (or zip). No `--script`,
  `--mcp-tools` or `--web-root` flags needed — the manifest declares it all.
- **Dev = prod**: the folder and the zip run the same code path.
- **Shared code**: `lib/utils.py` is imported by both HTTP handlers and could
  be imported by MCP tool scripts too.
