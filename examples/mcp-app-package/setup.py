# Entry point for the sales-dashboard-app package. serve = ["mcp"] means the
# package is an MCP server only; the tools/, prompts/, skills/ and resources/
# directories are served by convention, so no registration code is needed
# here. Setup scripts run before serving, so this is where auth middleware or
# per-user registration would go.
