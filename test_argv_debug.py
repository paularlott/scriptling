import sys

print("Full argv:", sys.argv)
print("Length:", len(sys.argv))
for i, arg in enumerate(sys.argv):
    print(f"argv[{i}] = {arg}")
