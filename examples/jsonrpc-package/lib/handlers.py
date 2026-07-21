import scriptling.runtime as runtime


def echo(params):
    return params


def add(params):
    return params["a"] + params["b"]


def divide(params):
    if params["b"] == 0:
        return runtime.jsonrpc.error(-32602, "division by zero", {"field": "b"})
    return params["a"] / params["b"]


def on_progress(params):
    pass
