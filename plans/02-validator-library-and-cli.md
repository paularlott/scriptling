# Plan: Phase 2 - Validator Library & CLI Integration

**Status:** Planning
**Priority:** High
**Dependencies:** Phase 1 (Typed Kwargs Validation)
**Estimated Complexity:** Medium-High

## Overview

Build a comprehensive validator library that can be used both as a standalone Go package and integrated into the `scriptling-cli` tool. The validator will check Scriptling code for syntax errors, structural issues, and provide detailed feedback including line numbers.

## Goals

1. Create a reusable validator library in Go
2. Integrate validation into `scriptling-cli` as a `validate` command
3. Support multiple validation levels (syntax, structure, types - optional)
4. Provide clear, actionable error messages with line numbers
5. Enable validation of generated libraries (for Phase 3)

## Architecture

### Components

```
validator/
├── validator.go          # Main validator interface and orchestration
├── syntax.go             # Syntax validation (lexer/parser level)
├── structure.go          # Structural validation (AST level)
├── types.go              # Type validation (optional, Phase 2.5)
├── errors.go             # Validation error types and formatting
└── reporter.go           # Output formatting (human, JSON, etc.)

scriptling-cli/
├── cmd_validate.go       # CLI command for validation
└── integration changes
```

## Files to Create

### 1. `validator/validator.go` (NEW)

**Purpose:** Main validator interface and orchestration

```go
package validator

import (
    "io"
    "github.com/paularlott/scriptling/object"
)

// ValidationLevel defines the depth of validation
type ValidationLevel int

const (
    SyntaxOnly    ValidationLevel = iota // Lexical and syntax errors only
    StructureCheck                        // Syntax + structure validation
    FullValidation                        // Syntax + structure + types (future)
)

// Validator handles validation of Scriptling code
type Validator struct {
    level    ValidationLevel
    reporter Reporter
}

// NewValidator creates a new validator with specified level
func NewValidator(level ValidationLevel) *Validator

// Validate validates Scriptling code from a reader
func (v *Validator) Validate(filename string, source io.Reader) *ValidationResult

// ValidateString validates Scriptling code from a string
func (v *Validator) ValidateString(filename, source string) *ValidationResult

// ValidationReport contains the results of validation
type ValidationReport struct {
    Filename  string
    Valid     bool
    Errors    []ValidationError
    Warnings  []ValidationWarning
    Duration  time.Duration
}

// ValidationError represents a validation error
type ValidationError struct {
    Line      int
    Column    int
    Severity  ErrorSeverity
    Category  ErrorCategory
    Message   string
    Context   string  // Line of code where error occurred
    Suggestion string // How to fix it (optional)
}

type ErrorSeverity int
const (
    Error   ErrorSeverity = iota
    Warning
    Info
)

type ErrorCategory string
const (
    Syntax      ErrorCategory = "syntax"
    Structure   ErrorCategory = "structure"
    Type        ErrorCategory = "type"
    Import      ErrorCategory = "import"
    Runtime     ErrorCategory = "runtime"
)
```

### 2. `validator/syntax.go` (NEW)

**Purpose:** Validate syntax using existing lexer/parser

```go
// SyntaxValidator checks for lexical and syntax errors
type SyntaxValidator struct {
    lexer  *lexer.Lexer
    parser *parser.Parser
}

// Validate performs syntax validation
func (sv *SyntaxValidator) Validate(source string) []ValidationError {
    errors := []ValidationError{}

    // Tokenize and check for lexical errors
    lexer := lexer.New(source)
    tokens := []lexer.Token{}
    for {
        token := lexer.NextToken()
        tokens = append(tokens, token)
        if token.Type == lexer.EOF {
            break
        }
        // Check for illegal tokens, unknown characters
        if token.Type == lexer.ILLEGAL {
            errors = append(errors, ValidationError{
                Line:     token.Line,
                Column:   token.Column,
                Severity: Error,
                Category: Syntax,
                Message:  fmt.Sprintf("Illegal token: %s", token.Literal),
                Context:  getLineContext(source, token.Line),
            })
        }
    }

    // Parse and check for syntax errors
    parser := parser.New(tokens)
    program := parser.ParseProgram()

    if len(parser.Errors()) > 0 {
        for _, err := range parser.Errors() {
            errors = append(errors, ValidationError{
                Line:      err.Line,
                Column:    err.Column,
                Severity:  Error,
                Category:  Syntax,
                Message:   err.Message,
                Context:   getLineContext(source, err.Line),
            })
        }
    }

    return errors
}
```

