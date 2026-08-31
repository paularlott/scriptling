import scriptling.runtime.mcp as mcp

# Demo credentials: a real deployment would validate tokens against the KV
# store, an API, or anything else the middleware can reach.
TOKENS = {
    "Bearer alice-key": "alice",
    "Bearer bob-key": "bob",
    "Bearer carol-key": "carol",
}


def check(request):
    user = TOKENS.get(request.header("authorization", ""))
    if user is None:
        return {"status": 401, "body": "unauthorized"}

    # Everything after this point can rely on request.context["user"].
    request.context["user"] = user

    if user == "alice":
        mcp.register_request_tool(
            "alpha_tool",
            handler="usertools.alpha",
            description="Alice's private tool",
            params={"note": "A note to echo back"},
        )

    if user == "bob":
        mcp.register_request_tool(
            "beta_tool",
            handler="usertools.beta",
            description="Bob's first tool",
            params={"x": {"type": "string", "description": "Text to echo", "required": True}},
        )
        mcp.register_request_tool(
            "gamma_tool",
            handler="usertools.gamma",
            description="Bob's second tool (no parameters)",
        )
        mcp.register_request_resource(
            "user://bob/notes/{topic}",
            handler="usertools.notes",
            name="Bob's notes",
            description="Notes only Bob can read",
            template=True,
        )
        mcp.register_request_prompt(
            "bob_report",
            handler="usertools.report",
            description="Build a report for Bob",
            arguments=[{"name": "subject", "description": "Report subject", "required": True}],
        )

    # carol is authenticated but gets no extra entries: the static tools only.

    return None
