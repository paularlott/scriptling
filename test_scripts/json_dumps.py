import json

obj = {"status": "success", "count": 42}
result = json.dumps(obj)
len(result) > 10