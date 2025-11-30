numbers = [1, 2, 3, 4, 5]
assert numbers[0] == 1
assert numbers[4] == 5
assert len(numbers) == 5

numbers = [1, 2, 3]
numbers.append(4)
assert len(numbers) == 4

numbers = [1, 2, 3, 4, 5]
slice_result = numbers[1:4]
assert len(slice_result) == 3

# Test list methods
lst = [10, 20, 30, 20, 40]
assert lst.index(20) == 1
assert lst.count(20) == 2

lst = [1, 2, 3, 4, 5]
popped = lst.pop()
assert popped == 5
assert len(lst) == 4

popped = lst.pop(0)
assert popped == 1
assert len(lst) == 3

lst = [1, 2, 4, 5]
lst.insert(2, 3)
assert lst[2] == 3

lst = [1, 2, 3, 2, 4]
lst.remove(2)
assert len(lst) == 4
assert lst[1] == 3

lst = [1, 2, 3]
lst.clear()
assert len(lst) == 0

original = [1, 2, 3]
copied = original.copy()
copied.append(4)
assert len(original) == 3
assert copied[3] == 4

# Test list.sort() with key and reverse parameters
nums = [3, 1, 4, 1, 5, 9, 2, 6]
nums.sort()
assert nums == [1, 1, 2, 3, 4, 5, 6, 9]

nums = [3, 1, 4, 1, 5]
nums.sort(reverse=True)
assert nums == [5, 4, 3, 1, 1]

words = ["banana", "apple", "cherry", "date"]
words.sort(key=len)
assert words == ["date", "apple", "banana", "cherry"]

words = ["banana", "apple", "cherry", "date"]
words.sort(key=len, reverse=True)
assert words == ["banana", "cherry", "apple", "date"]

items = [{"name": "Bob", "age": 30}, {"name": "Alice", "age": 25}, {"name": "Charlie", "age": 35}]
items.sort(key=lambda x: x["age"])
assert items[0]["name"] == "Alice"
assert items[2]["name"] == "Charlie"