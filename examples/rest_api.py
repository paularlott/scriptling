# REST API Example with Scriptling

# Example 1: Simple GET request with status check
print("=== GET Request ===")
options = {"timeout": 10}
response = http.get("https://jsonplaceholder.typicode.com/todos/1", options)
print("Status: " + str(response["status"]))
print("Body: " + response["body"])

# Example 2: Parse JSON response
print("\n=== Parse JSON ===")
if response["status"] == 200:
    data = json.parse(response["body"])
    print(data)
    print("Title: " + data["title"])

# Example 3: Create JSON and POST
print("\n=== POST Request ===")
new_todo = {"title": "Learn Scriptling", "completed": "false", "userId": "1"}
json_body = json.stringify(new_todo)
print("Sending: " + json_body)

post_response = http.post("https://jsonplaceholder.typicode.com/todos", json_body, options)
print("Status: " + str(post_response["status"]))
if post_response["status"] == 201:
    created = json.parse(post_response["body"])
    print("Created ID: " + str(created["id"]))

# Example 4: String manipulation
print("\n=== String Functions ===")
text = "hello world"
print(upper(text))
print(lower("HELLO WORLD"))
print(replace(text, "world", "scriptling"))

# Example 5: Split and Join
print("\n=== Split and Join ===")
words = split("one,two,three", ",")
print(words)
print(join(words, " - "))

# Example 6: Working with lists
print("\n=== Lists ===")
numbers = [1, 2, 3]
append(numbers, 4)
append(numbers, 5)
print(numbers)
print(len(numbers))

# Example 7: Type conversions
print("\n=== Type Conversions ===")
num_str = "42"
num = int(num_str)
print(num)
print(float("3.14"))
print(str(100))
