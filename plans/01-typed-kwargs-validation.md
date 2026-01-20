# Plan: Phase 1 - Typed Kwargs Validation for Scriptling

**Status:** Planning
**Priority:** High
**Dependencies:** None
**Estimated Complexity:** Low

## Overview

Add enhanced kwargs validation helpers to Scriptling's core to support robust parameter validation in generated OpenAPI libraries. This phase provides the foundation for type-safe keyword argument handling.

## Goals

1. Add validation helper functions to the `errors` package
2. Extend `Kwargs` type with additional validation methods
3. Add comprehensive error messages for parameter validation
4. Ensure backward compatibility with existing code

## Files to Create

### 1. `errors/validation.go` (NEW)

**Purpose:** Provide validation helper functions for kwargs and function parameters

**Functions to implement:**

```go
// RequireKwargs validates that all required kwargs are present
// Returns error if any required kwargs are missing
func RequireKwargs(kwargs object.Kwargs, required []string) object.Object

// ValidateKwargsType validates a specific kwarg's type
// Returns error if type doesn't match expected
func ValidateKwargsType(kwargs object.Kwargs, name string, expectedType ObjectType) object.Object

// ValidateKwargsTypes validates multiple kwargs' types
// Returns error on first type mismatch
func ValidateKwargsTypes(kwargs object.Kwargs, types map[string]ObjectType) object.Object

// ValidateKwargsRange validates a numeric kwarg is within range
// Returns error if value is outside [min, max]
func ValidateKwargsRange(kwargs object.Kwargs, name string, min, max int64) object.Object

// ValidateKwargsOneOf validates a kwarg's value is in allowed set
// Returns error if value not in allowedValues
func ValidateKwargsOneOf(kwargs object.Kwargs, name string, allowedValues []string) object.Object

// KwargsValidationError creates a detailed validation error
func KwargsValidationError(field, expected, actual string) object.Object
```

**Error messages should include:**
- Field name
- Expected value/type
- Actual value/type
- Helpful context (e.g., "must be one of: ...")

### 2. `errors/validation_test.go` (NEW)

**Purpose:** Comprehensive tests for validation functions

**Test cases needed:**
- `TestRequireKwargs`: All required present, some missing, all missing
- `TestValidateKwargsType`: Correct type, wrong type, missing field
- `TestValidateKwargsTypes`: Multiple fields, mixed valid/invalid
- `TestValidateKwargsRange`: Within range, below min, above max, not a number
- `TestValidateKwargsOneOf`: Valid value, invalid value, missing field
- Integration tests with realistic scenarios

## Files to Modify

### 3. `errors/errors.go` (MODIFY)

**Additions:**

```go
// Add to existing error constants
const (
    ErrMissingRequired  = "missing required parameter"
    ErrInvalidType      = "invalid parameter type"
    ErrOutOfRange       = "value out of range"
    ErrInvalidValue     = "invalid value"
)

// Add helper for creating parameter validation errors
func ParameterValidationError(field, message string) *Error {
    return &Error{
        Message: fmt.Sprintf("%s: %s", field, message),
    }
}
```

### 4. `object/kwargs.go` (MODIFY - OPTIONAL)

**Potential additions** (if needed after testing):

```go
// Validate checks kwargs against validation rules
// Returns validation result with all errors found
func (k Kwargs) Validate(rules map[string]ValidationRule) (bool, []string)

// HasAny returns true if at least one of the keys exists
func (k Kwargs) HasAny(keys ...string) bool

// GetAllOfType returns all kwargs of a specific type
func (k Kwargs) GetAllOfType(typ ObjectType) map[string]Object
```

**Note:** Only add these if the validation pattern requires them. Start with simpler approach in `errors/validation.go`.

## Implementation Steps

### Step 1: Create validation.go
- [ ] Implement `RequireKwargs`
- [ ] Implement `ValidateKwargsType`
- [ ] Implement `ValidateKwargsTypes`
- [ ] Implement `ValidateKwargsRange`
- [ ] Implement `ValidateKwargsOneOf`
- [ ] Implement `KwargsValidationError`

### Step 2: Write tests
- [ ] Create `validation_test.go`
- [ ] Write unit tests for each function
- [ ] Write integration tests
- [ ] Test error messages are clear and helpful

### Step 3: Update documentation
- [ ] Add package-level documentation to `validation.go`
- [ ] Update README if needed
- [ ] Add examples to docstrings

### Step 4: Integration
- [ ] Run full test suite to ensure no regressions
- [ ] Update existing code to use new helpers where applicable
- [ ] Performance benchmarks (if concerned about overhead)

## Usage Examples

### Example 1: Required parameters
```go
func createUser(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    // Validate required fields
    if err := RequireKwargs(kwargs, []string{"name", "email"}); err != nil {
        return err
    }

    name, _ := kwargs.GetString("name", "")
    email, _ := kwargs.GetString("email", "")

    // ... function logic
}
```

### Example 2: Type validation
```go
func createPet(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    // Validate required fields with types
    if err := ValidateKwargsTypes(kwargs, map[string]object.ObjectType{
        "name":        object.STRING_OBJ,
        "age":         object.INTEGER_OBJ,
        "is_vaccinated": object.BOOLEAN_OBJ,
    }); err != nil {
        return err
    }

    name, _ := kwargs.GetString("name", "")
    age, _ := kwargs.GetInt("age", 0)

    // ... function logic
}
```

### Example 3: Range validation
```go
func setAge(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    // Validate age is between 0 and 150
    if err := ValidateKwargsRange(kwargs, "age", 0, 150); err != nil {
        return err
    }

    age, _ := kwargs.GetInt("age", 0)

    // ... function logic
}
```

### Example 4: Enum validation
```go
func setStatus(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    // Validate status is one of allowed values
    if err := ValidateKwargsOneOf(kwargs, "status", []string{"active", "inactive", "pending"}); err != nil {
        return err
    }

    status, _ := kwargs.GetString("status", "pending")

    // ... function logic
}
```

## Success Criteria

- [ ] All validation functions implemented and tested
- [ ] 100% test coverage for new code
- [ ] No regressions in existing tests
- [ ] Clear, helpful error messages
- [ ] Documentation complete with examples
- [ ] Performance impact is minimal (if measurable)

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking existing code | High | Ensure all new functions are additive, not breaking changes |
| Performance overhead | Medium | Benchmark critical paths, optimize if needed |
| Unclear error messages | Medium | User testing of error messages, iterative improvement |
| Over-engineering | Low | Start simple, add complexity only if needed |

## Open Questions

1. **Validation rule complexity:** Should we support complex validation rules (e.g., "field A required if field B is present")?
   - **Recommendation:** Defer to Phase 2 or Phase 3 when actual use cases emerge

2. **Type system integration:** Should validation be integrated into the Kwargs type itself or kept separate?
   - **Recommendation:** Keep separate in `errors` package for now, simpler and more flexible

3. **Performance:** Should we cache validation results?
   - **Recommendation:** No, unless benchmarks show it's necessary (unlikely for typical usage)

## Next Steps

After completing this phase:
1. Move to **Phase 2: Validator Library & CLI Integration**
2. Use validation helpers in validator implementation
3. Test validation helpers with real-world scenarios

## Timeline Estimate

- **Implementation:** 2-3 hours
- **Testing:** 1-2 hours
- **Documentation:** 1 hour
- **Total:** 4-6 hours

## References

- Existing `errors/errors.go` for error patterns
- Existing `object/kwargs.go` for kwargs interface
- Python's `typing` and `pydantic` for inspiration (but simpler)
- OpenAPI 3.0 Specification for validation requirements (future phases)
