import json

nested_json = '{"user":{"name":"Bob","scores":[10,20,30]}}'
nested = json.loads(nested_json)
nested['user']['name'] == 'Bob' and nested['user']['scores'][0] == 10