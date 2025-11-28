# Test list methods
failures = 0

# index
lst = [10, 20, 30, 20, 40]
if lst.index(20) != 1:
    failures += 1

# count
if lst.count(20) != 2:
    failures += 1

# pop
lst = [1, 2, 3, 4, 5]
popped = lst.pop()
if popped != 5:
    failures += 1
if len(lst) != 4:
    failures += 1

popped = lst.pop(0)
if popped != 1:
    failures += 1
if len(lst) != 3:
    failures += 1

# insert
lst = [1, 2, 4, 5]
lst.insert(2, 3)
if lst[2] != 3:
    failures += 1

# remove
lst = [1, 2, 3, 2, 4]
lst.remove(2)
if len(lst) != 4:
    failures += 1
if lst[1] != 3:
    failures += 1

# clear
lst = [1, 2, 3]
lst.clear()
if len(lst) != 0:
    failures += 1

# copy
original = [1, 2, 3]
copied = original.copy()
copied.append(4)
if len(original) != 3:
    failures += 1
if len(copied) != 4:
    failures += 1

# reverse
lst = [1, 2, 3, 4, 5]
lst.reverse()
if lst[0] != 5:
    failures += 1
if lst[4] != 1:
    failures += 1

failures == 0
