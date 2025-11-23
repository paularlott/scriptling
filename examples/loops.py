# Loop examples
print("For loop with list:")
for item in [1, 2, 3, 4, 5]:
    print("Item:", item)

print("\nFor loop with range-like behavior:")
numbers = []
i = 0
while i < 5:
    numbers = append(numbers, i)
    i = i + 1

for num in numbers:
    print("Number:", num)

print("\nWhile loop countdown:")
count = 5
while count > 0:
    print("Countdown:", count)
    count = count - 1
print("Done!")