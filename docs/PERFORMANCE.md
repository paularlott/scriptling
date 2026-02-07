# Performance Guide

Tips and best practices for writing efficient Scriptling code.

## String Concatenation

### The Problem

String concatenation with `+=` in loops is slow because each operation creates a new string object:

```python
# ❌ SLOW - Creates 1000 string objects
result = ""
for i in range(1000):
    result += str(i)  # Each += creates a new string
```

**Performance**: 3.45 ms, 25.9 MB for 10,000 iterations

### Solution: Use join()

```python
# ✅ FAST - Efficient and Python-compatible
parts = []
for i in range(1000):
    parts.append(str(i))
result = "".join(parts)
```

**Performance**: ~36 μs for 1000 iterations (70x faster!)

**Why it's fast**: `join()` pre-allocates the exact amount of memory needed and copies all strings once.

### When += is OK

```python
# ✅ OK - Fine for small numbers
result = "hello" + " " + "world"
# or
result = "hello"
result += " "
result += "world"
```

**When to use**:
- Concatenating < 10 strings
- Outside of loops
- Readability matters more

## Examples

### Building CSV

```python
# ❌ SLOW
csv = ""
for row in data:
    csv += ",".join(row) + "\n"

# ✅ FAST - Using join()
lines = []
for row in data:
    lines.append(",".join(row))
csv = "\n".join(lines)
```

### Building HTML

```python
# ❌ SLOW
html = "<ul>"
for item in items:
    html += f"<li>{item}</li>"
html += "</ul>"

# ✅ FAST - Using join()
parts = ["<ul>"]
for item in items:
    parts.append(f"<li>{item}</li>")
parts.append("</ul>")
html = "".join(parts)
```

### Building JSON-like Strings

```python
# ❌ SLOW
json_str = "["
for i, item in enumerate(items):
    if i > 0:
        json_str += ", "
    json_str += f'"{item}"'
json_str += "]"

# ✅ FAST - Using join()
parts = [f'"{item}"' for item in items]
json_str = "[" + ", ".join(parts) + "]"
```

## Recursion vs Iteration

### The Problem

Deep recursion creates many function call frames and environment copies:

```python
# ❌ SLOW - Deep recursion
def fib(n):
    if n <= 1:
        return n
    return fib(n-1) + fib(n-2)

result = fib(10)  # 114 μs, 376 KB, 3,983 allocations
```

### Solution: Use Iteration

```python
# ✅ FAST - Iterative approach
def fib(n):
    if n <= 1:
        return n
    a, b = 0, 1
    for _ in range(n):
        a, b = b, a + b
    return a

result = fib(10)  # Much faster, constant memory
```

### When Recursion is OK

Recursion is fine for:
- Tree/graph traversal (limited depth)
- Divide-and-conquer algorithms
- Naturally recursive problems with small depth

Avoid recursion for:
- Problems with deep recursion (> 100 levels)
- Problems that can be solved iteratively
- Performance-critical code

## General Tips

1. **Profile before optimizing**: Use benchmarks to find real bottlenecks
2. **Readability first**: Optimize only when needed
3. **Use built-in functions**: They're already optimized
4. **Avoid premature optimization**: Write clear code first

## Benchmarking Your Code

```python
import time

# Measure execution time
start = time.time()
# Your code here
end = time.time()
print(f"Took {(end - start) * 1000:.2f} ms")
```

## See Also

- [String Methods](libraries/stdlib/string.md) - Built-in string operations
- [Benchmark Analysis](BENCHMARK_ANALYSIS.md) - Detailed performance analysis
