# CORRECTED: Generating Scriptling Code from OpenAPI

## The Critical Clarification

**We're generating `.py` files (Scriptling code), NOT Go libraries!**

The OpenAPI generator produces **actual Scriptling source code** that users import directly.

---

## Scriptling's Current Capabilities

### ✅ What Scriptling Function Definitions Support

From [parser/parser.go:1027](parser/parser.go:1027):

```python
# ✅ Regular parameters
def func(a, b):
    pass

# ✅ Default values
def func(a, b=10):
    pass

# ✅ Variadic args (*args)
def func(*args):
    pass

# ✅ Mix of all above
def func(a, b=10, *args):
    pass
```

### ❌ What Scriptling Does NOT Support

```python
# ❌ NO **kwargs syntax in definitions
def func(**kwargs):
    pass

# ❌ NO type annotations
def func(name: str):
    pass

# ❌ NO keyword-only parameters
def func(a, *, b):
    pass
```

### ✅ But kwargs Work in FUNCTION CALLS!

```python
# This works - calling with keyword arguments
result = my_func(name="Alice", age=30)

# Even if function defined with regular parameters
def my_func(name, age=0):
    return name + str(age)
```

---

## The Problem: How to Generate OpenAPI Functions?

### Challenge: OpenAPI Has Many Optional Parameters

```yaml
# OpenAPI spec
/pets:
  get:
    parameters:
      - name: limit
        in: query
        required: false
        schema:
          type: integer
          maximum: 100
      - name: page
        in: query
        required: false
        schema:
          type: integer
      - name: status
        in: query
        required: false
        schema:
          type: string
          enum: [available, pending, sold]
```

### Question: How do we represent this in Scriptling?

**Option 1: All parameters explicitly (NOT feasible)**
```python
def get_pets(limit=None, page=None, status=None, tag=None, ...):
    pass
```
**Problem:** Some endpoints have 50+ optional parameters!

**Option 2: Use a dict parameter**
```python
def get_pets(options):
    limit = options.Get("limit", 0)
    pass
```
**Problem:** Not Pythonic, loses kwargs syntax in calls

**Option 3: Add **kwargs support to Scriptling**
```python
def get_pets(**kwargs):
    pass
```
**Problem:** Requires parser/language changes

---

## Solutions Analysis

### Solution 1: Add **kwargs to Scriptling (Language Change)

**Changes needed:**

1. **Parser support** ([parser/parser.go:1027](parser/parser.go:1027))
```go
func (p *Parser) parseFunctionParameters() {
    // ... existing code ...

    // Add **kwargs support
    if p.curTokenIs(token.ASTERISK) && p.peekTokenIs(token.ASTERISK) {
        p.nextToken() // consume first *
        p.nextToken() // consume second *
        kwargParam = &ast.Identifier{...}
        // ...
    }
}
```

2. **AST support** ([ast/ast.go:445](ast/ast.go:445))
```go
type Function struct {
    // ...
    KwargParam *Identifier  // **kwargs parameter
}
```

3. **Evaluator support** ([evaluator/evaluator.go:863](evaluator/evaluator.go:863))
```go
func evalCallExpression(...) {
    // When calling function with **kwargs param
    // Pass kwargs map to function
}
```

**Pros:**
- Pythonic syntax
- Clean generated code
- Familiar to Python developers

**Cons:**
- Language change
- Parser complexity
- Testing burden

**Estimated effort:** 2-3 days

---

### Solution 2: Use Dict Helper Pattern (No Language Changes)

Generate code that uses a helper pattern:

```python
# Generated code
from kwargs import Kwargs

def get_pets(options):
    """List all pets

    Optional:
        limit (int): Max 100
        page (int): Page number
        status (str): available, pending, or sold
    """
    # Helper to safely get options
    kwargs = Kwargs(options)

    # Extract with validation
    limit = kwargs.GetInt("limit", 10)
    if limit > 100:
        return Error("'limit' must be <= 100")

    page = kwargs.GetInt("page", 1)
    status = kwargs.GetString("status", "available")

    # ... make request
```

