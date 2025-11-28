# Test hex(), bin(), oct() builtins

# Test hex()
r1 = hex(255) == "0xff"
r2 = hex(16) == "0x10"
r3 = hex(0) == "0x0"
r4 = hex(-255) == "-0xff"
r5 = hex(1000) == "0x3e8"

# Test bin()
r6 = bin(10) == "0b1010"
r7 = bin(255) == "0b11111111"
r8 = bin(0) == "0b0"
r9 = bin(-10) == "-0b1010"
r10 = bin(1) == "0b1"

# Test oct()
r11 = oct(8) == "0o10"
r12 = oct(64) == "0o100"
r13 = oct(0) == "0o0"
r14 = oct(-8) == "-0o10"
r15 = oct(255) == "0o377"

r1 and r2 and r3 and r4 and r5 and r6 and r7 and r8 and r9 and r10 and r11 and r12 and r13 and r14 and r15
