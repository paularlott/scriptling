# Test uuid library - comprehensive
import uuid

# Test uuid4 - random UUID
id1 = uuid.uuid4()
len(id1) == 36
"-" in id1

# Verify uuid4 format (version 4)
parts = id1.split("-")
len(parts) == 5
len(parts[0]) == 8
len(parts[1]) == 4
len(parts[2]) == 4
len(parts[3]) == 4
len(parts[4]) == 12

# Test uuid1 - time-based UUID
id2 = uuid.uuid1()
len(id2) == 36

# Test uuid7 - timestamp-based sortable UUID
id3 = uuid.uuid7()
len(id3) == 36

# UUIDs should be unique
id1 != id2
id2 != id3
id1 != id3

# Multiple uuid4 calls should be unique
a = uuid.uuid4()
b = uuid.uuid4()
c = uuid.uuid4()
a != b
b != c
a != c

# Multiple uuid7 calls should be unique and sortable
t1 = uuid.uuid7()
t2 = uuid.uuid7()
t1 != t2
