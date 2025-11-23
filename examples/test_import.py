import("json")
data = json["parse"]('{"name":"Alice"}')
print(data["name"])