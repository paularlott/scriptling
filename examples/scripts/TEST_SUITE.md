# Scriptling Test Suite

This directory contains comprehensive tests for all Scriptling language features.

## Test Coverage

### Core Language Features (test_basics.py)
- Variables and assignments
- Arithmetic operations (+, -, *, /, %)
- Comparison operators (>, <, ==, !=, >=, <=)
- Boolean operators (and, or, not)
- String concatenation
- Print statements

### Functions (test_functions.py)
- Function definition and calls
- Parameters and return values
- Recursive functions
- Nested function calls
- Functions without return (implicit None)

### Collections (test_collections.py)
- Lists: creation, indexing, length, append
- Dictionaries: creation, access, membership
- Tuples: creation, indexing, unpacking
- List slicing
- Nested structures

### Control Flow (test_control_flow.py)
- if/else statements
- elif chains (test_elif.py)
- Nested conditionals
- pass statement
- Multiple conditions

### Loops (test_loops.py)
- for loops with range
- for loops with lists
- while loops
- Nested loops
- break and continue statements (test_break_continue.py)

### Operators
- Membership: in, not in (test_operators_membership.py)
- Augmented assignment: +=, -=, *=, /= (test_operators_augmented.py)

### Scope
- Global variables (test_scope_global.py)
- Nonlocal variables (test_scope_nonlocal.py)
- Combined global/nonlocal (test_scope_combined.py)

### Error Handling
- try/except/finally (test_error_handling.py)
- Exception with 'as' clause
- raise statement
- Nested try/except
- HTTP error handling (test_error_http.py)
- Comprehensive error tests (test_error_comprehensive.py)

### Advanced Features
- Multiple assignment/tuple unpacking (test_multiple_assignment.py)
- List comprehensions (test_list_comprehensions.py)
- Lambda functions (test_lambda.py)
- Default parameters (test_default_params.py)
- String methods (test_string_methods.py)
- List methods - append, extend (test_append.py)
- Range and slicing (test_range_slice.py)

### Standard Libraries
- json: parse, stringify (test_lib_json.py)
- math: sqrt, pow, abs, floor, ceil, round, min, max, sin, cos, tan, log, exp, degrees, radians, fmod, gcd, factorial, pi, e (test_lib_math.py)
- base64: encode, decode (test_lib_base64.py)
- hashlib: md5, sha1, sha256 (test_lib_hashlib.py)
- random: random, randint, choice (test_lib_random.py)
- url: encode, decode, parse, build, join, query_parse, urlsplit, urlunsplit, parse_qs, urlencode, path_join (test_lib_url.py)
- re (regex): match, find, search, findall, replace, split, compile, escape, fullmatch (test_lib_regex.py)
- import mechanism (test_lib_import.py)

### HTTP/Requests Library
- HTTP methods: GET, POST, PUT, DELETE, PATCH (test_lib_http.py)
- Response attributes: text, status_code, headers (test_requests_api.py)
- Response methods: json(), raise_for_status() (test_requests_methods.py)
- Exception handling: HTTPError, RequestException
- Timeout and headers options

### Examples
- variables.py: Variable operations demonstration
- fibonacci.py: Fibonacci sequence generator

## Running Tests

Run all tests:
```bash
./run_all_tests.sh
```

Run a specific test:
```bash
go run main.go test_basics.py
```

## Test Status

**All 38 tests passing ✓**

The test suite validates that Scriptling provides a comprehensive Python-compatible scripting environment suitable for:
- LLM code generation
- Embedded scripting
- Configuration scripts
- Data processing
- HTTP API interactions
- Text processing and manipulation
