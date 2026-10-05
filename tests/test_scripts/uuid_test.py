import uuid

# Test uuid4 - random UUID (objects now, like Python: str() for the form)
id1 = uuid.uuid4()
assert len(str(id1)) == 36
assert "-" in str(id1)
assert id1.version == 4
assert id1.variant == "specified in RFC 4122"
assert len(id1.hex) == 32

# Verify uuid4 format (version 4)
parts = str(id1).split("-")
assert len(str(id1).split("-")) == 5
assert len(str(id1).split("-")[0]) == 8
assert len(str(id1).split("-")[1]) == 4
assert len(str(id1).split("-")[2]) == 4
assert len(str(id1).split("-")[3]) == 4
assert len(str(id1).split("-")[4]) == 12

# Test uuid1 - time-based UUID
id2 = uuid.uuid1()
assert len(str(id2)) == 36

# Test uuid7 - timestamp-based sortable UUID
id3 = uuid.uuid7()
assert len(str(id3)) == 36

# UUIDs should be unique
assert id1 != id2
assert id2 != id3
assert id1 != id3

# Multiple uuid4 calls should be unique
a = uuid.uuid4()
b = uuid.uuid4()
c = uuid.uuid4()
assert a != b
assert b != c
assert a != c

# Multiple uuid7 calls should be unique and sortable
t1 = uuid.uuid7()
t2 = uuid.uuid7()
assert t1 != t2