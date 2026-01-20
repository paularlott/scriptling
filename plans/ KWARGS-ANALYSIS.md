# KWARGS Analysis: Are We Over-Engineering?

## Question: Are we adding too much typing baggage to Scriptling?

**Short Answer:** **YES**, if we try to add type syntax to Scriptling itself.
**NO**, if we keep validation as **runtime helpers only** (no syntax changes).

---

## Current State of Scriptling Kwargs

### ✅ What Scriptling ALREADY Supports

1. **Keyword arguments in function calls** (via AST):
```python
# This already works!
result = my_function(name="Alice", age=30)
```

2. **Go builtin functions receive kwargs**:
```go
func myBuiltin(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    name := kwargs.MustGetString("name", "default")
    // ...
}
```

3. **Default parameters** in function definitions:
```python
def greet(name, prefix="Hello"):
    return prefix + ", " + name
```

4. **Variadic arguments** (`*args`):
```python
def sum_all(*args):
    total = 0
    for num in args:
        total += num
    return total
```

### ❌ What Scriptling Does NOT Support

1. **`**kwargs` syntax in function definitions**:
```python
# This does NOT work in Scriptling:
def my_function(**kwargs):
    pass
```

2. **Type annotations**:
```python
# This does NOT work:
def my_function(name: str, age: int) -> dict:
    pass
```

3. **Keyword-only parameters**:
```python
# This does NOT work:
def my_function(a, *, b):
    pass
```

---

## The Critical Insight

**Scriptling has a split personality:**

### 1. **Scriptling Language (User-Written)**
- Simple, Python-like
- No type annotations
- No `**kwargs` in function definitions
- Focus on simplicity for scripting

### 2. **Go Builtins (Library Functions)**
- Full kwargs support via `object.Kwargs`
- Type-safe getters (`MustGetString`, `GetInt`, etc.)
- Runtime validation
- This is where validation helpers live

---

## Our Proposed Plans vs. Reality

### ❌ What We Should NOT Do

**Don't add typing to the Scriptling language:**

```python
# ❌ BAD - Don't add this to Scriptling
@validate_types(name=str, age=int)
def create_user(name: str, age: int):
    pass
```

**Why this is bad:**
- Adds syntax complexity
- Goes against Scriptling's design philosophy
- Requires parser changes
- Harder for users to learn
- Not "scripting language" friendly

### ✅ What We SHOULD Do

**Keep validation as runtime helpers for Go library code:**

```go
// ✅ GOOD - Go library code (OpenAPI generated)
func createUser(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    // Validate required fields
    if err := RequireKwargs(kwargs, []string{"name", "email"}); err != nil {
        return err
    }

    // Extract with validation
    name, nameErr := kwargs.GetString("name", "")
    if nameErr != nil {
        return nameErr
    }

    // ... rest of function
}
```

**Why this is good:**
- No language syntax changes
- Existing pattern in Scriptling
- Users don't see validation complexity
- Clean, simple Scriptling API
- Generated libraries validate transparently

---

## The User Experience

### What the USER writes (Scriptling):

```python
# Simple, clean, no typing
import petstore

api = petstore.PetstoreAPI(api_key="xxx")
pet = api.create_pet(name="Fluffy", age=3)
```

### What the GENERATED library does (Go):

```go
// petstore.go (Go library)
func CreatePet(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    // Validate required fields
    if err := RequireKwargs(kwargs, []string{"name"}); err != nil {
        return err
    }

    // Extract and validate types
    name, err := kwargs.GetString("name", "")
    if err != nil {
        return errors.NewError("name must be a string")
    }

    // ... make HTTP request
}
```

**User doesn't see or care about the validation code - it just works.**

---

## Revised Phase 1 Plan

### OLD (Over-Engineered):
```
errors/validation.go
├── RequireKwargs()
├── ValidateKwargsType()
├── ValidateKwargsTypes()
├── ValidateKwargsRange()
├── ValidateKwargsOneOf()
└── KwargsValidationError()
```

### NEW (Minimal & Pragmatic):
```
errors/validation.go
├── RequireKwargs()           # Check required fields present
├── ValidateKwargsType()      # Check field type (if provided)
└── ValidateKwargsRange()     # Check numeric range (if needed)
```

**Keep it minimal. Don't add what we won't use immediately.**