### 3. `validator/structure.go` (NEW)

**Purpose:** Validate AST structure (imports, function signatures, etc.)

```go
// StructureValidator checks AST structural issues
type StructureValidator struct{}

// Validate performs structural validation
func (sv *StructureValidator) Validate(ast *ast.Program) []ValidationError {
    errors := []ValidationError{}

    // Check for undefined imports
    errors = append(errors, sv.checkImports(ast)...)

    // Check for duplicate definitions
    errors = append(errors, sv.checkDuplicates(ast)...)

    // Check function signatures
    errors = append(errors, sv.checkFunctionSignatures(ast)...)

    // Check class definitions
    errors = append(errors, sv.checkClassDefinitions(ast)...)

    return errors
}

func (sv *StructureValidator) checkImports(ast *ast.Program) []ValidationError {
    // Validate import statements
    // Check for circular imports (if possible)
    // Validate library names
}

func (sv *StructureValidator) checkDuplicates(ast *ast.Program) []ValidationError {
    // Check for duplicate function names
    // Check for duplicate class names
    // Check for duplicate variable declarations
}

func (sv *StructureValidator) checkFunctionSignatures(ast *ast.Program) []ValidationError {
    // Check for duplicate parameter names
    // Check default value syntax
    // Check *args and **kwargs placement
}
```

### 4. `validator/types.go` (NEW - OPTIONAL/PHASE 2.5)

**Purpose:** Type checking (optional enhancement)

**Note:** This is complex and can be deferred to Phase 2.5 or 3

```go
// TypeValidator performs type inference and checking
type TypeValidator struct {
    env *object.Environment
}

// Validate performs type validation
func (tv *TypeValidator) Validate(ast *ast.Program) []ValidationError {
    // Build symbol table
    // Infer types
    // Check type mismatches
    // Check function call signatures
    // Check operation types (e.g., adding string to int)
}
```

### 5. `validator/errors.go` (NEW)

**Purpose:** Error types and formatting

```go
// NewSyntaxError creates a syntax validation error
func NewSyntaxError(line, col int, message, context string) ValidationError

// NewStructureError creates a structure validation error
func NewStructureError(line, col int, message, context string) ValidationError

// FormatError formats a validation error for display
func FormatError(err ValidationError) string

// FormatJSON formats validation report as JSON
func FormatJSON(report *ValidationReport) (string, error)
```

### 6. `validator/reporter.go` (NEW)

**Purpose:** Output formatting and reporting

```go
// Reporter formats and outputs validation results
type Reporter interface {
    Report(report *ValidationReport) error
}

// ConsoleReporter outputs to terminal with colors
type ConsoleReporter struct {
    colorful bool
    verbose  bool
}

func NewConsoleReporter(colorful, verbose bool) *ConsoleReporter
func (cr *ConsoleReporter) Report(report *ValidationReport) error

// JSONReporter outputs machine-readable JSON
type JSONReporter struct {
    pretty bool
}

func NewJSONReporter(pretty bool) *JSONReporter
func (jr *JSONReporter) Report(report *ValidationReport) error
```

### 7. `validator/validator_test.go` (NEW)

**Purpose:** Comprehensive tests for validator

```go
func TestValidator_ValidCode(t *testing.T)
func TestValidator_SyntaxError(t *testing.T)
func TestValidator_UndefinedImport(t *testing.T)
func TestValidator_DuplicateDefinitions(t *testing.T)
func TestValidator_InvalidFunctionSignature(t *testing.T)
func TestValidator_ClassValidation(t *testing.T)
func TestValidator_MultipleErrors(t *testing.T)
func TestValidator_LargeFile(t *testing.T) // Performance test
```

## Files to Modify

### 8. `scriptling-cli/main.go` (MODIFY)

**Add validation command:**

