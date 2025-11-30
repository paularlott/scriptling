# Test os and os.path libraries
import os
import os.path
import platform

passed = True

# os.getenv
home = os.getenv("HOME", "/default")
if home == "":
    home = "/default"
passed = passed and len(home) > 0

# os.getcwd
cwd = os.getcwd()
passed = passed and len(cwd) > 0

# os constants
passed = passed and os.name in ["posix", "nt"]
passed = passed and os.sep in ["/", "\\"]

# os.path functions
path = "/usr/local/bin/python3"
passed = passed and os.path.dirname(path) == "/usr/local/bin"
passed = passed and os.path.basename(path) == "python3"

parts = os.path.split(path)
passed = passed and parts[0] == "/usr/local/bin"
passed = passed and parts[1] == "python3"

result = os.path.splitext("/home/user/file.txt")
passed = passed and result[0] == "/home/user/file"
passed = passed and result[1] == ".txt"

joined = os.path.join("/home", "user", "docs")
passed = passed and (joined == "/home/user/docs" or joined == "/home\\user\\docs")

# platform module
system = platform.system()
passed = passed and system in ["Darwin", "Linux", "Windows", "FreeBSD"]
machine = platform.machine()
passed = passed and len(machine) > 0
version = platform.scriptling_version()
passed = passed and len(version) > 0

assert passed