---

## What About Type Checking?

### Option A: No Type Checking (Recommended)

```python
# Generated library validates at runtime
def create_pet(**kwargs):
    if not kwargs.Has("name"):
        return Error("Missing required field: name")

    name = kwargs.GetString("name", "")
    if not name:
        return Error("name must be a string")

    # ... continue
```

**Pros:**
- Simple
- No changes to Scriptling
- Runtime errors are clear
- Matches Python's duck typing philosophy

**Cons:**
- Errors caught at runtime, not "compile time"
- But Scriptling is interpreted anyway, so this is fine!

### Option B: Optional Type Hints (Future Enhancement - NOT NOW)

```python
# FUTURE - maybe add type hints as comments only
def create_pet(
    name: str,      # type hint in comment/docstring only
    age: int = 0    # optional with default
):
    """Create a pet

    Args:
        name (str): Pet name [REQUIRED]
        age (int): Pet age [OPTIONAL]
    """
    pass
```

**NOT recommended for Phase 1.** Defer to future discussion.

---

## Recommendations

### 1. **Keep Phase 1 Minimal**

**DO implement:**
- `RequireKwargs(kwargs, []string{"name", "email"})`
- Simple type checking: `ValidateKwargsType(kwargs, "name", STRING_OBJ)`
- Range validation: `ValidateKwargsRange(kwargs, "age", 0, 150)`

**DON'T implement:**
- Complex validation rules
- Type inference system
- Schema validation
- Type syntax in Scriptling language

### 2. **Documentation is Key**

Instead of types, use **comprehensive docstrings**:

```python
def create_pet(**kwargs):
    """Create a new pet

    Required:
        name (str): Pet name
        photo_urls (list): List of photo URLs

    Optional:
        id (int): Pet ID
        age (int): Pet age (0-150)
        status (str): Pet status (available, pending, sold)

    Returns:
        dict: Created pet object

    Raises:
        Error: If validation fails
    """
```

### 3. **Generate Validation Code**

The OpenAPI generator (Phase 3) should **generate validation code** that:

1. Checks required parameters
2. Validates types at runtime
3. Returns clear Error objects
4. Documents everything

**Example generated code:**

```python
def create_pet(**kwargs):
    # Validate required
    if not kwargs.Has("name"):
        return Error("Missing required field: 'name'")

    # Validate type
    name = kwargs.Get("name")
    if name.Type() != "STRING":
        return Error(f"'name' must be STRING, not {name.Type()}")

    # Validate range
    age = kwargs.Get("age")
    if age and age.Value < 0:
        return Error("'age' must be >= 0")

    # ... rest of function
```

---

## Answer to Your Question

### "Is typing going to add too much baggage?"

**NO**, as long as:

1. ✅ **No syntax changes** to Scriptling language
2. ✅ **Validation is runtime only** (like Python's duck typing)
3. ✅ **Hidden in libraries** (users don't see it)
4. ✅ **Well-documented** with docstrings
5. ✅ **Clear error messages** (validation errors are helpful)

**We're adding "baggage" to Go library code, not to Scriptling itself.**

---

## Revised Implementation Strategy

### Phase 1: Minimal Validation Helpers (4-6 hours)

```go
// errors/validation.go
package errors

// RequireKwargs checks required kwargs are present
func RequireKwargs(kwargs object.Kwargs, required []string) object.Object

// ValidateKwargsType checks a kwarg's type
func ValidateKwargsType(kwargs object.Kwargs, name string, typ object.ObjectType) object.Object

// ValidateKwargsRange checks numeric range
func ValidateKwargsRange(kwargs object.Kwargs, name string, min, max int64) object.Object
```

### Phase 2: Validator Library (unchanged)

No changes needed - validates syntax/structure only.

### Phase 3: OpenAPI Generator (simplified)

Generate code that:
- Uses Phase 1 helpers
- Has excellent docstrings
- Validates at runtime
- Returns clear errors

---

## Conclusion

**Keep Scriptling simple. Add sophistication to the tools around it.**

- ✅ Scriptling stays a simple scripting language
- ✅ Libraries get robust validation (hidden from users)
- ✅ Users write clean, type-free code
- ✅ Errors are caught at runtime with clear messages
- ✅ No language syntax changes required

**This is the right balance.**
