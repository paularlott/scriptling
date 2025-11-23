# Scriptling - Minimal Python-like Interpreter Build Plan

## Overview
A minimal Python-like scripting language interpreter written in Go, designed as a library for embedding in other applications.

## Features
- [x] Project structure
- [x] Tokenizer (lexer)
- [x] Parser (AST generation)
- [x] Evaluator (interpreter)
- [x] Built-in functions
- [x] Variable management (set/get from Go)
- [x] Go function registration
- [x] Examples
- [x] Comprehensive tests (lexer, parser, evaluator)
- [x] Lists and list operations
- [x] Dictionaries and dict operations
- [x] For loops
- [x] String functions
- [x] JSON support
- [x] HTTP/REST client functions

## Architecture

### Core Components
1. **Token** (`token/token.go`) - Token types and definitions
2. **Lexer** (`lexer/lexer.go`) - Tokenization
3. **AST** (`ast/ast.go`) - Abstract Syntax Tree nodes
4. **Parser** (`parser/parser.go`) - Parse tokens into AST
5. **Object** (`object/object.go`) - Runtime value types
6. **Evaluator** (`evaluator/evaluator.go`) - Execute AST
7. **Environment** (`environment/environment.go`) - Variable scope management
8. **Scriptling** (`scriptling.go`) - Main library interface

### Language Features
- Variables: `x = 5`
- Arithmetic: `+, -, *, /, %`
- Comparison: `==, !=, <, >, <=, >=`
- Boolean: `and, or, not`
- If statements: `if condition: ... else: ...`
- While loops: `while condition: ...`
- For loops: `for item in list: ...`
- Lists: `[1, 2, 3]`
- Dictionaries: `{"key": "value"}`
- Functions: `def name(args): ...`
- Built-ins: `print()`, `len()`, `str()`, `int()`, `float()`
- String functions: `split()`, `join()`, `upper()`, `lower()`
- JSON: `json_parse()`, `json_stringify()`
- HTTP: `http_get()`, `http_post()`
- Comments: `# comment`

## File Structure
```
scriptling/
├── BUILD_PLAN.md
├── README.md
├── go.mod
├── scriptling.go                    # Main library interface
├── token/
│   └── token.go               # Token definitions
├── lexer/
│   └── lexer.go               # Tokenizer
├── ast/
│   └── ast.go                 # AST node types
├── parser/
│   └── parser.go              # Parser
├── object/
│   └── object.go              # Runtime objects
├── evaluator/
│   └── evaluator.go           # Interpreter
├── environment/
│   └── environment.go         # Variable scope
└── examples/
    ├── basic.scriptling            # Basic example
    ├── functions.scriptling        # Function example
    └── main.go                # Go usage example
```

## Build Progress

### Phase 1: Foundation ✓
- [x] Project structure
- [x] Build plan documentation

### Phase 2: Tokenization ✓
- [x] Token types
- [x] Lexer implementation

### Phase 3: Parsing ✓
- [x] AST nodes
- [x] Parser implementation

### Phase 4: Evaluation ✓
- [x] Object types
- [x] Environment
- [x] Evaluator

### Phase 5: Library Interface ✓
- [x] Main Scriptling API
- [x] Go function registration
- [x] Variable get/set

### Phase 6: Examples & Tests ✓
- [x] Example scripts
- [x] Go usage example
- [x] Basic tests

## API Usage (Planned)

```go
// Create interpreter
p := scriptling.New()

// Register Go function
p.RegisterFunc("add", func(args ...object.Object) object.Object {
    // implementation
})

// Set variable from Go
p.SetVar("x", 10)

// Execute script
result, err := p.Eval("y = x + 5")

// Get variable from Go
y := p.GetVar("y")
```

## Status: ✅ COMPLETE - All Phases Done
Last Updated: Phase 2 complete with 42 passing tests

## Phase 1 Complete ✅
- ✅ Basic arithmetic
- ✅ Variables (set/get from Go)
- ✅ Functions and recursion
- ✅ Conditionals (if/else)
- ✅ While loops
- ✅ Go function registration
- ✅ Example programs running successfully

## Phase 2: Enhanced Features ✅ COMPLETE
### Testing
- [x] Lexer tests (7 test cases)
- [x] Parser tests (7 test cases)
- [x] Evaluator tests (12 test cases)
- [x] Integration tests
- [x] All tests passing

### Data Structures
- [x] Lists implementation `[1, 2, 3]`
- [x] Dictionary implementation `{"key": "value"}`
- [x] For loops `for item in list:`
- [x] Index access `list[0]`, `dict["key"]`, `string[0]`

### Built-in Functions
- [x] Type conversions: `str()`, `int()`, `float()`
- [x] String methods: `split()`, `join()`, `upper()`, `lower()`, `replace()`
- [x] List methods: `append()`
- [x] Enhanced `len()` for lists and dicts

### REST/HTTP Support
- [x] JSON parsing: `json_parse()`, `json_stringify()`
- [x] HTTP client: `http_get()`, `http_post()`, `http_put()`, `http_delete()`, `http_patch()`
- [x] Response objects with status codes and headers
- [x] Configurable timeouts (default 30s)
- [x] Timeout error handling
- [x] Full REST API example

## Summary

### Total Test Coverage
- **42 test cases** across all components
- **100% pass rate**
- Lexer: 7 tests
- Parser: 7 tests  
- Evaluator: 12 tests
- Integration: 10 tests
- Main package: 6 tests

### Language Features Complete
- Variables, functions, recursion
- Arithmetic, comparison, boolean operators
- Control flow: if/else, while, for
- Data structures: lists, dicts
- String manipulation
- Type conversions
- JSON support
- HTTP/REST client

### Examples (using .py extension)
- basic.py - Basic features
- functions.py - Functions and recursion
- collections.py - Lists, dicts, for loops
- rest_api.py - REST API calls with JSON, status codes, timeouts
- main.go - Go integration

### Documentation
- LANGUAGE_GUIDE.md - Complete language reference for LLMs and developers
- QUICK_REFERENCE.md - Quick syntax reference
- README.md - User documentation
- Examples use .py extension for syntax highlighting

## Optional Future Enhancements
- Custom HTTP headers
- Regular expressions
- File I/O operations
- Error handling (try/catch)
- More list/dict methods: pop(), insert(), keys(), values(), items()
- Classes and objects
- Import/module system
