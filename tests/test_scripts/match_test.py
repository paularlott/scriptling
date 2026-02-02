# Test basic value matching
status = 200
result = ""
match status:
    case 200:
        result = "Success"
    case 404:
        result = "Not found"
    case 500:
        result = "Server error"
    case _:
        result = "Other status"
assert result == "Success"

# Test with different value
status = 404
match status:
    case 200:
        result = "Success"
    case 404:
        result = "Not found"
    case _:
        result = "Other"
assert result == "Not found"

# Test wildcard
status = 999
match status:
    case 200:
        result = "Success"
    case 404:
        result = "Not found"
    case _:
        result = "Unknown"
assert result == "Unknown"

# Test string matching
command = "start"
action = ""
match command:
    case "start":
        action = "starting"
    case "stop":
        action = "stopping"
    case "restart":
        action = "restarting"
    case _:
        action = "unknown"
assert action == "starting"

# Test boolean matching
flag = True
state = ""
match flag:
    case True:
        state = "on"
    case False:
        state = "off"
assert state == "on"

# Test None matching
value = None
check = ""
match value:
    case None:
        check = "null"
    case _:
        check = "not null"
assert check == "null"

# Test type-based matching
data = 42
msg = ""
match data:
    case int():
        msg = "integer"
    case str():
        msg = "string"
    case list():
        msg = "list"
    case _:
        msg = "other"
assert msg == "integer"

# Test with string type
data = "hello"
match data:
    case int():
        msg = "integer"
    case str():
        msg = "string"
    case _:
        msg = "other"
assert msg == "string"

# Test with list type
data = [1, 2, 3]
match data:
    case int():
        msg = "integer"
    case str():
        msg = "string"
    case list():
        msg = "list"
    case _:
        msg = "other"
assert msg == "list"

# Test with dict type
data = {"key": "value"}
match data:
    case dict():
        msg = "dictionary"
    case _:
        msg = "other"
assert msg == "dictionary"

# Test guard clauses
value = 150
category = ""
match value:
    case x if x > 100:
        category = "large"
    case x if x > 50:
        category = "medium"
    case x:
        category = "small"
assert category == "large"

# Test guard with smaller value
value = 75
match value:
    case x if x > 100:
        category = "large"
    case x if x > 50:
        category = "medium"
    case x:
        category = "small"
assert category == "medium"

# Test guard with small value
value = 25
match value:
    case x if x > 100:
        category = "large"
    case x if x > 50:
        category = "medium"
    case x:
        category = "small"
assert category == "small"

# Test structural matching with dict
response = {"status": 200, "data": "payload"}
output = ""
match response:
    case {"status": 200, "data": payload}:
        output = "success: " + payload
    case {"error": msg}:
        output = "error: " + msg
    case _:
        output = "unknown"
assert output == "success: payload"

# Test dict matching with error
response = {"error": "not found"}
match response:
    case {"status": 200, "data": payload}:
        output = "success: " + payload
    case {"error": msg}:
        output = "error: " + msg
    case _:
        output = "unknown"
assert output == "error: not found"

# Test dict matching with wildcard value
response = {"status": 404, "message": "Page not found"}
match response:
    case {"status": 200}:
        output = "ok"
    case {"status": _}:
        output = "has status"
    case _:
        output = "no status"
assert output == "has status"

# Test with capture variable
value = 42
captured = 0
match value:
    case x as num:
        captured = num * 2
assert captured == 84

# Test multiple cases with no match falls through
value = 999
matched = False
match value:
    case 1:
        matched = True
    case 2:
        matched = True
    case 3:
        matched = True
assert matched == False

# Test match with expressions
x = 10
y = 20
result = ""
match x + y:
    case 30:
        result = "thirty"
    case 20:
        result = "twenty"
    case _:
        result = "other"
assert result == "thirty"

# Test nested match (match inside case body)
outer = 1
inner = 2
result = ""
match outer:
    case 1:
        match inner:
            case 2:
                result = "1-2"
            case 3:
                result = "1-3"
    case 2:
        result = "2-x"
assert result == "1-2"

print("All match tests passed!")