**Usage:**
```python
# User code
api.get_pets({"limit": 10, "status": "available"})
```

**Pros:**
- No language changes
- Can implement validation helpers
- Works today

**Cons:**
- Not Pythonic (dict instead of kwargs)
- More verbose
- Loses keyword argument syntax

**Estimated effort:** 4-6 hours (implement Kwargs helper in Scriptling)

---

### Solution 3: Hybrid - Named Params + Options Dict (No Language Changes)

For common parameters, use named params. For rare ones, use options dict:

```python
# Generated code
def get_pets(limit=10, page=1, status="available", **options):
    """List all pets

    Common:
        limit (int): Max 100, default 10
        page (int): Page number, default 1
        status (str): Pet status, default "available"

    Rare (via **options):
        tags (list): Filter by tags
        category_id (int): Filter by category
    """
    # Validate common params
    if limit > 100:
        return Error("'limit' must be <= 100")

    # Extract rare params from options
    tags = options.Get("tags")
    category_id = options.Get("category_id")

    # ... make request
```

**BUT THIS REQUIRES **kwargs SUPPORT!**

So this doesn't work without Solution 1.

---

### Solution 4: Positional Args with Smart Defaults (Minimal Changes)

Generate functions with common parameters explicitly:

```python
# Generated code - only includes commonly used params
def get_pets(limit=10, page=1, status="available"):
    """List all pets

    Note: Only supports common parameters.
    For advanced usage, use the API directly via http.get()
    """
    # Validation
    if limit > 100:
        return Error("'limit' must be <= 100")

    # Build request
    url = f"{self.base_url}/pets"
    params = {"limit": limit, "page": page, "status": status}

    response = http.get(url, params=params)
    return response.json()
```

**For advanced use cases, users can access HTTP directly:**

```python
# User code - advanced usage
response = http.get(
    f"{api.base_url}/pets",
    params={"limit": 10, "tags": ["dog", "cat"], "category_id": 5},
    headers=api.get_headers()
)
```

**Pros:**
- No language changes
- Simple for common cases
- Advanced users have escape hatch
- Works today

