failures = 0

def sum_all(*args):
    total = 0
    for num in args:
        total += num
    return total

if sum_all(1, 2, 3) != 6:
    failures += 1
if sum_all() != 0:
    failures += 1

def prefix_print(prefix, *args):
    result = []
    for item in args:
        result.append(prefix + str(item))
    return result

prefixed = prefix_print(">", 1, 2)
if len(prefixed) != 2 or prefixed[0] != ">1":
    failures += 1

failures == 0