```go
// Add to command registration
commands["validate"] = &cli.Command{
    Name:        "validate",
    Description: "Validate Scriptling files",
    Usage:       "validate [options] <file...>",
    Action:      runValidate,
}

func runValidate(ctx *cli.Context) error {
    // Implementation in cmd_validate.go
}
```

### 9. `scriptling-cli/cmd_validate.go` (NEW)

**Purpose:** CLI command for validation

```go
package main

import (
    "fmt"
    "os"
    "github.com/paularlott/scriptling/validator"
)

type ValidateCmd struct {
    Level      string  // "syntax", "structure", "full"
    Format     string  // "text", "json"
    Verbose    bool
    WarningsAsErrors bool
}

func (cmd *ValidateCmd) Run(args []string) error {
    // Parse validation level
    level := parseLevel(cmd.Level)

    // Create validator
    v := validator.NewValidator(level)

    // Validate each file
    reports := []*validator.ValidationReport{}
    for _, file := range args {
        source, err := os.ReadFile(file)
        if err != nil {
            return err
        }

        report := v.ValidateString(file, string(source))
        reports = append(reports, report)
    }

    // Output results
    reporter := newReporter(cmd.Format, cmd.Verbose)
    for _, report := range reports {
        if err := reporter.Report(report); err != nil {
            return err
        }
    }

    // Exit with error if any validation failed
    for _, report := range reports {
        if !report.Valid {
            os.Exit(1)
        }
    }

    return nil
}
```

### 10. `validator/go.mod` (NEW - if separate module)

**Purpose:** Dependencies for validator package

```go
module github.com/paularlott/scriptling/validator

go 1.21

require (
    github.com/paularlott/scriptling v0.0.0
)
```

## Implementation Steps

### Phase 2.1: Core Validator (MVP)

**Week 1:**
- [ ] Create `validator/` package structure
- [ ] Implement `validator.go` with basic interface
- [ ] Implement `syntax.go` using existing lexer/parser
- [ ] Implement `errors.go` with error types
- [ ] Implement basic `ConsoleReporter`
- [ ] Write tests for syntax validation
- [ ] Manual testing with sample files

**Week 2:**
- [ ] Implement `structure.go` with basic checks
- [ ] Add import validation
- [ ] Add duplicate definition checks
- [ ] Add function signature validation
- [ ] Write tests for structure validation
- [ ] Integrate into `scriptling-cli`
- [ ] User testing with real scripts

### Phase 2.2: CLI Integration

**Week 3:**
- [ ] Add `validate` command to `scriptling-cli`
- [ ] Implement command-line flag parsing
- [ ] Add JSON output format
- [ ] Add verbose mode
- [ ] Add multi-file validation
- [ ] Documentation and help text
- [ ] Integration tests

### Phase 2.3: Enhanced Reporting (Optional)

**Week 4:**
- [ ] Add warning support (non-blocking issues)
- [ ] Add code suggestions to errors
- [ ] Add error codes for documentation linking
- [ ] Add performance stats
- [ ] Add filtering (by severity, category)
- [ ] Add file globbing support

### Phase 2.4: Type Validation (Optional/Defer)

**Future:**
- [ ] Design type inference system
- [ ] Implement `types.go`
- [ ] Add type checking to validation pipeline
- [ ] Performance optimization
- [ ] Documentation

## Usage Examples

### Command Line Usage

```bash
# Basic validation (syntax + structure)
scriptling validate myscript.py

# Syntax-only validation (faster)
scriptling validate --level=syntax myscript.py

# JSON output for CI/CD
scriptling validate --format=json myscript.py > report.json

# Multiple files
scriptling validate *.py

# Verbose output with warnings
scriptling validate --verbose myscript.py

# Treat warnings as errors (strict mode)
scriptling validate --warnings-as-errors myscript.py

# Check all files in directory
scriptling validate src/**/*.py
```

### Programmatic Usage

