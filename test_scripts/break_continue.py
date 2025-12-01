failures = 0

# Break in for
total = 0
for i in range(10):
    if i == 5:
        break
    total += i
if total != 10:
    failures += 1

# Continue in for
total = 0
for i in range(10):
    if i % 2 == 0:
        continue
    total += i
if total != 25:
    failures += 1

# Break in while
count = 0
total = 0
while True:
    if count >= 3:
        break
    total += count
    count += 1
if total != 3:
    failures += 1

failures == 0