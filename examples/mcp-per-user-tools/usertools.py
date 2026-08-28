import scriptling.mcp.tool as tool
import scriptling.runtime.mcp as mcp

# Per-user tool implementations, referenced by "usertools.<function>" from the
# middleware. They run on a fresh interpreter per call; tool.request_context()
# tells them who is calling, and mcp.transport() how they are being served.


def alpha(note):
    user = tool.request_context().get("user", "anonymous")
    return {"user": user, "note": note, "transport": mcp.transport()}


def beta(x):
    return "beta:" + x


def gamma():
    return "gamma has no parameters"


def notes(__uri, topic):
    user = tool.request_context().get("user", "anonymous")
    return "note:" + topic + ":" + user


def report(subject):
    user = tool.request_context().get("user", "anonymous")
    return {
        "messages": [
            {"role": "user", "content": "Write a report on " + subject},
            {"role": "assistant", "content": "Preparing it for " + user},
        ]
    }
