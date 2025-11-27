failures = 0

# For loop with range
sum = 0
for i in range(5):
    sum += i
if sum != 10:
    failures += 1

# For loop with list
count = 0
for fruit in ["apple", "banana", "cherry"]:
    count += 1
if count != 3:
    failures += 1

# While loop
count = 0
total = 0
while count < 3:
    total += count
    count += 1
if total != 3:
    failures += 1

# Range with start stop
sum = 0
for i in range(2, 5):
    sum += i
if sum != 9:
    failures += 1

failures == 0