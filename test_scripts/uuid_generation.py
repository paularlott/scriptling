# Test uuid library
import uuid

# Test uuid4 - random UUID
id1 = uuid.uuid4()
len(id1) == 36  # UUID format: 8-4-4-4-12 = 36 chars with hyphens
"-" in id1

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
a != b
