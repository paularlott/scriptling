# Console Library

The `sl.console` library provides console input/output functions for interactive scripts.

## Import

```python
import sl.console as console
```

## Functions

### `console.input([prompt]) -> str`

Read a line from standard input. If a `prompt` is provided, it is written to stdout without a trailing newline before reading input.

**Parameters:**
- `prompt` (str, optional): The prompt to display before reading input

**Returns:**
- str: The line read from input, without the trailing newline

**Raises:**
- Error: If input is not available in the current environment

**Example:**

```python
import sl.console as console

# Simple input
name = console.input("Enter your name: ")
print("Hello, " + name + "!")

# Input without prompt
line = console.input()
print("You entered: " + line)
```

## Notes

- The `console.input()` function is only available when running scripts through the scriptling CLI with access to stdin
- When stdin is closed (e.g., when piping input), the function will raise an error
- The returned string does not include the trailing newline character
- For scripts that don't require interactive input, consider using command-line arguments instead

## Availability

The console library is an extended library and must be explicitly imported:

```python
import sl.console as console
```

It is automatically registered by the scriptling CLI with stdin access configured.
