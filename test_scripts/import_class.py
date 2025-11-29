
import lib_with_class

g = lib_with_class.Greeter("World")
msg = g.say_hello()
print(msg)
assert msg == "Hello, World"
True
