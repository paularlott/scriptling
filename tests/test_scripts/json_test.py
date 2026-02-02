import json

json_array = '[1,2,3,4,5]'
arr = json.loads(json_array)
assert len(arr) == 5

obj = {"status": "success", "count": 42}
result = json.dumps(obj)
assert len(result) > 10

json_str = '{"name":"Alice","age":30,"active":true}'
data = json.loads(json_str)
assert len(data) == 3

nested_json = '{"user":{"name":"Bob","scores":[10,20,30]}}'
nested = json.loads(nested_json)
assert nested['user']['name'] == 'Bob'
assert nested['user']['scores'][0] == 10

# Test dot notation access
assert nested.user.name == 'Bob'
assert nested.user.scores[0] == 10
