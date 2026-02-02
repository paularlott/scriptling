failures = 0

# Range basic
count = 0
for i in range(5):
    count += 1
if count != 5:
    failures += 1

# Range start stop
total = 0
for i in range(2, 7):
    total += i
if total != 20:
    failures += 1

# List slicing
numbers = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]
slice1 = numbers[2:5]
if len(slice1) != 3 or slice1[0] != 2 or slice1[2] != 4:
    failures += 1

slice2 = numbers[0:3]
if len(slice2) != 3 or slice2[0] != 0 or slice2[2] != 2:
    failures += 1

# String slicing
text = "Hello, World!"
part = text[0:5]
if len(part) != 5 or part[0] != "H" or part[4] != "o":
    failures += 1

part2 = text[7:12]
if len(part2) != 5 or part2[0] != "W" or part2[4] != "d":
    failures += 1

failures == 0