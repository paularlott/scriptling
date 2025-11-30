import platform

# Test python_version (returns Scriptling version)
ver = platform.python_version()
assert len(ver) > 0
assert isinstance(ver, "str")

# Test system
sys_name = platform.system()
assert isinstance(sys_name, "str")
assert len(sys_name) > 0
# Should be one of Darwin, Linux, Windows, FreeBSD, etc.
known_systems = ["Darwin", "Linux", "Windows", "FreeBSD", "OpenBSD", "NetBSD"]
# We can't strictly assert it's in the list as it depends on the host, but it's a good check for common ones
if sys_name in known_systems:
    assert True

# Test machine
machine = platform.machine()
assert isinstance(machine, "str")
assert len(machine) > 0

# Test processor
proc = platform.processor()
assert isinstance(proc, "str")
# processor() often returns same as machine() in Go implementation
assert len(proc) > 0

# Test architecture
arch = platform.architecture()
assert isinstance(arch, "list")
assert len(arch) == 2
assert isinstance(arch[0], "str")
assert isinstance(arch[1], "str")
assert "bit" in arch[0]

# Test platform
plat = platform.platform()
assert isinstance(plat, "str")
assert len(plat) > 0
assert "-" in plat

# Test node (hostname)
node = platform.node()
assert isinstance(node, "str")
# node might be empty in some environments, but should be a string

# Test release
rel = platform.release()
assert isinstance(rel, "str")
assert len(rel) > 0

# Test version
ver_sys = platform.version()
assert isinstance(ver_sys, "str")
assert len(ver_sys) > 0

# Test uname
uname = platform.uname()
assert isinstance(uname, "dict")
assert "system" in uname
assert "node" in uname
assert "release" in uname
assert "version" in uname
assert "machine" in uname
assert "processor" in uname

assert uname["system"] == sys_name
assert uname["machine"] == machine
assert uname["processor"] == proc

# Test scriptling specific functions
s_ver = platform.scriptling_version()
assert isinstance(s_ver, "str")
assert len(s_ver) > 0
assert s_ver == ver  # python_version() returns scriptling version for compatibility
