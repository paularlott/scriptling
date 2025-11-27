import json

json_str = '{"name":"Alice","age":30,"active":true}'
data = json.loads(json_str)
len(data) == 3