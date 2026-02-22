# Basic OR pattern
def classify(status):
    match status:
        case 200 | 201 | 204:
            return "success"
        case 400 | 401 | 403:
            return "client_error"
        case 500 | 502 | 503:
            return "server_error"
        case _:
            return "unknown"

assert classify(200) == "success"
assert classify(201) == "success"
assert classify(204) == "success"
assert classify(400) == "client_error"
assert classify(401) == "client_error"
assert classify(500) == "server_error"
assert classify(503) == "server_error"
assert classify(999) == "unknown"

# OR pattern with strings
def day_type(day):
    match day:
        case "Saturday" | "Sunday":
            return "weekend"
        case "Monday" | "Tuesday" | "Wednesday" | "Thursday" | "Friday":
            return "weekday"
        case _:
            return "unknown"

assert day_type("Saturday") == "weekend"
assert day_type("Sunday") == "weekend"
assert day_type("Monday") == "weekday"
assert day_type("Friday") == "weekday"
assert day_type("Holiday") == "unknown"

# OR pattern with guard
def check(x):
    match x:
        case 1 | 2 | 3 if x > 1:
            return "small_gt1"
        case 1 | 2 | 3:
            return "small"
        case _:
            return "other"

assert check(1) == "small"
assert check(2) == "small_gt1"
assert check(3) == "small_gt1"
assert check(10) == "other"

print("All match OR pattern tests passed!")
