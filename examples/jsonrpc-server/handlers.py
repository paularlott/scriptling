import scriptling.runtime as runtime


# echo: return the params unchanged. Demonstrates a basic request/response.
def echo(params):
    return params


# add: pull fields off a dict params and return a scalar result.
def add(params):
    return params["a"] + params["b"]


# divide: return a structured JSON-RPC error for bad input via the
# runtime.jsonrpc.error() helper, which carries a custom code and message.
def divide(params):
    if params["b"] == 0:
        return runtime.jsonrpc.error(-32602, "division by zero", {"field": "b"})
    return params["a"] / params["b"]


# on_progress: a notification handler. It receives params like any method but
# no response is written. Return values are ignored.
def on_progress(params):
    # Side effects only (e.g. update a store, log, push to a panel).
    pass
