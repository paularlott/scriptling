# Sequence patterns
def describe(val):
    match val:
        case []:
            return "empty"
        case [x]:
            return f"single {x}"
        case [x, y]:
            return f"pair {x} {y}"
        case [x, y, z]:
            return f"triple {x} {y} {z}"
        case _:
            return "longer"

assert describe([]) == "empty"
assert describe([1]) == "single 1"
assert describe([1, 2]) == "pair 1 2"
assert describe([1, 2, 3]) == "triple 1 2 3"
assert describe([1, 2, 3, 4]) == "longer"

# Sequence pattern with guard
def classify_pair(pair):
    match pair:
        case [x, y] if x == y:
            return "equal"
        case [x, y] if x > y:
            return "descending"
        case [x, y]:
            return "ascending"
        case _:
            return "not a pair"

assert classify_pair([3, 3]) == "equal"
assert classify_pair([5, 2]) == "descending"
assert classify_pair([1, 4]) == "ascending"
assert classify_pair([1]) == "not a pair"

# OR patterns
def http_status(code):
    match code:
        case 200 | 201 | 204:
            return "success"
        case 301 | 302:
            return "redirect"
        case 400 | 401 | 403 | 404:
            return "client error"
        case 500 | 502 | 503:
            return "server error"
        case _:
            return "unknown"

assert http_status(200) == "success"
assert http_status(201) == "success"
assert http_status(301) == "redirect"
assert http_status(404) == "client error"
assert http_status(500) == "server error"
assert http_status(999) == "unknown"

# Dict structural matching
def handle_response(resp):
    match resp:
        case {"status": 200, "data": data}:
            return f"ok: {data}"
        case {"status": 404}:
            return "not found"
        case {"error": msg}:
            return f"error: {msg}"
        case _:
            return "unknown"

assert handle_response({"status": 200, "data": "payload"}) == "ok: payload"
assert handle_response({"status": 404}) == "not found"
assert handle_response({"error": "timeout"}) == "error: timeout"

# Capture variable
def extract(val):
    match val:
        case x if isinstance(x, int) and x > 0:
            return f"positive int {x}"
        case x if isinstance(x, str):
            return f"string {x}"
        case _:
            return "other"

assert extract(42) == "positive int 42"
assert extract("hello") == "string hello"
assert extract(-1) == "other"
