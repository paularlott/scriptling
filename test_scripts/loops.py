# For loop with range
sum = 0
for i in range(5):
    sum += i
assert sum == 10

# For loop with list
count = 0
for fruit in ["apple", "banana", "cherry"]:
    count += 1
assert count == 3

# While loop
count = 0
total = 0
while count < 3:
    total += count
    count += 1
assert total == 3

# Range with start stop
sum = 0
for i in range(2, 5):
    sum += i
assert sum == 9