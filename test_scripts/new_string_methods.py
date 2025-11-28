# Test new string methods

# Test splitlines() - using a multi-line string with actual newlines
text = """hello
world"""
lines = text.splitlines()
r1 = len(lines) == 2 and lines[0] == 'hello' and lines[1] == 'world'
single_lines = "single".splitlines()
r2 = len(single_lines) == 1 and single_lines[0] == 'single'

# Test swapcase()
r3 = "Hello World".swapcase() == "hELLO wORLD"
r4 = "UPPER".swapcase() == "upper"
r5 = "lower".swapcase() == "LOWER"

# Test partition()
p1 = "hello-world".partition("-")
r6 = p1[0] == 'hello' and p1[1] == '-' and p1[2] == 'world'
p2 = "no sep here".partition("-")
r7 = p2[0] == 'no sep here' and p2[1] == '' and p2[2] == ''
p3 = "a-b-c".partition("-")
r8 = p3[0] == 'a' and p3[1] == '-' and p3[2] == 'b-c'

# Test rpartition()
p4 = "a-b-c".rpartition("-")
r9 = p4[0] == 'a-b' and p4[1] == '-' and p4[2] == 'c'
p5 = "hello".rpartition("-")
r10 = p5[0] == '' and p5[1] == '' and p5[2] == 'hello'
p6 = "one-two-three".rpartition("-")
r11 = p6[0] == 'one-two' and p6[1] == '-' and p6[2] == 'three'

# Test removeprefix()
r12 = "TestCase".removeprefix("Test") == "Case"
r13 = "TestCase".removeprefix("Foo") == "TestCase"
r14 = "Hello".removeprefix("He") == "llo"

# Test removesuffix()
r15 = "MyClass.py".removesuffix(".py") == "MyClass"
r16 = "MyClass.py".removesuffix(".txt") == "MyClass.py"
r17 = "filename.tar.gz".removesuffix(".gz") == "filename.tar"

# Test encode()
e1 = "ABC".encode()
r18 = len(e1) == 3 and e1[0] == 65 and e1[1] == 66 and e1[2] == 67
e2 = "hello".encode()
r19 = len(e2) == 5 and e2[0] == 104 and e2[4] == 111
e3 = "".encode()
r20 = len(e3) == 0

r1 and r2 and r3 and r4 and r5 and r6 and r7 and r8 and r9 and r10 and r11 and r12 and r13 and r14 and r15 and r16 and r17 and r18 and r19 and r20
