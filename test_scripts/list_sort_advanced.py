# Test list.sort() with key and reverse parameters

# Basic sort
nums = [3, 1, 4, 1, 5, 9, 2, 6]
nums.sort()
assert nums == [1, 1, 2, 3, 4, 5, 6, 9], "basic sort"

# Sort with reverse
nums = [3, 1, 4, 1, 5]
nums.sort(reverse=True)
assert nums == [5, 4, 3, 1, 1], "sort with reverse=True"

# Sort with key function
words = ["banana", "apple", "cherry", "date"]
words.sort(key=len)
assert words == ["date", "apple", "banana", "cherry"], "sort by length"

# Sort with both key and reverse
words = ["banana", "apple", "cherry", "date"]
words.sort(key=len, reverse=True)
assert words == ["banana", "cherry", "apple", "date"], "sort by length descending"

# Sort with lambda
items = [{"name": "Bob", "age": 30}, {"name": "Alice", "age": 25}, {"name": "Charlie", "age": 35}]
items.sort(key=lambda x: x["age"])
assert items[0]["name"] == "Alice", "sort by age"
assert items[2]["name"] == "Charlie", "sort by age - last"

print("All list.sort() tests passed!")

# Return true for test framework
True
