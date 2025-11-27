import json

json_array = '[1,2,3,4,5]'
arr = json.loads(json_array)
len(arr) == 5