import json as j
import math as m
import sys as s

data = j.dumps({"pi": m.pi, "argv": s.argv})
print(data)