```go
package main

import (
    "fmt"
    "os"
    "github.com/paularlott/scriptling/validator"
)

func main() {
    // Create validator
    v := validator.NewValidator(validator.StructureCheck)

    // Validate file
    source, _ := os.ReadFile("myscript.py")
    report := v.ValidateString("myscript.py", string(source))

    // Check results
    if report.Valid {
        fmt.Println("✓ No errors found")
    } else {
        fmt.Printf("✗ Found %d errors\n", len(report.Errors))
        for _, err := range report.Errors {
            fmt.Printf("  Line %d: %s\n", err.Line, err.Message)
        }
    }
}
```

### Example Output

**Text output (default):**
```
✗ myscript.py: 3 errors found

  Line 15: Syntax error
    import invalid library name
    15 │ import 123invalid
       │         ^^^^^^^^^

  Line 23: Undefined function
    Function 'undefined_func' is not defined
    23 │ result = undefined_func()
       │          ^^^^^^^^^^^^^^
    Suggestion: Did you mean 'defined_func'?

  Line 42: Duplicate function definition
    Function 'calculate' already defined at line 10
    42 │ def calculate(x, y):
       │    ^^^^^^^^^^^^^^^^^^^

⚠ 1 warning found

  Line 50: Unused import
    Import 'math' is imported but never used
    50 │ import math

Validation failed in 0.023s
```

**JSON output:**
```json
{
  "files": [
    {
      "filename": "myscript.py",
      "valid": false,
      "errors": [
        {
          "line": 15,
          "column": 10,
          "severity": "error",
          "category": "syntax",
          "message": "Syntax error: invalid library name",
          "code": "E001"
        }
      ],
      "warnings": [
        {
          "line": 50,
          "severity": "warning",
          "category": "structure",
          "message": "Unused import 'math'",
          "code": "W001"
        }
      ]
    }
  ],
  "summary": {
    "total_files": 1,
    "valid_files": 0,
    "total_errors": 3,
    "total_warnings": 1
  }
}
```

## Success Criteria

- [ ] Validator can detect all syntax errors
- [ ] Validator can detect common structural issues
- [ ] CLI command works with single and multiple files
- [ ] Error messages include line numbers and helpful context
- [ ] JSON output works for CI/CD integration
- [ ] 80%+ test coverage
- [ ] Performance: Validate 1000-line file in < 1 second
- [ ] Documentation complete with examples

## Integration with Phase 3

The validator library will be used in Phase 3 to:

1. **Validate generated libraries** - Ensure OpenAPI-generated code is syntactically correct
2. **Validate input specs** - Check if OpenAPI specs can be successfully converted
3. **Test generator output** - Automatically validate generated code

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Parser errors not helpful | High | Add context and suggestions to errors |
| Performance issues | Medium | Benchmark and optimize hot paths |
| False positives | Medium | User testing, adjustable strictness levels |
| Complex to maintain | Medium | Keep validators simple, well-tested |
| Type checking too complex | High | Defer to Phase 2.5 or separate project |

## Open Questions

1. **Type checking complexity:** Should we include basic type checking in Phase 2 or defer?
   - **Recommendation:** Defer to Phase 2.5, focus on syntax/structure first

2. **Configuration file:** Should validator support `.scriptlintrc` configuration?
   - **Recommendation:** Yes, in Phase 2.3 for rule customization

3. **Autofix:** Should validator suggest or apply fixes?
   - **Recommendation:** Suggest fixes only, no autofix (keep simple)

4. **Language Server:** Should this become an LSP for editor integration?
   - **Recommendation:** Separate project in future, reuse validator library

## Timeline Estimate

- **Phase 2.1 (MVP):** 2 weeks
- **Phase 2.2 (CLI):** 1 week
- **Phase 2.3 (Enhanced):** 1 week
- **Phase 2.4 (Types):** 2-3 weeks (optional)
- **Total (without types):** 4 weeks
- **Total (with types):** 6-7 weeks

## Next Steps

After completing this phase:
1. Move to **Phase 3: OpenAPI Spec to Scriptling Library Generator**
2. Use validator to check all generated code
3. Use validator in CI/CD for scriptling repo itself

## References

- AST parser implementation: `parser/parser.go`
- Lexer implementation: `lexer/lexer.go`
- Error handling patterns: `errors/errors.go`
- Python's `flake8`, `pylint` for inspiration
- ESLint for JSON output format inspiration
