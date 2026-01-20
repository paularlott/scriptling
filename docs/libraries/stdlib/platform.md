# Platform Library

The `platform` library provides access to underlying platform's identifying data.

## Import

```python
import platform
```

## Functions

### `architecture()`
Returns a tuple `(bits, linkage)` containing information about the bit architecture and the linkage format.

```python
print(platform.architecture())
# Output: ('64bit', '')
```

### `machine()`
Returns the machine type, e.g. 'i386'. An empty string is returned if the value cannot be determined.

```python
print(platform.machine())
# Output: 'arm64'
```

### `node()`
Returns the computer's network name (may not be fully qualified!). An empty string is returned if the value cannot be determined.

```python
print(platform.node())
# Output: 'my-computer.local'
```

### `platform(aliased=0, terse=0)`
Returns a single string identifying the underlying platform with as much useful information as possible.

```python
print(platform.platform())
# Output: 'Darwin-21.6.0-arm64-arm-64bit'
```

### `processor()`
Returns the (real) processor name, e.g. 'amdk6'.

```python
print(platform.processor())
# Output: 'arm'
```

### `python_version()`
Returns the Python version as string 'major.minor.patchlevel'.
**Note:** In Scriptling, this returns the Scriptling version for compatibility.

```python
print(platform.python_version())
# Output: '1.0.0'
```

### `release()`
Returns the system's release, e.g. '2.2.0' or 'NT'.

```python
print(platform.release())
# Output: '21.6.0'
```

### `scriptling_version()`
Returns the Scriptling version as string.

```python
print(platform.scriptling_version())
# Output: '1.0.0'
```

### `system()`
Returns the system/OS name, e.g. 'Linux', 'Windows', or 'Java'. An empty string is returned if the value cannot be determined.

```python
print(platform.system())
# Output: 'Darwin'
```

### `uname()`
Returns a dictionary containing system information.
**Note:** Unlike Python which returns a `namedtuple`, Scriptling returns a dictionary with keys: `system`, `node`, `release`, `version`, `machine`, `processor`.

```python
info = platform.uname()
print(info["system"])
# Output: 'Darwin'
```

### `version()`
Returns the system's release version, e.g. '#3 on degas'.

```python
print(platform.version())
# Output: 'Darwin Kernel Version 21.6.0...'
```
