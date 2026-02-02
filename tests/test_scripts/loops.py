# For loop with range
total = 0
for i in range(5):
    total += i
assert total == 10

# For loop with list
count = 0
for fruit in ["apple", "banana", "cherry"]:
    count += 1
assert count == 3

# While loop
count = 0
sum_total = 0
while count < 3:
    sum_total += count
    count += 1
assert sum_total == 3

# Range with start stop
range_sum = 0
for i in range(2, 5):
    range_sum += i
assert range_sum == 9