**Cons:**
- Not comprehensive (doesn't cover all parameters)
- Advanced cases require manual HTTP calls

**Estimated effort:** Minimal

---

## My Recommendation

### **Phase 1: Add **kwargs Support to Scriptling**

**This is the RIGHT solution because:**

1. **It's genuinely useful** - Not just for OpenAPI, but for all library authors
2. **Pythonic** - Matches Python syntax developers expect
3. **Enables clean generated code** - OpenAPI generator will be much simpler
4. **Not that complex** - AST/Parser already has most infrastructure
5. **One-time investment** - Benefits entire ecosystem

**Implementation Plan (2-3 days):**

#### Day 1: Parser & AST Changes

```go
// ast/ast.go
type Function struct {
    // ... existing fields ...
    KwargParam *Identifier  // **kwargs parameter (NEW)
}

// parser/parser.go
func (p *Parser) parseFunctionParameters() {
    // ... existing code ...

    // Check for **kwargs
    if p.curTokenIs(token.ASTERISK) && p.peekTokenIs(token.ASTERISK) {
        p.nextToken() // consume first *
        p.nextToken() // consume second *
        if !p.expectPeek(token.IDENT) {
            return nil, nil, nil, nil  // Error
        }
        kwargParam = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
        if !p.expectPeek(token.RPAREN) {
            return nil, nil, nil, nil  // Error
        }
        return identifiers, defaults, variadic, kwargParam
    }
}
```

#### Day 2: Evaluator Changes

```go
// evaluator/evaluator.go
func evalCallExpression(...) {
    // ... existing code ...

    // If function has **kwargs parameter
    if fn.KwargParam != nil {
        // Create kwargs dict from call's keyword arguments
        kwargsDict := &object.Dict{Pairs: {}}
        for key, value := range node.Keywords {
            kwargsDict.Pairs[key] = object.DictPair{
                Key:   &object.String{Value: key},
                Value: evalWithContext(ctx, value, env),
            }
        }
        // Add kwargsDict to environment
        env.Set(fn.KwargParam.Value, kwargsDict)
    }
}
```

#### Day 3: Tests & Documentation

```python
# Test cases
def test_kwargs_basic(**kwargs):
    return kwargs

result = test_kwargs_basic(name="Alice", age=30)
assert result["name"] == "Alice"
assert result["age"] == 30

def test_kwargs_mixed(a, b=10, *args, **kwargs):
    return a, b, args, kwargs

result = test_kwargs_mixed(1, 2, 3, 4, x=5, y=6)
# result = (1, 2, (3, 4), {"x": 5, "y": 6})
```

### Phase 2: Add Kwargs Helper Library

```python
# stdlib/kwargs.py (or extlibs/kwargs.py)

class Kwargs:
    """Helper for working with **kwargs dict"""

    def __init__(self, kwargs):
        self.kwargs = kwargs

    def Has(self, name):
        return name in self.kwargs

    def Get(self, name, default=None):
        if name in self.kwargs:
            return self.kwargs[name]
        return default

    def GetString(self, name, default=""):
        value = self.Get(name, default)
        if value.Type() != "STRING":
            return Error(f"'{name}' must be a string")
        return value

    def GetInt(self, name, default=0):
        value = self.Get(name, default)
        if value.Type() != "INTEGER":
            return Error(f"'{name}' must be an integer")
        return value

    def Require(self, names):
        """Check all required names present"""
        missing = []
        for name in names:
            if not self.Has(name):
                missing.append(name)
        if missing:
            return Error(f"Missing required: {', '.join(missing)}")
        return None
```

### Phase 3: Generate Clean OpenAPI Libraries

```python
# Generated code (now clean and Pythonic!)
def get_pets(self, **kwargs):
    """List all pets

    Optional:
        limit (int): Max 100
        page (int): Page number
        status (str): available, pending, or sold

    Returns:
        list: List of pets
    """
    # Helper for validation
    params = Kwargs(kwargs)

    # Validation
    if params.Has("limit"):
        limit = params.GetInt("limit", 10)
        if limit.Value > 100:
            return Error("'limit' must be <= 100")

    # Build request
    url = f"{self.base_url}/pets"

    # Make request
    response = http.get(url, params=params.kwargs)
    return response.json()
```

---

## Updated Plans

### Phase 1: Add **kwargs to Scriptling (NEW)

**File:** `plans/01-add-kwargs-support.md`

**Tasks:**
1. AST changes for **kwargs
2. Parser support for **kwargs
3. Evaluator support for passing kwargs
4. Tests
5. Documentation

**Estimated:** 2-3 days

### Phase 2: Kwargs Helper Library

**File:** `plans/02-kwargs-helper-library.md`

**Tasks:**
1. Create `stdlib/kwargs.py` or `extlibs/kwargs.py`
2. Implement Kwargs helper class
3. Add validation methods
4. Tests
5. Documentation

**Estimated:** 4-6 hours

### Phase 3: OpenAPI Generator (UPDATED)

**File:** `plans/03-openapi-to-scriptling-generator.md`

**Tasks:**
1. Parser for OpenAPI specs
2. Code generator (generates .py files)
3. Template system
4. Uses **kwargs + Kwargs helper
5. Auth library
6. CLI

**Estimated:** 3-4 weeks

---

## Answer to Your Question

**"I want the openapi spec to be converted to a scriptling library in scriptling"**

**YES!** And to do this WELL, we need **kwargs support in Scriptling.

**Why:**
- ✅ Clean, Pythonic generated code
- ✅ Handles 50+ optional parameters gracefully
- ✅ Familiar to Python developers
- ✅ One-time language change benefits entire ecosystem
- ✅ Not that complex (2-3 days)

**Alternative (no **kwargs):**
- ❌ Verbose dict-based API
- ❌ Not Pythonic
- ❌ Can't handle complex APIs well
- ❌ Feels "not quite right"

---

## Recommendation

**Add **kwargs support to Scriptling FIRST.**

Then everything else becomes clean and simple:
- Generated libraries are Pythonic
- Validation is straightforward
- User experience matches expectations
- Future library authors benefit too

**This is the right investment.**
