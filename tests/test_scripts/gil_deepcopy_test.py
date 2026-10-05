import copy
import scriptling.runtime as runtime

shared = {"items": [1, 2, 3], "n": 0}

def mutator():
    for i in range(5000):
        shared["items"].append(i)
        shared["n"] = i

t = runtime.background("m", "mutator", shared=True)
for i in range(5000):
    copy.deepcopy(shared)
t.wait()
print("ok n =", shared["n"])
